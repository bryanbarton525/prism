package cli

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/bryanbarton525/prism/internal/graphify"
)

func TestGraphifyBindRequiresExplicitValues(t *testing.T) {
	cmd := newGraphifyBindCmd()
	cmd.SetArgs([]string{"--workspace", t.TempDir()})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected required flag error")
	}
}

func TestGraphifyBindWritesExplicitBinding(t *testing.T) {
	originalStateDir := gf.stateDir
	gf.stateDir = t.TempDir()
	t.Cleanup(func() { gf.stateDir = originalStateDir })
	workspace := t.TempDir()
	index := filepath.Join(workspace, "index")
	cmd := newGraphifyBindCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{
		"--workspace", workspace, "--index", index, "--upstream-version", "1.2.3",
		"--schema-version", "v1", "--fingerprint", "source-sha", "--server", "graphify",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	cfg, err := graphify.Load(filepath.Join(gf.stateDir, "graphify.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Binding == nil || cfg.Binding.Workspace != workspace || cfg.Binding.IndexPath != index {
		t.Fatalf("binding = %#v", cfg.Binding)
	}
	if cfg.Endpoint == nil || cfg.Endpoint.Server != "graphify" {
		t.Fatalf("endpoint = %#v", cfg.Endpoint)
	}
}
