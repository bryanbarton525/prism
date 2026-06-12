package bundles

import (
	"path/filepath"
	"testing"

	bundlepkg "github.com/bryanbarton525/prism/pkg/bundle"
)

func TestPromoteAndDeprecateInstalledBundle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bundles.yaml")
	if err := Save(path, State{Bundles: []bundlepkg.Installed{{
		ID:          "k8s-core-triage",
		Version:     "0.1.0",
		Channel:     "canary",
		InstalledAt: "2026-06-11T00:00:00Z",
	}}}); err != nil {
		t.Fatal(err)
	}
	if err := Promote(path, "k8s-core-triage", "stable"); err != nil {
		t.Fatal(err)
	}
	if err := Deprecate(path, "k8s-core-triage", "deprecated"); err != nil {
		t.Fatal(err)
	}
	state, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if state.Bundles[0].Channel != "stable" || state.Bundles[0].DeprecationStatus != "deprecated" {
		t.Fatalf("state = %#v", state)
	}
}
