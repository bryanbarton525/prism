package agent

import (
	"testing"
)

func TestParse_ValidSpec(t *testing.T) {
	raw := []byte(`---
id: github-cli
name: GitHub CLI
description: Diagnose PRs and CI with gh.
model: llama3.1:8b
context_budget: 6144
allowed_skills:
  - gh-pr-triage
  - gh-actions-diagnostics
latency_budget_ms: 30000
temperature: 0.1
tools:
  - gh
outputs: summary findings confidence
constitution_path: constitutions/github-cli.md
---

# GitHub CLI agent

Mission: inspect GitHub state.
`)

	spec, err := Parse(raw, "github-cli.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if spec.ID != "github-cli" {
		t.Errorf("ID: want github-cli, got %q", spec.ID)
	}
	if spec.Model != "llama3.1:8b" {
		t.Errorf("Model: want llama3.1:8b, got %q", spec.Model)
	}
	if len(spec.AllowedSkills) != 2 {
		t.Errorf("AllowedSkills: want 2, got %d", len(spec.AllowedSkills))
	}
	if spec.ContextBudget != 6144 {
		t.Errorf("ContextBudget: want 6144, got %d", spec.ContextBudget)
	}
	if spec.Body == "" {
		t.Error("Body should not be empty")
	}
}

func TestParse_MissingDelimiter(t *testing.T) {
	_, err := Parse([]byte("no frontmatter here"), "bad.md")
	if err == nil {
		t.Fatal("expected error for missing delimiter")
	}
}

func TestParse_MissingRequiredFields(t *testing.T) {
	raw := []byte(`---
id: test
name: Test
---
body
`)
	_, err := Parse(raw, "test.md")
	if err == nil {
		t.Fatal("expected validation error for missing required fields")
	}
}

func TestParse_IDMismatch(t *testing.T) {
	raw := []byte(`---
id: wrong-id
name: Test
description: Some desc.
model: llama3.1:8b
context_budget: 1024
allowed_skills:
  - skill-a
latency_budget_ms: 10000
---
body
`)
	_, err := Parse(raw, "correct-id.md")
	if err == nil {
		t.Fatal("expected error when id does not match filename stem")
	}
}

func TestAllowsSkill(t *testing.T) {
	spec := &Spec{AllowedSkills: []string{"alpha", "beta"}}
	if !spec.AllowsSkill("alpha") {
		t.Error("AllowsSkill(alpha) should be true")
	}
	if spec.AllowsSkill("gamma") {
		t.Error("AllowsSkill(gamma) should be false")
	}
}
