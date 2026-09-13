package graphify

import (
	"os"
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
	index := filepath.Join(t.TempDir(), "index")
	if err := Save(path, Config{Binding: &Binding{
		Workspace: t.TempDir(), IndexPath: index, UpstreamVersion: "1", SchemaVersion: "v1", GenerationFingerprint: "a",
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
		UpstreamVersion: "1", SchemaVersion: "v1", GenerationFingerprint: "abc",
	}
	cfg := Config{
		Version: ConfigVersion, OperatorApproved: true, Binding: binding,
		Endpoint: &Endpoint{Server: "graphify", Kind: EndpointSelfHosted},
	}
	if ready := CheckReadiness(cfg, workspace, "abc"); ready.Ready {
		t.Fatal("missing index unexpectedly ready")
	}
	if err := os.Mkdir(binding.IndexPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if ready := CheckReadiness(cfg, workspace, "abc"); !ready.Ready {
		t.Fatalf("readiness = %#v", ready)
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
	if err := os.Mkdir(index, 0o755); err != nil {
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
