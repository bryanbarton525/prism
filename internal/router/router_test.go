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
