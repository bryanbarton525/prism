package cli

import (
	"testing"

	"github.com/bryanbarton525/prism/internal/downstreammcp"
)

func TestWithConfiguredLinearMCPAddsServerFromURL(t *testing.T) {
	state, err := withConfiguredLinearMCP(downstreammcp.State{}, "https://mcp.linear.app/mcp")
	if err != nil {
		t.Fatal(err)
	}
	server, ok := state.Get("linear")
	if !ok {
		t.Fatal("linear server missing")
	}
	if server.Transport != downstreammcp.TransportCommand || server.Command != "npx" {
		t.Fatalf("server = %#v", server)
	}
	if len(server.Args) != 3 || server.Args[2] != "https://mcp.linear.app/mcp" {
		t.Fatalf("args = %#v", server.Args)
	}
}

func TestWithConfiguredLinearMCPPreservesExplicitServer(t *testing.T) {
	state := downstreammcp.State{}
	state.Upsert(downstreammcp.Server{
		Name:      "linear",
		Transport: downstreammcp.TransportSSE,
		URL:       "https://example.test/linear",
	})

	merged, err := withConfiguredLinearMCP(state, "https://mcp.linear.app/mcp")
	if err != nil {
		t.Fatal(err)
	}
	server, ok := merged.Get("linear")
	if !ok {
		t.Fatal("linear server missing")
	}
	if server.Transport != downstreammcp.TransportSSE || server.URL != "https://example.test/linear" {
		t.Fatalf("server = %#v", server)
	}
}

func TestWithConfiguredLinearMCPRejectsInvalidURL(t *testing.T) {
	if _, err := withConfiguredLinearMCP(downstreammcp.State{}, "linear"); err == nil {
		t.Fatal("expected invalid URL error")
	}
}
