package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Parse tests
// ---------------------------------------------------------------------------

const validSpec = `---
id: github-cli
name: GitHub CLI
description: Diagnose PRs with gh.
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
---

# GitHub CLI constitution

Inspect GitHub state.
`

func TestParse_Valid(t *testing.T) {
	spec, err := Parse([]byte(validSpec), "github-cli.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.ID != "github-cli" {
		t.Errorf("ID: want github-cli, got %s", spec.ID)
	}
	if spec.Model != "llama3.1:8b" {
		t.Errorf("Model: want llama3.1:8b, got %s", spec.Model)
	}
	if spec.ContextBudget != 6144 {
		t.Errorf("ContextBudget: want 6144, got %d", spec.ContextBudget)
	}
	if len(spec.AllowedSkills) != 2 {
		t.Errorf("AllowedSkills: want 2, got %d", len(spec.AllowedSkills))
	}
	if spec.Temperature != 0.1 {
		t.Errorf("Temperature: want 0.1, got %f", spec.Temperature)
	}
	if !strings.Contains(spec.Body, "GitHub CLI constitution") {
		t.Errorf("Body should contain constitution text, got: %s", spec.Body)
	}
}

func TestParse_MissingFrontmatter(t *testing.T) {
	_, err := Parse([]byte("# No frontmatter"), "x.md")
	if err == nil {
		t.Fatal("expected error for missing frontmatter delimiter")
	}
}

func TestParse_UnclosedFrontmatter(t *testing.T) {
	_, err := Parse([]byte("---\nid: x\nname: x"), "x.md")
	if err == nil {
		t.Fatal("expected error for unclosed frontmatter")
	}
}

func TestParse_MissingRequiredFields(t *testing.T) {
	cases := []struct {
		name    string
		content string
		missing string
	}{
		{
			"missing id",
			"---\nname: N\ndescription: D\nmodel: m\ncontext_budget: 1\nallowed_skills: [s]\nlatency_budget_ms: 1\n---\nbody",
			"id",
		},
		{
			"missing model",
			"---\nid: x\nname: N\ndescription: D\ncontext_budget: 1\nallowed_skills: [s]\nlatency_budget_ms: 1\n---\nbody",
			"model",
		},
		{
			"missing allowed_skills",
			"---\nid: x\nname: N\ndescription: D\nmodel: m\ncontext_budget: 1\nlatency_budget_ms: 1\n---\nbody",
			"allowed_skills",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.content), "x.md")
			if err == nil {
				t.Fatalf("expected error for %s", tc.missing)
			}
			if !strings.Contains(err.Error(), tc.missing) {
				t.Errorf("error should mention %q, got: %v", tc.missing, err)
			}
		})
	}
}

func TestParse_IDStemMismatch(t *testing.T) {
	_, err := Parse([]byte(validSpec), "wrong-name.md")
	if err == nil {
		t.Fatal("expected error when id does not match filename stem")
	}
	if !strings.Contains(err.Error(), "does not match filename stem") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAllowsSkill(t *testing.T) {
	spec, _ := Parse([]byte(validSpec), "github-cli.md")
	if !spec.AllowsSkill("gh-pr-triage") {
		t.Error("should allow gh-pr-triage")
	}
	if spec.AllowsSkill("kubectl-triage") {
		t.Error("should not allow kubectl-triage")
	}
}

// ---------------------------------------------------------------------------
// ResolveConstitution tests
// ---------------------------------------------------------------------------

func TestResolveConstitution_Body(t *testing.T) {
	spec, _ := Parse([]byte(validSpec), "github-cli.md")
	text, src, err := spec.ResolveConstitution(os.DirFS("/nonexistent"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src != "body" {
		t.Errorf("source: want body, got %s", src)
	}
	if !strings.Contains(text, "GitHub CLI constitution") {
		t.Errorf("unexpected constitution text: %s", text)
	}
}

func TestResolveConstitution_Path(t *testing.T) {
	rootDir := t.TempDir()
	constDir := filepath.Join(rootDir, "constitutions")
	if err := os.MkdirAll(constDir, 0o755); err != nil {
		t.Fatal(err)
	}
	constContent := "# External constitution\n\nMission text here."
	if err := os.WriteFile(filepath.Join(constDir, "myagent.md"), []byte(constContent), 0o644); err != nil {
		t.Fatal(err)
	}

	spec := &Spec{
		ID:               "myagent",
		Name:             "My Agent",
		Description:      "desc",
		Model:            "llama3.1:8b",
		ContextBudget:    4096,
		AllowedSkills:    []string{"some-skill"},
		ConstitutionPath: "constitutions/myagent.md",
	}
	text, src, err := spec.ResolveConstitution(os.DirFS(rootDir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src != "path" {
		t.Errorf("source: want path, got %s", src)
	}
	if !strings.Contains(text, "External constitution") {
		t.Errorf("unexpected text: %s", text)
	}
}

func TestResolveConstitution_Legacy(t *testing.T) {
	rootDir := t.TempDir()
	constDir := filepath.Join(rootDir, "constitutions")
	if err := os.MkdirAll(constDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyContent := "# Legacy constitution"
	if err := os.WriteFile(filepath.Join(constDir, "legacy-agent.md"), []byte(legacyContent), 0o644); err != nil {
		t.Fatal(err)
	}

	spec := &Spec{
		ID:            "legacy-agent",
		Name:          "Legacy",
		Description:   "desc",
		Model:         "llama3.1:8b",
		ContextBudget: 4096,
		AllowedSkills: []string{"some-skill"},
		// No Body, no ConstitutionPath — should fall back to legacy path.
	}
	text, src, err := spec.ResolveConstitution(os.DirFS(rootDir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src != "legacy" {
		t.Errorf("source: want legacy, got %s", src)
	}
	if !strings.Contains(text, "Legacy constitution") {
		t.Errorf("unexpected text: %s", text)
	}
}

func TestResolveConstitution_None(t *testing.T) {
	spec := &Spec{
		ID:            "empty-agent",
		Name:          "Empty",
		Description:   "desc",
		Model:         "llama3.1:8b",
		ContextBudget: 4096,
		AllowedSkills: []string{"some-skill"},
	}
	text, src, err := spec.ResolveConstitution(os.DirFS(t.TempDir()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src != "none" {
		t.Errorf("source: want none, got %s", src)
	}
	if text != "" {
		t.Errorf("expected empty text, got: %s", text)
	}
}

func TestResolveConstitution_PathNotFound(t *testing.T) {
	spec := &Spec{
		ID:               "x",
		ConstitutionPath: "constitutions/missing.md",
	}
	_, _, err := spec.ResolveConstitution(os.DirFS(t.TempDir()))
	if err == nil {
		t.Fatal("expected error for missing constitution_path")
	}
}
