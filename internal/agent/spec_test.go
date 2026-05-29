package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	t.Run("valid frontmatter and body", func(t *testing.T) {
		input := "---\nid: test\nname: Test\n---\n# Body\n\nContent here."
		fm, body, err := parseFrontmatter(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if fm != "id: test\nname: Test" {
			t.Errorf("frontmatter = %q", fm)
		}
		if body != "# Body\n\nContent here." {
			t.Errorf("body = %q", body)
		}
	})

	t.Run("no frontmatter returns body unchanged", func(t *testing.T) {
		input := "# Just a body\n"
		fm, body, err := parseFrontmatter(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if fm != "" {
			t.Errorf("expected empty frontmatter, got %q", fm)
		}
		if body != input {
			t.Errorf("body = %q", body)
		}
	})

	t.Run("missing closing delimiter returns error", func(t *testing.T) {
		input := "---\nid: test\n"
		_, _, err := parseFrontmatter(input)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestLoadSpec(t *testing.T) {
	dir := t.TempDir()
	content := `---
id: test-agent
name: Test Agent
description: A test agent.
model: llama3.1:8b
context_budget: 4096
temperature: 0.1
allowed_skills:
  - skill-a
latency_budget_ms: 30000
---

# Constitution

Agent mission.
`
	path := filepath.Join(dir, "test-agent.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	spec, err := LoadSpec(path)
	if err != nil {
		t.Fatalf("LoadSpec error: %v", err)
	}

	if spec.ID != "test-agent" {
		t.Errorf("ID = %q, want %q", spec.ID, "test-agent")
	}
	if spec.Name != "Test Agent" {
		t.Errorf("Name = %q, want %q", spec.Name, "Test Agent")
	}
	if spec.Model != "llama3.1:8b" {
		t.Errorf("Model = %q, want %q", spec.Model, "llama3.1:8b")
	}
	if len(spec.AllowedSkills) != 1 || spec.AllowedSkills[0] != "skill-a" {
		t.Errorf("AllowedSkills = %v", spec.AllowedSkills)
	}
	if spec.Body == "" {
		t.Error("Body should not be empty")
	}
}

func TestSpecValidate(t *testing.T) {
	t.Run("valid spec passes", func(t *testing.T) {
		s := &Spec{ID: "x", Name: "X", Description: "desc", Model: "m"}
		if err := s.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing required fields", func(t *testing.T) {
		s := &Spec{ID: "x"}
		if err := s.Validate(); err == nil {
			t.Error("expected error for missing fields")
		}
	})
}

func TestLoadRegistry(t *testing.T) {
	agentDir := t.TempDir()
	skillsDir := t.TempDir()

	// Create a minimal agent spec.
	agentContent := `---
id: test
name: Test
description: Testing.
model: llama3.1:8b
allowed_skills:
  - test-skill
latency_budget_ms: 10000
---
# Constitution
`
	if err := os.WriteFile(filepath.Join(agentDir, "test.md"), []byte(agentContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a skill directory.
	skillDir := filepath.Join(skillsDir, "test-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillContent := `---
name: test-skill
description: A test skill.
---
# Instructions
Do testing things.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	reg, err := LoadRegistry(agentDir, skillsDir)
	if err != nil {
		t.Fatalf("LoadRegistry error: %v", err)
	}

	if _, ok := reg.Agents["test"]; !ok {
		t.Error("expected 'test' agent in registry")
	}
	if _, ok := reg.Skills["test-skill"]; !ok {
		t.Error("expected 'test-skill' in registry")
	}
}
