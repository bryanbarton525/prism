package graphify

import (
	"path/filepath"
	"testing"
)

func TestBindingMatchesExactWorkspaceAndGeneration(t *testing.T) {
	workspace := t.TempDir()
	binding := Binding{
		Workspace: workspace, IndexPath: filepath.Join(workspace, ".graphify", "index"),
		UpstreamVersion: "1.0.0", SchemaVersion: "v1", GenerationFingerprint: "abc",
	}
	if err := binding.Matches(workspace, "abc"); err != nil {
		t.Fatal(err)
	}
	if err := binding.Matches(t.TempDir(), "abc"); err == nil {
		t.Fatal("expected workspace mismatch")
	}
	if err := binding.Matches(workspace, "changed"); err == nil {
		t.Fatal("expected stale binding")
	}
}

func TestValidateToolEnforcesFixedBoundedContract(t *testing.T) {
	if err := ValidateTool("query_graph", 1, []byte("query")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"call_tool", "run_command"} {
		if err := ValidateTool(name, 1, nil); err == nil {
			t.Fatalf("tool %q unexpectedly permitted", name)
		}
	}
	if err := ValidateTool("get_node", MaxQueryRounds+1, nil); err == nil {
		t.Fatal("expected round bound")
	}
	if err := ValidateTool("get_node", 1, make([]byte, MaxResultBytes+1)); err == nil {
		t.Fatal("expected payload bound")
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "graphify.yaml")
	if err := Save(path, Config{Binding: &Binding{
		Workspace: t.TempDir(), IndexPath: "/tmp/index", UpstreamVersion: "1", SchemaVersion: "v1", GenerationFingerprint: "a",
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
}
