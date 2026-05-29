// Package benchmark holds contract and golden tests for Prism metrics fixtures.
package benchmark

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/app"
	"github.com/bryanbarton525/prism/internal/skill"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "agents")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root")
		}
		dir = parent
	}
}

// TestAgentSpecsLoad validates every agents/*.md spec in the repository.
func TestAgentSpecsLoad(t *testing.T) {
	root := repoRoot(t)
	reg := agent.NewRegistry(filepath.Join(root, "agents"))
	if err := reg.Load(); err != nil {
		t.Fatal(err)
	}
	list := reg.List()
	if len(list) < 4 {
		t.Fatalf("expected at least 4 agents, got %d", len(list))
	}
}

// TestSkillsDiscover validates skill layout (SKILL.md, references/, scripts/).
func TestSkillsDiscover(t *testing.T) {
	root := repoRoot(t)
	skills, err := skill.DiscoverAll(filepath.Join(root, "skills"))
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) < 8 {
		t.Fatalf("expected at least 8 skills, got %d", len(skills))
	}
}

// TestAgentSkillAllowlists ensures every allowed_skills entry resolves to a skill directory.
func TestAgentSkillAllowlists(t *testing.T) {
	root := repoRoot(t)
	skills, err := skill.DiscoverAll(filepath.Join(root, "skills"))
	if err != nil {
		t.Fatal(err)
	}
	index := make(map[string]struct{}, len(skills))
	for _, sk := range skills {
		index[sk.Name] = struct{}{}
	}

	reg := agent.NewRegistry(filepath.Join(root, "agents"))
	if err := reg.Load(); err != nil {
		t.Fatal(err)
	}
	for _, sum := range reg.List() {
		spec, err := reg.Get(sum.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range spec.AllowedSkills {
			if _, ok := index[name]; !ok {
				t.Errorf("agent %q references unknown skill %q", spec.ID, name)
			}
		}
	}
}

// TestGoldenPromptAssembly_githubCLI verifies prompt assembly uses constitution + attached skill only.
func TestGoldenPromptAssembly_githubCLI(t *testing.T) {
	root := repoRoot(t)
	reg := agent.NewRegistry(filepath.Join(root, "agents"))
	if err := reg.Load(); err != nil {
		t.Fatal(err)
	}
	spec, err := reg.Get("github-cli")
	if err != nil {
		t.Fatal(err)
	}
	constitution, _, err := spec.ResolveConstitution(root)
	if err != nil {
		t.Fatal(err)
	}
	skills, err := skill.LoadMany(filepath.Join(root, "skills"), []string{"gh-pr-triage"})
	if err != nil {
		t.Fatal(err)
	}
	system, user := app.AssemblePromptForTest(constitution, skills, []string{"gh-pr-triage"}, "Summarize PR #1 status.")
	if user != "Summarize PR #1 status." {
		t.Fatalf("user prompt: got %q", user)
	}
	for _, marker := range []string{
		"gh-pr-triage",
		"# Attached Agent Skills",
	} {
		if !strings.Contains(system, marker) {
			t.Errorf("system prompt missing %q", marker)
		}
	}
	// Another agent's skill must not appear.
	if strings.Contains(system, "kubectl-triage") {
		t.Error("system prompt must not include unrelated skills")
	}
}

// TestBenchmarkThresholdsFile ensures CI threshold config is present and parseable.
func TestBenchmarkThresholdsFile(t *testing.T) {
	root := repoRoot(t)
	th, err := LoadThresholds(root)
	if err != nil {
		t.Fatal(err)
	}
	if th.TokenReductionPercentMin <= 0 {
		t.Fatal("token_reduction_percent_min must be positive")
	}
}
