package mcp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/app"
	"github.com/bryanbarton525/prism/internal/buildinfo"
	"github.com/bryanbarton525/prism/internal/downstreammcp"
	internalpolicy "github.com/bryanbarton525/prism/internal/policy"
	"github.com/bryanbarton525/prism/internal/result"
	policypkg "github.com/bryanbarton525/prism/pkg/policy"
)

func TestSuggestRouteHandlerUsesPolicy(t *testing.T) {
	policy := internalpolicy.New(policypkg.Policy{
		Version: 1,
		Agents: map[string]policypkg.Agent{
			"kubectl": {Allowed: false},
		},
		Sources: map[string]policypkg.Source{"mcp": {Allowed: true}},
	})
	_, out, err := suggestRouteHandler(mcpFakeRunner{}, policy)(context.Background(), nil, SuggestRouteInput{
		Task: "Investigate Kubernetes rollout in namespace staging",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.PolicyDecision.Decision != policypkg.DecisionDeny {
		t.Fatalf("policy decision = %#v", out.PolicyDecision)
	}
}

func TestExplainPolicyHandlerDefaultsWhenUnconfigured(t *testing.T) {
	_, out, err := explainPolicyHandler(nil)(context.Background(), nil, ExplainPolicyInput{AgentID: "kubectl"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Decision != policypkg.DecisionAllow || out.Reason != "no policy configured" {
		t.Fatalf("decision = %#v", out)
	}
}

func TestListMCPServersHandler(t *testing.T) {
	client := downstreammcp.New(downstreammcp.State{Servers: []downstreammcp.Server{{
		Name:      "linear",
		Transport: downstreammcp.TransportCommand,
		Command:   "npx",
		Args:      []string{"-y", "mcp-remote", "https://mcp.linear.app/mcp"},
	}}})
	_, out, err := listMCPServersHandler(client)(context.Background(), nil, ListMCPServersInput{})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Configured || len(out.Servers) != 1 || out.Servers[0].Name != "linear" {
		t.Fatalf("out = %#v", out)
	}
}

func TestCallMCPToolHandlerRequiresServerAndTool(t *testing.T) {
	client := downstreammcp.New(downstreammcp.State{})
	if _, _, err := callMCPToolHandler(client)(context.Background(), nil, CallMCPToolInput{}); err == nil {
		t.Fatal("expected missing server error")
	}
	if _, _, err := callMCPToolHandler(client)(context.Background(), nil, CallMCPToolInput{Server: "linear"}); err == nil {
		t.Fatal("expected missing tool error")
	}
}

func TestRunAgentUsesStructuredWorkspaceAndNoCallerBundleFields(t *testing.T) {
	root := t.TempDir()
	runner := &capturingRunner{}
	_, _, err := runAgentHandler(runner, Config{})(context.Background(), nil, RunAgentInput{
		AgentID: "github-cli", SkillNames: []string{"gh-pr-triage"}, Task: "triage", Workspace: &WorkspaceInput{Root: root},
	})
	if err != nil {
		t.Fatal(err)
	}
	canonical, _ := filepath.EvalSymlinks(root)
	if runner.request.Workspace.Root != canonical {
		t.Fatalf("workspace = %q, want %q", runner.request.Workspace.Root, canonical)
	}
	data, _ := json.Marshal(RunAgentInput{})
	if strings.Contains(string(data), "bundle_id") || strings.Contains(string(data), "bundle_version") {
		t.Fatalf("caller bundle fields remain in contract: %s", data)
	}
}

func TestResolveWorkspaceRequiresExplicitRootWhenHostAdvertisesMultiple(t *testing.T) {
	root := t.TempDir()
	got, err := resolveWorkspace(context.Background(), nil, nil, root)
	if err != nil || got != root {
		t.Fatalf("fallback = %q, %v", got, err)
	}
	if _, err := resolveWorkspace(context.Background(), nil, &WorkspaceInput{Root: "relative"}, ""); err == nil {
		t.Fatal("expected relative workspace rejection")
	}
}

func TestRunAgentRejectsAmbiguousAdvertisedRoots(t *testing.T) {
	ctx := context.Background()
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "prism", Version: "test"}, nil)
	mcpsdk.AddTool(server, &mcpsdk.Tool{Name: "run_agent"}, runAgentHandler(&capturingRunner{}, Config{}))
	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "1"}, nil)
	client.AddRoots(&mcpsdk.Root{URI: "file:///tmp/one"}, &mcpsdk.Root{URI: "file:///tmp/two"})
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()
	result, err := clientSession.CallTool(ctx, &mcpsdk.CallToolParams{Name: "run_agent", Arguments: map[string]any{"agent_id": "github-cli", "skill_names": []string{"gh-pr-triage"}, "task": "triage"}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatalf("expected ambiguous roots error: %#v", result)
	}
}

func TestRegisteredToolsUseReleaseContract(t *testing.T) {
	ctx := context.Background()
	old := buildinfo.Version
	buildinfo.Version = "v4.5.6"
	t.Cleanup(func() { buildinfo.Version = old })
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: serverName, Version: buildinfo.Current().Version}, nil)
	registerTools(server, mcpFakeRunner{}, Config{})
	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "1"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()
	if got := clientSession.InitializeResult().ServerInfo.Version; got != "v4.5.6" {
		t.Fatalf("server version = %q", got)
	}
	listed, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var runSchema []byte
	for _, tool := range listed.Tools {
		if tool.Name == "install_bundle" || tool.Name == "list_bundles" {
			t.Fatalf("removed tool is still registered: %s", tool.Name)
		}
		if tool.Name == "run_agent" {
			runSchema, _ = json.Marshal(tool.InputSchema)
		}
	}
	if !strings.Contains(string(runSchema), "workspace") || strings.Contains(string(runSchema), "bundle_id") || strings.Contains(string(runSchema), "bundle_version") {
		t.Fatalf("run_agent schema = %s", runSchema)
	}
}

