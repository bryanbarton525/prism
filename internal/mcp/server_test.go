package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	prismbundle "github.com/bryanbarton525/prism"
	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/app"
	"github.com/bryanbarton525/prism/internal/buildinfo"
	"github.com/bryanbarton525/prism/internal/downstreammcp"
	"github.com/bryanbarton525/prism/internal/extensions"
	"github.com/bryanbarton525/prism/internal/graphify"
	llmruntime "github.com/bryanbarton525/prism/internal/llm/runtime"
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

func TestRecommendToolsHandlerCallsRunner(t *testing.T) {
	runner := &recommendationRunner{}
	_, got, err := recommendToolsHandler(runner)(context.Background(), nil, RecommendToolsInput{
		AgentID: "linear", Task: "find an issue", TopK: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if runner.agentID != "linear" || runner.task != "find an issue" || runner.topK != 3 {
		t.Fatalf("request = %+v", runner)
	}
	if len(got.Tools) != 1 || got.Tools[0].Name != "find_issue" {
		t.Fatalf("response = %+v", got)
	}
}

func TestRecommendToolsIsCallableThroughMCP(t *testing.T) {
	ctx := context.Background()
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "prism-test", Version: "1"}, nil)
	runner := &recommendationRunner{}
	registerTools(server, runner, Config{})
	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "parent", Version: "1"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()
	result, err := clientSession.CallTool(ctx, &mcpsdk.CallToolParams{Name: "recommend_tools", Arguments: map[string]any{"agent_id": "linear", "task": "find issue", "top_k": 1}})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError || runner.agentID != "linear" || runner.topK != 1 {
		t.Fatalf("result=%+v runner=%+v", result, runner)
	}
}

type recommendationRunner struct {
	mcpFakeRunner
	agentID, task string
	topK          int
}

