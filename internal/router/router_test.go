package router

import (
	"context"
	"testing"

	"github.com/bryanbarton525/prism/internal/agent"
	internalpolicy "github.com/bryanbarton525/prism/internal/policy"
	policypkg "github.com/bryanbarton525/prism/pkg/policy"
)

type fakeLister []agent.Summary

func (f fakeLister) ListAgents(context.Context) ([]agent.Summary, error) {
	return []agent.Summary(f), nil
}

func TestSuggestReturnsPolicyDenial(t *testing.T) {
	policy := internalpolicy.New(policypkg.Policy{
		Version: 1,
		Agents: map[string]policypkg.Agent{
			"kubectl": {Allowed: false},
		},
		Sources: map[string]policypkg.Source{"cli": {Allowed: true}},
	})
	r := New(fakeLister{{ID: "kubectl", AllowedSkills: []string{"k8s-rollout-diagnostics"}}}, policy)
	res, err := r.Suggest(context.Background(), Request{Task: "Investigate deployment checkout-api rollout in namespace staging", Source: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	if res.PolicyDecision.Decision != policypkg.DecisionDeny {
		t.Fatalf("policy decision = %#v", res.PolicyDecision)
	}
}

func TestSuggestKubernetes(t *testing.T) {
	r := New(fakeLister{{ID: "kubectl", AllowedSkills: []string{"k8s-rollout-diagnostics"}}}, nil)
	res, err := r.Suggest(context.Background(), Request{Task: "Investigate deployment checkout-api rollout in namespace staging", Source: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	if res.AgentID != "kubectl" {
		t.Fatalf("agent = %q, want kubectl", res.AgentID)
	}
	if len(res.SkillNames) != 1 || res.SkillNames[0] != "k8s-rollout-diagnostics" {
		t.Fatalf("skills = %#v", res.SkillNames)
	}
}

func TestSuggestLinear(t *testing.T) {
	r := New(fakeLister{{ID: "linear", AllowedSkills: []string{"linear-issue-management"}}}, nil)
	res, err := r.Suggest(context.Background(), Request{Task: "Create a Linear issue for checkout-api rollout follow-up", Source: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	if res.AgentID != "linear" {
		t.Fatalf("agent = %q, want linear", res.AgentID)
	}
	if len(res.SkillNames) != 1 || res.SkillNames[0] != "linear-issue-management" {
		t.Fatalf("skills = %#v", res.SkillNames)
	}
	if res.Risk != "requires_write_approval" {
		t.Fatalf("risk = %q", res.Risk)
	}
}

func TestSuggestRepositoryInvestigatorForGraphShapedTasks(t *testing.T) {
	r := New(fakeLister{{ID: "repo-investigator", AllowedSkills: []string{"graphify-query"}}}, nil)
	for _, task := range []string{
		"Investigate the repository architecture of the CLI.",
		"Trace the component relationship between runner and MCP.",
		"Find the dependency path from host wrapper to execution.",
		"Perform change impact analysis for the router.",
	} {
		t.Run(task, func(t *testing.T) {
			res, err := r.Suggest(context.Background(), Request{Task: task, Source: "mcp"})
			if err != nil {
				t.Fatal(err)
			}
			if res.AgentID != "repo-investigator" || len(res.SkillNames) != 1 || res.SkillNames[0] != "graphify-query" {
				t.Fatalf("route = %#v", res)
			}
		})
	}
}

func TestSuggestDoesNotOverRouteOneFileLookupToGraphify(t *testing.T) {
	r := New(fakeLister{
		{ID: "repo-investigator", AllowedSkills: []string{"graphify-query"}},
		{ID: "go-helper", AllowedSkills: []string{"go-helper-fn"}},
	}, nil)
	res, err := r.Suggest(context.Background(), Request{Task: "Locate and read internal/cli/root.go.", Source: "mcp"})
	if err != nil {
		t.Fatal(err)
	}
	if res.AgentID == "repo-investigator" {
		t.Fatalf("ordinary one-file lookup was over-routed: %#v", res)
	}
}
