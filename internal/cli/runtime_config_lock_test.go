package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/downstreammcp"
	"github.com/bryanbarton525/prism/internal/extensions"
	"github.com/bryanbarton525/prism/internal/graphify"
)

func TestRuntimeConfigLockSerializesCrossFileMutations(t *testing.T) {
	old := gf.stateDir
	gf.stateDir = t.TempDir()
	t.Cleanup(func() { gf.stateDir = old })
	unlock, err := acquireRuntimeConfigLock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	if _, err := acquireRuntimeConfigLock(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("competing mutation was not serialized: %v", err)
	}
}

func TestConcurrentRuntimeConfigurationMutationsPreserveAllFiles(t *testing.T) {
	old := gf.stateDir
	gf.stateDir = t.TempDir()
	t.Cleanup(func() { gf.stateDir = old })
	initial := downstreammcp.State{Servers: []downstreammcp.Server{
		{Name: "docs", Transport: downstreammcp.TransportCommand, Command: "old-docs"},
		{Name: "graphify", Transport: downstreammcp.TransportStreamableHTTP, URL: "https://graphify.example/mcp"},
		{Name: "obsolete", Transport: downstreammcp.TransportCommand, Command: "obsolete"},
	}}
	if err := downstreammcp.Save(mcpServersPath(), initial); err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	index := filepath.Join(workspace, "graph.json")
	if err := os.WriteFile(index, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	commands := []*cobra.Command{newMCPAccessDefaultSetCmd(), newMCPAccessAgentSetCmd(), newGraphifySetupCmd(), newMCPAddCmd(), newMCPRemoveCmd()}
	commands[0].SetArgs([]string{"--server", "docs"})
	commands[1].SetArgs([]string{"worker", "--mode", "custom", "--server", "graphify"})
	commands[2].SetArgs([]string{"--workspace", workspace, "--index", index, "--fingerprint", "sha", "--server", "graphify", "--endpoint-kind", "self-hosted", "--approve"})
	commands[3].SetArgs([]string{"docs", "--replace", "--", "new-docs"})
	commands[4].SetArgs([]string{"obsolete"})
	start := make(chan struct{})
	results := make(chan error, len(commands))
	for _, cmd := range commands {
		go func(cmd *cobra.Command) { <-start; results <- cmd.ExecuteContext(context.Background()) }(cmd)
	}
	close(start)
	for range commands {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	access, configured, err := extensions.LoadMCPAccess(gf.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if !configured || len(access.DefaultServers) != 1 || access.DefaultServers[0] != "docs" || access.Agents["worker"].Servers[0] != "graphify" {
		t.Fatalf("access updates lost: %#v", access)
	}
	servers, err := downstreammcp.Load(mcpServersPath())
	if err != nil {
		t.Fatal(err)
	}
	if docs, ok := servers.Get("docs"); !ok || docs.Command != "new-docs" {
		t.Fatalf("server update lost: %#v", docs)
	}
	if _, ok := servers.Get("obsolete"); ok {
		t.Fatal("server removal lost")
	}
	if cfg, err := graphify.Load(filepath.Join(gf.stateDir, "graphify.yaml")); err != nil || cfg.Endpoint == nil || cfg.Endpoint.Server != "graphify" {
		t.Fatalf("Graphify binding lost: %#v %v", cfg, err)
	}
}
