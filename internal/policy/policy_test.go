package policy

import (
	"os"
	"path/filepath"
	"testing"

	policypkg "github.com/bryanbarton525/prism/pkg/policy"
)

func TestEngineDeniesDisallowedSkill(t *testing.T) {
	engine := New(policypkg.Policy{
		Version: 1,
		Agents: map[string]policypkg.Agent{
			"kubectl": {Allowed: true, Skills: []string{"k8s-rollout-diagnostics"}},
		},
		Sources: map[string]policypkg.Source{"cli": {Allowed: true}},
	})
	decision := engine.Explain(policypkg.Request{
		AgentID: "kubectl",
		Skills:  []string{"danger"},
		Source:  "cli",
	})
	if decision.Decision != policypkg.DecisionDeny {
		t.Fatalf("decision = %s, want deny", decision.Decision)
	}
}

func TestNilEngineAllowsCompatibility(t *testing.T) {
	decision := (*Engine)(nil).Explain(policypkg.Request{AgentID: "kubectl"})
	if decision.Decision != policypkg.DecisionAllow {
		t.Fatalf("decision = %s, want allow", decision.Decision)
	}
}

func TestEngineEnforcesProductPolicyFields(t *testing.T) {
	engine := New(policypkg.Policy{
		Version: 1,
		Defaults: policypkg.Defaults{
			RawPromptCapture:    false,
			RemoteModelsAllowed: false,
			MaxGraphNodes:       2,
			MaxGraphDepth:       1,
			MaxEvidenceBytes:    10,
		},
		Agents: map[string]policypkg.Agent{
			"kubectl": {Allowed: true, Skills: []string{"k8s"}, Plugins: map[string]policypkg.Plugin{"kubernetes": {Mode: "read_only"}}},
		},
		Sources:    map[string]policypkg.Source{"cli": {Allowed: true}},
		Workspaces: map[string]policypkg.Workspace{"repo-a": {Allowed: true}},
		Bundles:    map[string]policypkg.Bundle{"k8s": {Allowed: true}},
	})

	tests := []struct {
		name string
		req  policypkg.Request
		want string
	}{
		{"unknown source", policypkg.Request{Source: "mcp"}, policypkg.DecisionDeny},
		{"unknown workspace", policypkg.Request{WorkspaceID: "repo-b"}, policypkg.DecisionDeny},
		{"unknown bundle", policypkg.Request{BundleID: "other"}, policypkg.DecisionDeny},
		{"raw prompt", policypkg.Request{RawPromptCaptureRequested: true}, policypkg.DecisionDeny},
		{"remote model", policypkg.Request{RemoteModelRequested: true}, policypkg.DecisionDeny},
		{"write", policypkg.Request{WriteRequested: true}, policypkg.DecisionDeny},
		{"graph nodes", policypkg.Request{GraphNodes: 3}, policypkg.DecisionDeny},
		{"graph depth", policypkg.Request{GraphDepth: 2}, policypkg.DecisionDeny},
		{"evidence", policypkg.Request{EvidenceBytes: 11}, policypkg.DecisionDeny},
		{"allowed", policypkg.Request{AgentID: "kubectl", Skills: []string{"k8s"}, Plugins: []string{"kubernetes"}, Source: "cli", WorkspaceID: "repo-a", BundleID: "k8s", EvidenceBytes: 10}, policypkg.DecisionAllow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := engine.Explain(tt.req); got.Decision != tt.want {
				t.Fatalf("decision = %#v, want %s", got, tt.want)
			}
		})
	}
}

func TestPolicyTestSuite(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")
	casesPath := filepath.Join(dir, "cases.yaml")
	if err := os.WriteFile(policyPath, []byte(`version: 1
agents:
  kubectl:
    allowed: true
    skills: [k8s]
sources:
  cli:
    allowed: true
write_actions:
  allowed: false
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(casesPath, []byte(`cases:
  - name: allow kubectl
    request:
      agent_id: kubectl
      skills: [k8s]
      source: cli
    want_decision: allow
  - name: deny write
    request:
      write_requested: true
    want_decision: deny
`), 0o644); err != nil {
		t.Fatal(err)
	}
	engine, err := Load(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	suite, err := LoadTestSuite(casesPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, res := range engine.Test(suite) {
		if !res.Passed {
			t.Fatalf("case failed: %#v", res)
		}
	}
}
