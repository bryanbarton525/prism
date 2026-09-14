package graphify

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBindingMatchesExactWorkspaceAndGeneration(t *testing.T) {
	workspace := t.TempDir()
	binding := Binding{
		Workspace: workspace, IndexPath: filepath.Join(workspace, ".graphify", "index"),
		UpstreamVersion: PinnedUpstreamVersion, SchemaVersion: PinnedContractID, GenerationFingerprint: "abc",
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
	index := filepath.Join(t.TempDir(), "index")
	if err := Save(path, Config{Binding: &Binding{
		Workspace: t.TempDir(), IndexPath: index, UpstreamVersion: PinnedUpstreamVersion, SchemaVersion: PinnedContractID, GenerationFingerprint: "a",
	}, OperatorApproved: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("version: 1\nunknown: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestEndpointRequiresExplicitServer(t *testing.T) {
	if err := (Endpoint{}).Validate(); err == nil {
		t.Fatal("expected missing endpoint server error")
	}
	path := filepath.Join(t.TempDir(), "graphify.yaml")
	if err := Save(path, Config{OperatorApproved: true, Endpoint: &Endpoint{Server: "graphify", Kind: EndpointSelfHosted}}); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Endpoint == nil || cfg.Endpoint.Server != "graphify" {
		t.Fatalf("endpoint = %#v", cfg.Endpoint)
	}
}

func TestCheckReadinessDoesNotTreatMissingIndexAsReady(t *testing.T) {
	workspace := t.TempDir()
	binding := &Binding{
		Workspace: workspace, IndexPath: filepath.Join(workspace, "missing-index"),
		UpstreamVersion: PinnedUpstreamVersion, SchemaVersion: PinnedContractID, GenerationFingerprint: "abc",
	}
	cfg := Config{
		Version: ConfigVersion, OperatorApproved: true, Binding: binding,
		Endpoint: &Endpoint{Server: "graphify", Kind: EndpointSelfHosted},
	}
	if ready := CheckReadiness(cfg, workspace, "abc"); ready.Ready {
		t.Fatal("missing index unexpectedly ready")
	}
	if err := os.WriteFile(binding.IndexPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if ready := CheckReadiness(cfg, workspace, "abc"); !ready.Ready {
		t.Fatalf("readiness = %#v", ready)
	}
	directoryIndex := filepath.Join(workspace, "directory-index")
	if err := os.Mkdir(directoryIndex, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg.Binding.IndexPath = directoryIndex
	if ready := CheckReadiness(cfg, workspace, "abc"); ready.Ready {
		t.Fatal("index directory unexpectedly accepted as a graph.json file")
	}
}

func TestEndpointKindsRequireTheirOwnershipMetadata(t *testing.T) {
	tests := []struct {
		name     string
		endpoint Endpoint
		wantErr  bool
	}{
		{name: "local missing executable", endpoint: Endpoint{Server: "graphify", Kind: EndpointLocal}, wantErr: true},
		{name: "self hosted", endpoint: Endpoint{Server: "graphify", Kind: EndpointSelfHosted}},
		{name: "managed missing pin", endpoint: Endpoint{Server: "graphify", Kind: EndpointManaged}, wantErr: true},
		{name: "managed pinned", endpoint: Endpoint{Server: "graphify", Kind: EndpointManaged, Environment: "prod", EnvironmentVersion: "2026.09.1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.endpoint.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}

func TestCheckReadinessRequiresOperatorApprovalAndSupportedSchema(t *testing.T) {
	workspace := t.TempDir()
	index := filepath.Join(workspace, "index")
	if err := os.WriteFile(index, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Version: ConfigVersion,
		Binding: &Binding{
			Workspace: workspace, IndexPath: index, UpstreamVersion: "1.0.0",
			SchemaVersion: "v2", GenerationFingerprint: "abc",
		},
		Endpoint: &Endpoint{Server: "graphify", Kind: EndpointSelfHosted},
	}
	ready := CheckReadiness(cfg, workspace, "abc")
	if ready.Ready {
		t.Fatalf("unapproved unsupported configuration ready: %#v", ready)
	}
	if len(ready.Checks) < 4 || ready.Checks[0].Name != "operator_approval" || ready.Checks[0].Ready {
		t.Fatalf("checks = %#v", ready.Checks)
	}
}

func TestPinnedContractFixtureMatchesRuntimeContract(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "graphify", "upstream-v0.9.61", "mcp-tools.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		UpstreamTag    string         `json:"upstream_tag"`
		UpstreamCommit string         `json:"upstream_commit"`
		ContractID     string         `json:"contract_id"`
		Tools          []ToolContract `json:"tools"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.UpstreamTag != PinnedUpstreamVersion || fixture.UpstreamCommit != PinnedUpstreamCommit || fixture.ContractID != PinnedContractID {
		t.Fatalf("fixture pin = %#v", fixture)
	}
	if err := ValidatePinnedToolInventory(fixture.Tools); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePinnedToolInventory(fixture.Tools[:len(fixture.Tools)-1]); err == nil {
		t.Fatal("missing approved tool did not fail contract validation")
	}
	drifted := append([]ToolContract{}, fixture.Tools...)
	drifted[0].InputSchema = map[string]any{"type": "object"}
	if err := ValidatePinnedToolInventory(drifted); err == nil {
		t.Fatal("schema drift did not fail contract validation")
	}
}

func TestValidateToolArgumentsBoundsAndWorkspaceIsolation(t *testing.T) {
	tests := []struct {
		name    string
		tool    string
		args    map[string]any
		wantErr bool
	}{
		{name: "bounded query", tool: "query_graph", args: map[string]any{"question": "find root", "depth": 3, "token_budget": 512}},
		{name: "depth too broad", tool: "query_graph", args: map[string]any{"question": "find root", "depth": 4}, wantErr: true},
		{name: "foreign project path", tool: "query_graph", args: map[string]any{"question": "find root", "project_path": "/other"}, wantErr: true},
		{name: "valid node", tool: "get_node", args: map[string]any{"label": "cli-root"}},
		{name: "legacy id argument", tool: "get_node", args: map[string]any{"id": "cli-root"}, wantErr: true},
		{name: "nil relation filter", tool: "get_neighbors", args: map[string]any{"label": "cli-root", "relation_filter": nil}, wantErr: true},
		{name: "path too long", tool: "shortest_path", args: map[string]any{"source": "a", "target": "b", "max_hops": 7}, wantErr: true},
		{name: "overflowed number", tool: "shortest_path", args: map[string]any{"source": "a", "target": "b", "max_hops": json.Number("9223372036854775808")}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateToolArguments(tt.tool, tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateToolArguments() error = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}

func TestVerifyGraphSourcesBoundsReadsAndRejectsUnsafePaths(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	output := "Source: main.go L2\nSource: ../secret.go L1\n"
	verifications := VerifyGraphSources(os.DirFS(workspace), output)
	if len(verifications) != 2 || !verifications[0].Verified || verifications[1].Verified {
		t.Fatalf("verifications = %#v", verifications)
	}
	if !strings.Contains(verifications[0].Excerpt, "func main") || !strings.Contains(verifications[1].Reason, "unsafe") {
		t.Fatalf("verifications = %#v", verifications)
	}
}

func TestGraphifyFixtureEvaluation(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "cmd", "prism"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := []byte("package main\nfunc main() {}\n")
	if err := os.WriteFile(filepath.Join(workspace, "cmd", "prism", "main.go"), source, 0o600); err != nil {
		t.Fatal(err)
	}
	graphOutput, err := os.ReadFile(filepath.Join("..", "..", "testdata", "graphify", "fake-mcp", "query_graph.txt"))
	if err != nil {
		t.Fatal(err)
	}
	index, err := os.ReadFile(filepath.Join("..", "..", "testdata", "graphify", "fake-index", "graph.json"))
	if err != nil {
		t.Fatal(err)
	}
	verifications := VerifyGraphSources(os.DirFS(workspace), string(graphOutput))
	if len(verifications) != 1 || !verifications[0].Verified {
		t.Fatalf("Graphify source verification = %#v", verifications)
	}
	graphContextBytes := len(graphOutput) + len(verifications[0].Excerpt)
	sourceOnlyContextBytes := len(source)
	t.Logf("fixture comparison: graph={useful:%t verified:%t tool_calls:%d bytes:%d context:%d} source_only={useful:%t verified:%t tool_calls:%d bytes:%d context:%d}",
		true, verifications[0].Verified, 1, len(graphOutput), graphContextBytes,
		true, true, 0, len(source), sourceOnlyContextBytes)
	if len(graphOutput) > MaxGraphResponseBytes || len(index) == 0 || graphContextBytes > MaxResultBytes {
		t.Fatalf("fixture bounds exceeded: graph=%d index=%d context=%d", len(graphOutput), len(index), graphContextBytes)
	}
}

func TestGraphifyDocumentationMatchesPinnedContract(t *testing.T) {
	for _, name := range []string{
		filepath.Join("..", "..", "README.md"),
		filepath.Join("..", "..", "docs", "usage.md"),
		filepath.Join("..", "..", "docs", "acceptance-matrix.md"),
	} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, marker := range []string{PinnedUpstreamVersion, PinnedContractID} {
			if !strings.Contains(text, marker) {
				t.Fatalf("%s missing %q", name, marker)
			}
		}
	}
	usage, err := os.ReadFile(filepath.Join("..", "..", "docs", "usage.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, stale := range []string{"--upstream-version 1.2.3", "--schema-version v1", "Tool calling is not exposed"} {
		if strings.Contains(string(usage), stale) {
			t.Fatalf("usage retains stale claim %q", stale)
		}
	}
}