func TestPublishedToolContract(t *testing.T) {
	ctx := context.Background()
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "prism", Version: buildinfo.Current().Version}, nil)
	registerTools(server, mcpFakeRunner{}, Config{})
	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "1"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()
	if got := clientSession.InitializeResult().ServerInfo.Version; got != buildinfo.Current().Version {
		t.Fatalf("MCP version = %q", got)
	}
	listed, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	foundRun := false
	for _, tool := range listed.Tools {
		if tool.Name == "install_bundle" || tool.Name == "list_bundles" {
			t.Fatalf("removed tool still published: %s", tool.Name)
		}
		if tool.Name == "run_agent" {
			foundRun = true
			schema, _ := json.Marshal(tool.InputSchema)
			text := string(schema)
			if !strings.Contains(text, "workspace") || strings.Contains(text, "bundle_id") || strings.Contains(text, "bundle_version") {
				t.Fatalf("run_agent schema = %s", text)
			}
		}
	}
	if !foundRun {
		t.Fatal("run_agent not published")
	}
}

func TestRunAgentHandlerMarksFailedEnvelopesAsErrors(t *testing.T) {
	runner := failingRunner{status: result.StatusValidationFail, summary: "skill not allowed"}
	res, out, err := runAgentHandler(runner, Config{})(context.Background(), nil, RunAgentInput{
		AgentID:    "kubectl",
		Task:       "triage",
		SkillNames: []string{"not-allowed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatalf("IsError = false for status %q", out.Status)
	}

	okRunner := failingRunner{status: result.StatusOK, summary: "fine"}
	res, _, err = runAgentHandler(okRunner, Config{})(context.Background(), nil, RunAgentInput{
		AgentID:    "kubectl",
		Task:       "triage",
		SkillNames: []string{"k8s-rollout-diagnostics"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatal("IsError = true for status ok")
	}
}

type failingRunner struct {
	mcpFakeRunner
	status  string
	summary string
}

func (f failingRunner) Run(context.Context, app.RunRequest) (result.RunResult, error) {
	return result.RunResult{Status: f.status, Summary: f.summary}, nil
}

type mcpFakeRunner struct{}

type capturingRunner struct {
	mcpFakeRunner
	request app.RunRequest
}

func (r *capturingRunner) Run(_ context.Context, request app.RunRequest) (result.RunResult, error) {
	r.request = request
	return result.RunResult{}, nil
}

func (mcpFakeRunner) ListAgents(context.Context) ([]agent.Summary, error) {
	return []agent.Summary{{ID: "kubectl", AllowedSkills: []string{"k8s-rollout-diagnostics"}}}, nil
}

func (mcpFakeRunner) Run(context.Context, app.RunRequest) (result.RunResult, error) {
	return result.RunResult{}, nil
}

func (mcpFakeRunner) GetConstitution(context.Context, string) (app.Constitution, error) {
	return app.Constitution{}, nil
}

func (mcpFakeRunner) Doctor(context.Context) (result.DoctorResult, error) {
	return result.DoctorResult{}, nil
}
