package agent

import (
	"testing"
)

func TestSplitFrontmatter_Valid(t *testing.T) {
	input := []byte("---\nid: test\nname: Test\n---\n\n# Body\n")
	fm, body, err := splitFrontmatter(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The newline immediately before the closing "---" is consumed by the
	// search pattern, so frontmatter does not carry a trailing newline.
	if string(fm) != "id: test\nname: Test" {
		t.Errorf("frontmatter = %q, want \"id: test\\nname: Test\"", fm)
	}
	if string(body) != "\n# Body\n" {
		t.Errorf("body = %q, want \"\\n# Body\\n\"", body)
	}
}

func TestSplitFrontmatter_MissingOpenDelimiter(t *testing.T) {
	input := []byte("id: test\nname: Test\n---\n")
	_, _, err := splitFrontmatter(input)
	if err == nil {
		t.Fatal("expected error for missing opening delimiter")
	}
}

func TestSplitFrontmatter_MissingCloseDelimiter(t *testing.T) {
	input := []byte("---\nid: test\nname: Test\n")
	_, _, err := splitFrontmatter(input)
	if err == nil {
		t.Fatal("expected error for missing closing delimiter")
	}
}

func TestParse_FullSpec(t *testing.T) {
	content := []byte(`---
id: test-agent
name: Test Agent
description: A test agent.
model: llama3.1:8b
context_budget: 4096
temperature: 0.2
allowed_skills:
  - skill-a
  - skill-b
latency_budget_ms: 20000
tools:
  - mytool
---

# Test constitution
`)
	spec, err := Parse(content, "agents/test-agent.md")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if spec.ID != "test-agent" {
		t.Errorf("ID = %q, want %q", spec.ID, "test-agent")
	}
	if spec.Name != "Test Agent" {
		t.Errorf("Name = %q, want %q", spec.Name, "Test Agent")
	}
	if spec.Model != "llama3.1:8b" {
		t.Errorf("Model = %q", spec.Model)
	}
	if spec.ContextBudget != 4096 {
		t.Errorf("ContextBudget = %d, want 4096", spec.ContextBudget)
	}
	if spec.Temperature != 0.2 {
		t.Errorf("Temperature = %f, want 0.2", spec.Temperature)
	}
	if len(spec.AllowedSkills) != 2 {
		t.Errorf("AllowedSkills length = %d, want 2", len(spec.AllowedSkills))
	}
	if spec.LatencyBudgetMs != 20000 {
		t.Errorf("LatencyBudgetMs = %d, want 20000", spec.LatencyBudgetMs)
	}
	if spec.Body != "# Test constitution" {
		t.Errorf("Body = %q, want %q", spec.Body, "# Test constitution")
	}
}

func TestParse_DefaultIDFromFilename(t *testing.T) {
	content := []byte(`---
name: No ID
description: desc
model: m
context_budget: 1
allowed_skills:
  - s
latency_budget_ms: 1
---
`)
	spec, err := Parse(content, "agents/my-agent.md")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if spec.ID != "my-agent" {
		t.Errorf("default ID = %q, want %q", spec.ID, "my-agent")
	}
}

func TestValidate_MissingRequired(t *testing.T) {
	s := &Spec{} // all empty
	err := s.Validate()
	if err == nil {
		t.Fatal("expected validation error for empty spec")
	}
}

func TestValidate_Valid(t *testing.T) {
	s := &Spec{
		ID:              "ok-agent",
		Name:            "OK Agent",
		Description:     "desc",
		Model:           "llama3.1:8b",
		ContextBudget:   4096,
		AllowedSkills:   []string{"some-skill"},
		LatencyBudgetMs: 10000,
	}
	if err := s.Validate(); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}
