package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/bryanbarton525/prism/internal/bundles"
)

func TestResolveRegistrySourceArgLocal(t *testing.T) {
	old := gf
	t.Cleanup(func() { gf = old })
	gf.stateDir = t.TempDir()
	if err := bundles.SaveSources(registrySourcesPath(), bundles.RegistrySources{
		Sources: []bundles.RegistrySource{{Name: "local", URL: "/repo/registry"}},
	}); err != nil {
		t.Fatal(err)
	}

	manifest, sourceRoot, err := resolveRegistrySourceArg("local", "k8s/registry.json", "")
	if err != nil {
		t.Fatal(err)
	}
	if manifest != filepath.Join("/repo/registry", "k8s", "registry.json") {
		t.Fatalf("manifest = %q", manifest)
	}
	if sourceRoot != "/repo/registry" {
		t.Fatalf("sourceRoot = %q", sourceRoot)
	}
}

func TestResolveRegistrySourceArgRemote(t *testing.T) {
	old := gf
	t.Cleanup(func() { gf = old })
	gf.stateDir = t.TempDir()
	if err := bundles.SaveSources(registrySourcesPath(), bundles.RegistrySources{
		Sources: []bundles.RegistrySource{{Name: "remote", URL: "https://example.com/prism-registry"}},
	}); err != nil {
		t.Fatal(err)
	}

	manifest, sourceRoot, err := resolveRegistrySourceArg("remote", "k8s-core-triage/registry.json", "")
	if err != nil {
		t.Fatal(err)
	}
	if manifest != "https://example.com/prism-registry/k8s-core-triage/registry.json" {
		t.Fatalf("manifest = %q", manifest)
	}
	if sourceRoot != "https://example.com/prism-registry" {
		t.Fatalf("sourceRoot = %q", sourceRoot)
	}
}

func TestResolveRegistrySourceArgRejectsEscapingPath(t *testing.T) {
	old := gf
	t.Cleanup(func() { gf = old })
	gf.stateDir = t.TempDir()
	if err := bundles.SaveSources(registrySourcesPath(), bundles.RegistrySources{
		Sources: []bundles.RegistrySource{{Name: "local", URL: "/repo/registry"}},
	}); err != nil {
		t.Fatal(err)
	}

	_, _, err := resolveRegistrySourceArg("local", "../registry.json", "")
	if err == nil {
		t.Fatal("expected escaping path error")
	}
}

func TestValidateRegistrySourceLocalAndRemote(t *testing.T) {
	dir := t.TempDir()
	if err := validateRegistrySource(dir); err != nil {
		t.Fatalf("local source: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	if err := validateRegistrySource(server.URL); err != nil {
		t.Fatalf("remote source: %v", err)
	}
}

func TestValidateRegistrySourceRejectsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateRegistrySource(path); err == nil {
		t.Fatal("expected non-directory source error")
	}
}
