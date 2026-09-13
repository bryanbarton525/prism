package cli

import (
	"path/filepath"
	"testing"

	"github.com/bryanbarton525/prism/internal/downstreammcp"
)

func TestPrintDownstreamMCPMutationConflict(t *testing.T) {
	err := printDownstreamMCPMutation("linear", downstreammcp.OutcomeConflict)
	if err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestPrintDownstreamMCPMutationSuccessOutcomes(t *testing.T) {
	for _, outcome := range []string{
		downstreammcp.OutcomeCreated,
		downstreammcp.OutcomeUnchanged,
		downstreammcp.OutcomeReplaced,
	} {
		if err := printDownstreamMCPMutation("linear", outcome); err != nil {
			t.Fatalf("outcome %s: %v", outcome, err)
		}
	}
}

func TestParseReferenceAssignments(t *testing.T) {
	got, err := parseReferenceAssignments([]string{
		"Authorization=OPENAI_TOKEN",
		"X-Key = SOME_ENV ",
	}, "--header-from")
	if err != nil {
		t.Fatal(err)
	}
	if got["Authorization"] != "OPENAI_TOKEN" {
		t.Fatalf("Authorization ref = %q", got["Authorization"])
	}
	if got["X-Key"] != "SOME_ENV" {
		t.Fatalf("X-Key ref = %q", got["X-Key"])
	}
}

func TestParseReferenceAssignmentsRejectsInvalid(t *testing.T) {
	_, err := parseReferenceAssignments([]string{"broken"}, "--header-from")
	if err == nil {
		t.Fatal("expected invalid assignment error")
	}
}

func TestMCPAddInfersStreamableHTTPFromURL(t *testing.T) {
	orig := gf.stateDir
	gf.stateDir = t.TempDir()
	defer func() { gf.stateDir = orig }()

	cmd := newMCPAddCmd()
	cmd.SetArgs([]string{"openaiDeveloperDocs", "--url", "https://developers.openai.com/mcp"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	state, err := downstreammcp.Load(filepath.Join(gf.stateDir, "mcp-servers.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	server, ok := state.Get("openaiDeveloperDocs")
	if !ok {
		t.Fatal("server missing")
	}
	if server.Transport != downstreammcp.TransportStreamableHTTP {
		t.Fatalf("transport = %s", server.Transport)
	}
}

func TestMCPAddAcceptsHTTPTransportAlias(t *testing.T) {
	orig := gf.stateDir
	gf.stateDir = t.TempDir()
	defer func() { gf.stateDir = orig }()

	cmd := newMCPAddCmd()
	cmd.SetArgs([]string{"docs", "--transport", "http", "--url", "https://developers.openai.com/mcp"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	state, err := downstreammcp.Load(filepath.Join(gf.stateDir, "mcp-servers.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	server, ok := state.Get("docs")
	if !ok {
		t.Fatal("server missing")
	}
	if server.Transport != downstreammcp.TransportStreamableHTTP {
		t.Fatalf("transport = %s", server.Transport)
	}
}

func TestMCPAddAcceptsCommandArgvAfterSeparator(t *testing.T) {
	orig := gf.stateDir
	gf.stateDir = t.TempDir()
	defer func() { gf.stateDir = orig }()

	cmd := newMCPAddCmd()
	cmd.SetArgs([]string{"linear", "--", "npx", "-y", "mcp-remote", "https://mcp.linear.app/mcp"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	state, err := downstreammcp.Load(filepath.Join(gf.stateDir, "mcp-servers.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	server, ok := state.Get("linear")
	if !ok {
		t.Fatal("server missing")
	}
	if server.Transport != downstreammcp.TransportCommand || server.Command != "npx" {
		t.Fatalf("server = %#v", server)
	}
	if len(server.Args) != 3 || server.Args[0] != "-y" {
		t.Fatalf("args = %#v", server.Args)
	}
}

func TestMCPServerAddCommandImplicitlyReplaces(t *testing.T) {
	orig := gf.stateDir
	gf.stateDir = t.TempDir()
	defer func() { gf.stateDir = orig }()

	cmd := newMCPServerAddCommandCmd()
	cmd.SetArgs([]string{"linear", "npx", "-y", "mcp-remote", "https://mcp.linear.app/mcp"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	cmd = newMCPServerAddCommandCmd()
	cmd.SetArgs([]string{"linear", "npx", "-y", "mcp-remote", "https://other.example/mcp"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	state, err := downstreammcp.Load(filepath.Join(gf.stateDir, "mcp-servers.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	server, ok := state.Get("linear")
	if !ok {
		t.Fatal("server missing")
	}
	if server.Args[len(server.Args)-1] != "https://other.example/mcp" {
		t.Fatalf("expected replacement, got args %#v", server.Args)
	}
}

func TestMCPAddRejectsMalformedInputs(t *testing.T) {
	orig := gf.stateDir
	gf.stateDir = t.TempDir()
	defer func() { gf.stateDir = orig }()

	cmd := newMCPAddCmd()
	cmd.SetArgs([]string{"bad", "--url", "ftp://example.com"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected invalid url scheme error")
	}

	cmd = newMCPAddCmd()
	cmd.SetArgs([]string{"bad", "--env-from", "broken", "--", "echo"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected malformed env assignment error")
	}

	cmd = newMCPAddCmd()
	cmd.SetArgs([]string{"bad", "--timeout-ms", "0", "--", "echo"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected non-positive timeout error")
	}
}
