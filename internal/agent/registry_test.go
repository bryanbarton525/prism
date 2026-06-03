package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func makeAgentDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	return dir
}

func TestRegistry_Load_And_Get(t *testing.T) {
	dir := makeAgentDir(t, map[string]string{
		"github-cli.md": validSpec,
		"README.md":     "ignored",
	})
	reg := NewRegistry(os.DirFS(dir))
	if err := reg.Load(); err != nil {
		t.Fatalf("Load(): %v", err)
	}

	spec, err := reg.Get("github-cli")
	if err != nil {
		t.Fatalf("Get(): %v", err)
	}
	if spec.ID != "github-cli" {
		t.Errorf("ID: want github-cli, got %s", spec.ID)
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	reg := NewRegistry(os.DirFS(makeAgentDir(t, map[string]string{})))
	if err := reg.Load(); err != nil {
		t.Fatalf("Load(): %v", err)
	}
	_, err := reg.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

func TestRegistry_List_Sorted(t *testing.T) {
	dir := makeAgentDir(t, map[string]string{
		"argo.md": `---
id: argo
name: Argo
description: d
model: m
context_budget: 1
allowed_skills: [s]
latency_budget_ms: 1
---
body`,
		"github-cli.md": validSpec,
	})
	reg := NewRegistry(os.DirFS(dir))
	if err := reg.Load(); err != nil {
		t.Fatalf("Load(): %v", err)
	}
	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("want 2 agents, got %d", len(list))
	}
	if list[0].ID != "argo" || list[1].ID != "github-cli" {
		t.Errorf("unexpected order: %v", list)
	}
}

func TestRegistry_Load_InvalidSpec(t *testing.T) {
	dir := makeAgentDir(t, map[string]string{
		"bad.md": "---\nid: bad\n---\nbody",
	})
	reg := NewRegistry(os.DirFS(dir))
	err := reg.Load()
	if err == nil {
		t.Fatal("expected error for invalid spec")
	}
}

func TestRegistry_Load_MissingDir(t *testing.T) {
	reg := NewRegistry(os.DirFS("/tmp/nonexistent-prism-test-dir"))
	if err := reg.Load(); err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestRegistry_Load_RealAgents(t *testing.T) {
	// Integration test against the real agents/ directory.
	reg := NewRegistry(os.DirFS("../../agents"))
	if err := reg.Load(); err != nil {
		t.Fatalf("loading real agents: %v", err)
	}
	list := reg.List()
	if len(list) == 0 {
		t.Fatal("expected at least one agent")
	}
	for _, s := range list {
		if s.ID == "" || s.Model == "" {
			t.Errorf("incomplete summary for agent: %+v", s)
		}
	}
}
