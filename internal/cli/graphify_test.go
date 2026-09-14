package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bryanbarton525/prism/internal/downstreammcp"
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
	if err := os.WriteFile(index, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := newGraphifyBindCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{
		"--workspace", workspace, "--index", index, "--upstream-version", graphify.PinnedUpstreamVersion,
		"--schema-version", graphify.PinnedContractID, "--fingerprint", "source-sha", "--server", "graphify",
		"--endpoint-kind", "self-hosted", "--approve",
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
	if !cfg.OperatorApproved || cfg.Endpoint == nil || cfg.Endpoint.Server != "graphify" || cfg.Endpoint.Kind != graphify.EndpointSelfHosted {
		t.Fatalf("endpoint = %#v", cfg.Endpoint)
	}
}

func TestGraphifySetupRequiresApprovalAndDryRunDoesNotWrite(t *testing.T) {
	originalStateDir := gf.stateDir
	gf.stateDir = t.TempDir()
	t.Cleanup(func() { gf.stateDir = originalStateDir })
	workspace := t.TempDir()
	index := filepath.Join(workspace, "index")
	if err := os.WriteFile(index, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := newGraphifySetupCmd()
	cmd.SetArgs([]string{
		"--workspace", workspace, "--index", index, "--upstream-version", graphify.PinnedUpstreamVersion,
		"--schema-version", graphify.PinnedContractID, "--fingerprint", "source-sha", "--server", "graphify",
		"--endpoint-kind", "self-hosted",
	})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--approve") {
		t.Fatalf("expected approval error, got %v", err)
	}
	cmd = newGraphifySetupCmd()
	cmd.SetArgs([]string{
		"--workspace", workspace, "--index", index, "--upstream-version", graphify.PinnedUpstreamVersion,
		"--schema-version", graphify.PinnedContractID, "--fingerprint", "source-sha", "--server", "graphify",
		"--endpoint-kind", "self-hosted", "--approve", "--dry-run",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(gf.stateDir, "graphify.yaml")); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote config: %v", err)
	}
}

func TestGraphifyRemovePreservesExternalResources(t *testing.T) {
	originalStateDir := gf.stateDir
	gf.stateDir = t.TempDir()
	t.Cleanup(func() { gf.stateDir = originalStateDir })
	index := filepath.Join(t.TempDir(), "user-graph.json")
	if err := os.WriteFile(index, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(gf.stateDir, "graphify.yaml")
	if err := graphify.Save(config, graphify.Config{OperatorApproved: true}); err != nil {
		t.Fatal(err)
	}
	cmd := newGraphifyRemoveCmd()
	cmd.SetArgs([]string{"--approve"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(config); !os.IsNotExist(err) {
		t.Fatalf("configuration was not removed: %v", err)
	}
	if info, err := os.Stat(index); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("user-managed index changed: info=%#v err=%v", info, err)
	}
}

func TestGraphifyDoctorChecksRegisteredEndpointWithoutContactingIt(t *testing.T) {
	originalStateDir := gf.stateDir
	gf.stateDir = t.TempDir()
	t.Cleanup(func() { gf.stateDir = originalStateDir })
	workspace := t.TempDir()
	index := filepath.Join(workspace, "index")
	if err := os.WriteFile(index, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := graphify.Save(filepath.Join(gf.stateDir, "graphify.yaml"), graphify.Config{
		OperatorApproved: true,
		Binding: &graphify.Binding{
			Workspace: workspace, IndexPath: index, UpstreamVersion: graphify.PinnedUpstreamVersion,
			SchemaVersion: graphify.PinnedContractID, GenerationFingerprint: "source-sha",
		},
		Endpoint: &graphify.Endpoint{Server: "graphify", Kind: graphify.EndpointSelfHosted},
	}); err != nil {
		t.Fatal(err)
	}
	if err := downstreammcp.Save(mcpServersPath(), downstreammcp.State{Servers: []downstreammcp.Server{{
		Name: "graphify", Transport: downstreammcp.TransportSSE, URL: "http://127.0.0.1:1/mcp",
	}}}); err != nil {
		t.Fatal(err)
	}
	cmd := newGraphifyDoctorCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--workspace", workspace, "--fingerprint", "source-sha"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "ready") {
		t.Fatalf("doctor output = %q", out.String())
	}
}

func TestGraphifySetupRejectsUnpinnedContract(t *testing.T) {
	originalStateDir := gf.stateDir
	gf.stateDir = t.TempDir()
	t.Cleanup(func() { gf.stateDir = originalStateDir })
	workspace := t.TempDir()
	cmd := newGraphifySetupCmd()
	cmd.SetArgs([]string{
		"--workspace", workspace, "--index", filepath.Join(workspace, "index"), "--upstream-version", "v0.0.1",
		"--fingerprint", "fixture", "--server", "graphify", "--endpoint-kind", "self-hosted", "--approve",
	})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "pinned release") {
		t.Fatalf("error = %v", err)
	}
}
