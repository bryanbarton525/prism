package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bryanbarton525/prism/internal/downstreammcp"
	"github.com/bryanbarton525/prism/internal/extensions"
	"github.com/bryanbarton525/prism/internal/graphify"
)

func TestPrintDownstreamMCPMutationConflict(t *testing.T) {
	err := printDownstreamMCPMutation("linear", downstreammcp.OutcomeConflict)
	if err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestMCPRemoveRejectsAtomicAccessAndGraphifyReferences(t *testing.T) {
	orig := gf.stateDir
	gf.stateDir = t.TempDir()
	t.Cleanup(func() { gf.stateDir = orig })
	server := downstreammcp.Server{Name: "graphify", Transport: downstreammcp.TransportStreamableHTTP, URL: "https://graphify.example/mcp"}
	if err := downstreammcp.Save(mcpServersPath(), downstreammcp.State{Servers: []downstreammcp.Server{server}}); err != nil {
		t.Fatal(err)
	}
	if _, err := extensions.UpdateMCPAccess(context.Background(), gf.stateDir, func(state *extensions.MCPAccessState) error { state.DefaultServers = []string{"graphify"}; return nil }); err != nil {
		t.Fatal(err)
	}
	cfg := graphify.Config{OperatorApproved: true, Endpoint: &graphify.Endpoint{Server: "graphify", Kind: graphify.EndpointSelfHosted}, Binding: &graphify.Binding{Workspace: t.TempDir(), IndexPath: filepath.Join(t.TempDir(), "graph.json"), UpstreamVersion: graphify.PinnedUpstreamVersion, SchemaVersion: graphify.PinnedContractID, GenerationFingerprint: "sha"}}
	if err := graphify.Save(filepath.Join(gf.stateDir, "graphify.yaml"), cfg); err != nil {
		t.Fatal(err)
	}
	cmd := newMCPRemoveCmd()
	cmd.SetArgs([]string{"graphify"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "default access set") || !strings.Contains(err.Error(), "Graphify endpoint") {
		t.Fatalf("reference error = %v", err)
	}
	state, err := downstreammcp.Load(mcpServersPath())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := state.Get("graphify"); !ok {
		t.Fatal("referenced server was removed")
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

func TestMCPAddRejectsURLCredentials(t *testing.T) {
	orig := gf.stateDir
	gf.stateDir = t.TempDir()
	t.Cleanup(func() { gf.stateDir = orig })
	cmd := newMCPAddCmd()
	cmd.SetArgs([]string{"secret", "--url", "https://user:password@example.com/mcp"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "userinfo") || strings.Contains(err.Error(), "password") {
		t.Fatalf("URL credentials rejection = %v", err)
	}
	state, err := downstreammcp.Load(mcpServersPath())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := state.Get("secret"); ok {
		t.Fatal("credential-bearing endpoint was persisted")
	}
}

func TestHTTPURLValidationDistinguishesURLFromFilesystemPath(t *testing.T) {
	if err := validateAbsoluteHTTPURL("https://example.com:8443/mcp"); err != nil {
		t.Fatalf("valid URL with colon rejected: %v", err)
	}
	for _, raw := range []string{"/tmp/mcp", `\\server\share`, "C:/mcp"} {
		if err := validateAbsoluteHTTPURL(raw); err == nil {
			t.Fatalf("filesystem path accepted as URL: %q", raw)
		}
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

func TestMCPAccessCommandsPersistState(t *testing.T) {
	orig := gf.stateDir
	gf.stateDir = t.TempDir()
	defer func() { gf.stateDir = orig }()
	if err := downstreammcp.Save(mcpServersPath(), downstreammcp.State{Servers: []downstreammcp.Server{
		{Name: "linear", Transport: downstreammcp.TransportCommand, Command: "linear"},
		{Name: "docs", Transport: downstreammcp.TransportCommand, Command: "docs"},
	}}); err != nil {
		t.Fatal(err)
	}

	cmd := newMCPAccessDefaultSetCmd()
	cmd.SetArgs([]string{"--server", "linear", "--server", "docs"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	cmd = newMCPAccessAgentSetCmd()
	cmd.SetArgs([]string{"github-cli", "--mode", "custom", "--server", "docs"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	state, configured, err := extensions.LoadMCPAccess(gf.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("expected configured state")
	}
	if len(state.DefaultServers) != 2 {
		t.Fatalf("defaults = %#v", state.DefaultServers)
	}
	rule, ok := state.Agents["github-cli"]
	if !ok || rule.Mode != extensions.MCPAccessModeCustom || len(rule.Servers) != 1 || rule.Servers[0] != "docs" {
		t.Fatalf("rule = %#v", rule)
	}
}
