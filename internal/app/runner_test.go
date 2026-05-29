package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	agentpkg "github.com/bryanbarton525/prism/internal/agent"
)

func makeTestRegistry(t *testing.T) (agentDir, skillsDir, repoRoot string) {
	t.Helper()
	root := t.TempDir()
	agentDir = filepath.Join(root, "agents")
	skillsDir = filepath.Join(root, "skills")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}

	agentContent := `---
id: test
name: Test
description: A test agent.
model: llama3.1:8b
context_budget: 4096
temperature: 0.1
allowed_skills:
  - test-skill
latency_budget_ms: 30000
---
# Test constitution body.
`
	if err := os.WriteFile(filepath.Join(agentDir, "test.md"), []byte(agentContent), 0o644); err != nil {
		t.Fatal(err)
	}

	skillDir := filepath.Join(skillsDir, "test-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillContent := `---
name: test-skill
description: A test skill.
---
# Instructions
Skill instructions.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	return agentDir, skillsDir, root
}

func TestListAgents(t *testing.T) {
	agentDir, skillsDir, root := makeTestRegistry(t)
	runner, err := New(Config{
		OllamaHost: "http://127.0.0.1:11434",
		AgentDir:   agentDir,
		SkillsDir:  skillsDir,
		RepoRoot:   root,
	})
	if err != nil {
		t.Fatal(err)
	}
	agents, err := runner.ListAgents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 1 || agents[0].ID != "test" {
		t.Errorf("unexpected agents: %v", agents)
	}
}

func TestGetConstitution(t *testing.T) {
	agentDir, skillsDir, root := makeTestRegistry(t)
	runner, err := New(Config{
		OllamaHost: "http://127.0.0.1:11434",
		AgentDir:   agentDir,
		SkillsDir:  skillsDir,
		RepoRoot:   root,
	})
	if err != nil {
		t.Fatal(err)
	}
	con, err := runner.GetConstitution(context.Background(), "test")
	if err != nil {
		t.Fatal(err)
	}
	if con.AgentID != "test" {
		t.Errorf("AgentID = %q", con.AgentID)
	}
	if con.Text == "" {
		t.Error("constitution text should not be empty")
	}
}

func TestGetConstitutionNotFound(t *testing.T) {
	agentDir, skillsDir, root := makeTestRegistry(t)
	runner, err := New(Config{
		OllamaHost: "http://127.0.0.1:11434",
		AgentDir:   agentDir,
		SkillsDir:  skillsDir,
		RepoRoot:   root,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = runner.GetConstitution(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for unknown agent")
	}
}

func TestDoctorDegraded(t *testing.T) {
	agentDir, skillsDir, root := makeTestRegistry(t)
	runner, err := New(Config{
		// Use a port that is guaranteed to be closed to simulate unreachable Ollama.
		OllamaHost: "http://127.0.0.1:1",
		AgentDir:   agentDir,
		SkillsDir:  skillsDir,
		RepoRoot:   root,
	})
	if err != nil {
		t.Fatal(err)
	}
	dr, err := runner.Doctor(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if dr.Status != "degraded" {
		t.Errorf("expected status=degraded, got %q", dr.Status)
	}
	if dr.AgentCount != 1 {
		t.Errorf("expected 1 agent, got %d", dr.AgentCount)
	}
}

func TestBuildPrompt(t *testing.T) {
	skills := []*agentpkg.SkillSpec{{Name: "my-skill", Body: "Do things carefully."}}
	prompt := buildPrompt("Agent mission here.", skills, "test task", "json")
	for _, want := range []string{"Agent mission here.", "my-skill", "Do things carefully.", "JSON"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestExtractSummary(t *testing.T) {
	t.Run("extracts first paragraph", func(t *testing.T) {
		raw := "First paragraph.\n\nSecond paragraph."
		got := extractSummary(raw)
		if got != "First paragraph." {
			t.Errorf("got %q", got)
		}
	})

	t.Run("truncates long single-paragraph", func(t *testing.T) {
		raw := strings.Repeat("x", 300)
		got := extractSummary(raw)
		if len(got) != 200 {
			t.Errorf("expected 200 chars, got %d", len(got))
		}
	})

	t.Run("empty input", func(t *testing.T) {
		if got := extractSummary(""); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}
