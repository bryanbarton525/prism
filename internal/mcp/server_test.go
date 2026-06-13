package mcp

import (
	"context"
	"testing"

	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/app"
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

type mcpFakeRunner struct{}

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
