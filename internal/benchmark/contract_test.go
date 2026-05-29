// Package benchmark holds contract and golden tests for Prism metrics fixtures.
package benchmark

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bryanbarton525/prism/internal/agent"
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
