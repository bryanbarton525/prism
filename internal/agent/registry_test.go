package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRegistry_RealAgentDir(t *testing.T) {
	// Walk up from the test binary location to find the repo root.
	// During `go test ./...` the working directory is the package directory.
	wd, _ := os.Getwd()
	// wd = .../internal/agent; repo root = ../..
	agentDir := filepath.Join(wd, "..", "..", "agents")

	registry, warnings, err := LoadRegistry(agentDir)
	if err != nil {
		t.Fatalf("LoadRegistry error: %v", err)
	}
	for _, w := range warnings {
		t.Logf("warning: %v", w)
	}
	if registry.Len() == 0 {
		t.Error("expected at least one agent to be loaded")
	}
}

func TestLoadRegistry_GetExisting(t *testing.T) {
	wd, _ := os.Getwd()
	agentDir := filepath.Join(wd, "..", "..", "agents")

	registry, _, err := LoadRegistry(agentDir)
	if err != nil {
		t.Fatalf("LoadRegistry error: %v", err)
	}

	spec, err := registry.Get("github-cli")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if spec.ID != "github-cli" {
		t.Errorf("ID = %q, want github-cli", spec.ID)
	}
	if len(spec.AllowedSkills) == 0 {
		t.Error("expected allowed_skills to be non-empty")
	}
}

func TestLoadRegistry_GetMissing(t *testing.T) {
	wd, _ := os.Getwd()
	agentDir := filepath.Join(wd, "..", "..", "agents")

	registry, _, err := LoadRegistry(agentDir)
	if err != nil {
		t.Fatalf("LoadRegistry error: %v", err)
	}

	_, err = registry.Get("nonexistent-agent-xyz")
	if err == nil {
		t.Fatal("expected error for missing agent")
	}
}

func TestLoadRegistry_SortedList(t *testing.T) {
	wd, _ := os.Getwd()
	agentDir := filepath.Join(wd, "..", "..", "agents")

	registry, _, err := LoadRegistry(agentDir)
	if err != nil {
		t.Fatalf("LoadRegistry error: %v", err)
	}

	specs := registry.List()
	for i := 1; i < len(specs); i++ {
		if specs[i].ID < specs[i-1].ID {
			t.Errorf("list is not sorted: %q appears before %q", specs[i-1].ID, specs[i].ID)
		}
	}
}

func TestLoadRegistry_InvalidDir(t *testing.T) {
	_, _, err := LoadRegistry("/does/not/exist/at/all")
	if err == nil {
		t.Fatal("expected error for non-existent directory")
	}
}

func TestLoadRegistry_SkipREADME(t *testing.T) {
	tmp := t.TempDir()

	// Write a valid agent and a README.
	validMD := `---
id: dummy
name: Dummy
description: desc
model: llama3.1:8b
context_budget: 1024
allowed_skills:
  - skill-x
latency_budget_ms: 5000
---
# body
`
	readmeMD := "# README\nThis is a readme."
	if err := os.WriteFile(filepath.Join(tmp, "dummy.md"), []byte(validMD), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "README.md"), []byte(readmeMD), 0o644); err != nil {
		t.Fatal(err)
	}

	registry, warnings, err := LoadRegistry(tmp)
	if err != nil {
		t.Fatalf("LoadRegistry error: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if registry.Len() != 1 {
		t.Errorf("expected 1 agent, got %d", registry.Len())
	}
}