func (r *recommendationRunner) RecommendTools(_ context.Context, agentID, task string, topK int) (app.ToolRecommendations, error) {
	r.agentID, r.task, r.topK = agentID, task, topK
	return app.ToolRecommendations{Tools: []app.ToolRecommendation{{Server: "issues", Name: "find_issue"}}}, nil
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

func TestListAgentsHandlerIncludesManagedCatalogAgents(t *testing.T) {
	temp := t.TempDir()
	agentPath := filepath.Join(temp, "managed-agent.md")
	if err := os.WriteFile(agentPath, []byte(`---
id: managed-agent
name: Managed Agent
description: Managed extension agent.
model: llama3.1:8b-instruct-q6_K
context_budget: 16000
allowed_skills: [k8s-rollout-diagnostics]
latency_budget_ms: 10000
---
Managed body.`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	base := os.DirFS(repoRoot)
	agentData, err := os.ReadFile(agentPath)
	if err != nil {
		t.Fatal(err)
	}
	agentDigest := sha256.Sum256(agentData)
	snapshot, err := extensions.ComposeCatalog(extensions.ComposeInput{
		BundleFS: base,
		Manifest: extensions.Manifest{
			Version: extensions.ManifestVersion,
			Entries: []extensions.ManifestEntry{
				{Identity: "managed-agent", Kind: "agent", ObjectPath: agentPath, Digest: hex.EncodeToString(agentDigest[:]), Source: "test"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	runner, err := app.New(app.Config{BundleFS: base, ExtensionSnapshot: &snapshot})
	if err != nil {
		t.Fatal(err)
	}
	_, out, err := listAgentsHandler(runner)(context.Background(), nil, ListAgentsInput{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range out.Agents {
		if a.ID == "managed-agent" {
			found = true
		}
	}
	if !found {
		t.Fatalf("managed agent missing from MCP list: %#v", out.Agents)
	}
}

func TestSkillHealthHandlerUsesManagedRunnerSkills(t *testing.T) {
	state := t.TempDir()
	source := filepath.Join(t.TempDir(), "managed-health-skill")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("---\nname: managed-health-skill\ndescription: Managed health fixture\n---\n# Skill"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := extensions.NewLocalSkillService(state).InstallLocalSkills(context.Background(), extensions.InstallLocalSkillsRequest{Source: source}); err != nil {
		t.Fatal(err)
	}
	store := extensions.NewStore(state)
	manifest, _, err := store.RecoverAndLoadManifest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := extensions.ComposeCatalog(extensions.ComposeInput{BundleFS: prismbundle.BundleFS(), Manifest: manifest, ObjectStoreRoot: store.ObjectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	runner, err := app.New(app.Config{BundleFS: prismbundle.BundleFS(), ExtensionSnapshot: &snapshot})
	if err != nil {
		t.Fatal(err)
	}
	_, out, err := skillHealthHandler(Config{SkillsFS: runner.SkillsFS()})(context.Background(), nil, SkillHealthInput{SkillName: "managed-health-skill"})
	if err != nil || out.Count != 1 || out.Skills[0].Name != "managed-health-skill" || !out.Skills[0].OK {
		t.Fatalf("managed skill health: output=%#v err=%v", out, err)
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

func TestFixtureParentRoutesGraphifyInvestigationThroughRunAgent(t *testing.T) {
	workspace := t.TempDir()
	sourcePath := filepath.Join(workspace, "cmd", "prism", "main.go")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath, []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(workspace, "graphify-out", "graph.json")
	if err := os.MkdirAll(filepath.Dir(index), 0o755); err != nil {
		t.Fatal(err)
	}
	indexFixture := readGraphifyFixture(t, "fake-index", "graph.json")
	if err := os.WriteFile(index, indexFixture, 0o600); err != nil {
		t.Fatal(err)
	}
	graphResponse := string(readGraphifyFixture(t, "fake-mcp", "query_graph.txt"))
	downstream := &fixtureGraphifyMCP{content: graphResponse, index: index}
	model := &fixtureGraphifyRuntime{responses: []llmruntime.ChatResponse{
		{
			Model: "fixture-offload",
			Message: llmruntime.Message{Role: "assistant", ToolCalls: []llmruntime.ToolCall{{
				Type: "function",
				Function: llmruntime.ToolCallFunction{
					Name: "query_graph",
					Arguments: map[string]any{
						"question": "Where does the Prism CLI root start?",
						"depth":    2,
					},
				},
			}}},
			Usage: llmruntime.Usage{PromptTokens: 120, CompletionTokens: 16},
		},
		{
			Model: "fixture-offload",
			Message: llmruntime.Message{Role: "assistant", Content: `{
	  "summary":"The Prism CLI root starts in cmd/prism/main.go.",
	  "findings":["cmd/prism/main.go:1 was verified from the selected workspace source."],
	  "confidence":"high"
	}`},
			Usage: llmruntime.Usage{PromptTokens: 180, CompletionTokens: 30},
		},
	}}
	runner, err := app.New(app.Config{
		BundleFS:      prismbundle.BundleFS(),
		ModelRuntime:  model,
		DownstreamMCP: downstream,
		Graphify: graphify.Config{
			Version: graphify.ConfigVersion, OperatorApproved: true,
			Binding: &graphify.Binding{
				Workspace: workspace, IndexPath: index, UpstreamVersion: graphify.PinnedUpstreamVersion,
				SchemaVersion: graphify.PinnedContractID, GenerationFingerprint: "fixture-generation",
			},
			Endpoint: &graphify.Endpoint{Server: "graphify-fixture", Kind: graphify.EndpointSelfHosted},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "prism", Version: "test"}, nil)
	registerTools(server, runner, Config{})
	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "fixture-parent", Version: "1"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	call, err := clientSession.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "run_agent",
		Arguments: map[string]any{
			"agent_id":    "repo-investigator",
			"skill_names": []string{"graphify-query"},
			"task":        "Investigate the CLI architecture.",
			"workspace": map[string]any{
				"root":                   workspace,
				"generation_fingerprint": "fixture-generation",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if call.IsError || len(call.Content) != 1 {
		t.Fatalf("run_agent result = %#v", call)
	}
	text, ok := call.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("run_agent content type = %T", call.Content[0])
	}
	var output result.RunResult
	if err := json.Unmarshal([]byte(text.Text), &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != result.StatusOK || !strings.Contains(output.Summary, "cmd/prism/main.go") {
		t.Fatalf("run_agent output = %#v", output)
	}
	if got := downstream.calls; len(got) != 1 || got[0] != "graphify-fixture.query_graph" {
		t.Fatalf("Graphify calls = %#v", got)
	}
	if len(model.requests) != 2 || !strings.Contains(model.requests[0].Messages[0].Content, "Graphify Query Tools") {
		t.Fatalf("offload requests did not receive Graphify capability: %#v", model.requests)
	}
	if strings.Contains(model.requests[0].Messages[0].Content, "Prism MCP Bridge Tools") {
		t.Fatal("repo-investigator was routed through the generic MCP bridge")
	}
	assertGraphifyEvidence(t, output.Artifacts, workspace)
}

func readGraphifyFixture(t *testing.T, parts ...string) []byte {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	path := append([]string{filepath.Dir(thisFile), "..", "..", "testdata", "graphify"}, parts...)
	data, err := os.ReadFile(filepath.Join(path...))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertGraphifyEvidence(t *testing.T, artifacts []result.Artifact, workspace string) {
	t.Helper()
	var graphCall, sourceVerification string
	for _, artifact := range artifacts {
		switch artifact.Label {
		case "graphify-tool:query_graph":
			graphCall = artifact.Content
		case "graphify-source:cmd/prism/main.go":
			sourceVerification = artifact.Content
		}
	}
	for _, marker := range []string{
		`"upstream_version": "v0.9.61"`,
		`"contract_id": "prism-graphify-mcp-v0.9.61"`,
		workspace,
		`"verified":true`,
	} {
		if !strings.Contains(graphCall+"\n"+sourceVerification, marker) {
			t.Fatalf("missing evidence provenance %q in graph=%q source=%q", marker, graphCall, sourceVerification)
		}
	}
	if !strings.Contains(sourceVerification, "func main()") {
		t.Fatalf("source verification did not contain selected workspace source: %q", sourceVerification)
	}
}

type fixtureGraphifyMCP struct {
	calls   []string
	content string
	index   string
}

func (f *fixtureGraphifyMCP) Servers() []downstreammcp.Server {
	return []downstreammcp.Server{{
		Name:      "graphify-fixture",
		Transport: downstreammcp.TransportCommand,
		Command:   "graphify-mcp",
		Args:      []string{f.index},
	}}
}

func (f *fixtureGraphifyMCP) ListTools(context.Context, string, downstreammcp.ListToolsOptions) (downstreammcp.ListToolsResult, error) {
	contracts := graphify.PinnedToolContracts()
	tools := make([]downstreammcp.ToolSummary, 0, len(contracts))
	for _, contract := range contracts {
		tools = append(tools, downstreammcp.ToolSummary{Name: contract.Name, InputSchema: contract.InputSchema})
	}
	return downstreammcp.ListToolsResult{Tools: tools, Total: len(tools)}, nil
}

func (f *fixtureGraphifyMCP) CallTool(_ context.Context, server, tool string, _ map[string]any) (downstreammcp.CallResult, error) {
	f.calls = append(f.calls, server+"."+tool)
	return downstreammcp.CallResult{Server: server, Tool: tool, Content: f.content}, nil
}

type fixtureGraphifyRuntime struct {
	requests  []llmruntime.ChatRequest
	responses []llmruntime.ChatResponse
}

func (f *fixtureGraphifyRuntime) Engine() llmruntime.Engine { return llmruntime.EngineOllama }

func (f *fixtureGraphifyRuntime) Health(context.Context) (*llmruntime.HealthStatus, error) {
	return &llmruntime.HealthStatus{Healthy: true, Engine: llmruntime.EngineOllama}, nil
}

func (f *fixtureGraphifyRuntime) Chat(_ context.Context, request llmruntime.ChatRequest) (*llmruntime.ChatResponse, error) {
	f.requests = append(f.requests, request)
	if len(f.responses) == 0 {
		return nil, os.ErrNotExist
	}
	response := f.responses[0]
	f.responses = f.responses[1:]
	return &response, nil
}

func (f *fixtureGraphifyRuntime) Stream(context.Context, llmruntime.ChatRequest) (<-chan llmruntime.StreamEvent, error) {
	return nil, nil
}

func (f *fixtureGraphifyRuntime) GenerateStructured(context.Context, llmruntime.StructuredRequest) (*llmruntime.StructuredResponse, error) {
	return nil, nil
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
