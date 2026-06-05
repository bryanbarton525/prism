package cli

import (
	"path/filepath"
	"testing"

	"github.com/bryanbarton525/prism/internal/bundles"
	bundlepkg "github.com/bryanbarton525/prism/pkg/bundle"
)

func TestResolveBundleProvenanceExplicitVersion(t *testing.T) {
	id, version, err := resolveBundleProvenance(filepath.Join(t.TempDir(), "missing.yaml"), "k8s-core-triage", "0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if id != "k8s-core-triage" || version != "0.2.0" {
		t.Fatalf("bundle provenance = %q@%q", id, version)
	}
}

func TestResolveBundleProvenanceFromInstalledState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bundles.yaml")
	if err := bundles.Save(path, bundles.State{Bundles: []bundlepkg.Installed{{
		ID:      "k8s-core-triage",
		Version: "0.1.0",
	}}}); err != nil {
		t.Fatal(err)
	}

	id, version, err := resolveBundleProvenance(path, "k8s-core-triage", "")
	if err != nil {
		t.Fatal(err)
	}
	if id != "k8s-core-triage" || version != "0.1.0" {
		t.Fatalf("bundle provenance = %q@%q", id, version)
	}
}

func TestResolveBundleProvenanceRequiresIDForVersion(t *testing.T) {
	_, _, err := resolveBundleProvenance(filepath.Join(t.TempDir(), "bundles.yaml"), "", "0.1.0")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveBundleProvenanceRequiresInstalledBundleWhenVersionOmitted(t *testing.T) {
	_, _, err := resolveBundleProvenance(filepath.Join(t.TempDir(), "bundles.yaml"), "k8s-core-triage", "")
	if err == nil {
		t.Fatal("expected error")
	}
}
