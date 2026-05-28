package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistry_Load(t *testing.T) {
	dir := t.TempDir()

	// Write two valid agent specs.
	writeSpec(t, dir, "alpha.md", `---
id: alpha
name: Alpha
description: Alpha agent.
model: llama3.1:8b
context_budget: 1024
allowed_skills:
  - skill-a
latency_budget_ms: 5000
---

Alpha constitution.
`)
	writeSpec(t, dir, "beta.md", `---
id: beta
name: Beta
description: Beta agent.
model: llama3.1:8b
context_budget: 2048
allowed_skills:
  - skill-b
latency_budget_ms: 10000
---

Beta constitution.
`)
	// README.md should be ignored.
	writeSpec(t, dir, "README.md", "# Readme\n")

	reg := NewRegistry(dir)
	if err := reg.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	summaries := reg.List()
	if len(summaries) != 2 {
		t.Fatalf("List(): want 2 agents, got %d", len(summaries))
	}
	if summaries[0].ID != "alpha" || summaries[1].ID != "beta" {
		t.Errorf("unexpected order/IDs: %v", summaries)
	}
}

func TestRegistry_GetMissing(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)
	_ = reg.Load()
	_, err := reg.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing agent")
	}
}

func writeSpec(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}
