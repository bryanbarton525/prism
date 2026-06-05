package bundles

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bryanbarton525/prism/pkg/registry"
)

func TestInstallVerifiedCopiesFilesAndRecordsState(t *testing.T) {
	source := t.TempDir()
	dest := t.TempDir()
	statePath := filepath.Join(t.TempDir(), "bundles.yaml")
	writeBundleFile(t, filepath.Join(source, "skills", "k8s", "SKILL.md"), "skill")
	manifest, pub, _ := signedRegistry(t, source)
	manifestPath := writeRegistryManifest(t, source, manifest)

	installed, err := InstallVerified(InstallOptions{
		ManifestPath: manifestPath,
		DestRoot:     dest,
		StatePath:    statePath,
		PublicKey:    base64.StdEncoding.EncodeToString(pub),
		PrismVersion: "0.1.0",
	})
	if err != nil {
		t.Fatalf("InstallVerified(): %v", err)
	}
	if installed.Bundles[0].ID != "k8s-core-triage" {
		t.Fatalf("installed manifest = %#v", installed)
	}
	data, err := os.ReadFile(filepath.Join(dest, "skills", "k8s", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "skill" {
		t.Fatalf("installed file = %q", data)
	}
	state, err := Load(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Bundles) != 1 || state.Bundles[0].ID != "k8s-core-triage" {
		t.Fatalf("state = %#v", state)
	}
}

func TestInstallVerifiedFromRemoteManifest(t *testing.T) {
	source := t.TempDir()
	dest := t.TempDir()
	statePath := filepath.Join(t.TempDir(), "bundles.yaml")
	writeBundleFile(t, filepath.Join(source, "skills", "k8s", "SKILL.md"), "remote skill")
	manifest, pub, _ := signedRegistry(t, source)
	writeRegistryManifest(t, source, manifest)
	server := httptest.NewServer(http.FileServer(http.Dir(source)))
	defer server.Close()

	installed, err := InstallVerified(InstallOptions{
		ManifestPath: server.URL + "/registry.json",
		DestRoot:     dest,
		StatePath:    statePath,
		PublicKey:    base64.StdEncoding.EncodeToString(pub),
		PrismVersion: "0.1.0",
	})
	if err != nil {
		t.Fatalf("InstallVerified(): %v", err)
	}
	if installed.RegistryID != "platform-sre" {
		t.Fatalf("installed manifest = %#v", installed)
	}
	data, err := os.ReadFile(filepath.Join(dest, "skills", "k8s", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "remote skill" {
		t.Fatalf("installed remote file = %q", data)
	}
}

func TestInstallVerifiedFailsClosedAndDoesNotRecordState(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(registry.Manifest) registry.Manifest
		version string
	}{
		{
			name: "bad signature",
			mutate: func(m registry.Manifest) registry.Manifest {
				m.Signature = ""
				return m
			},
			version: "0.1.0",
		},
		{
			name: "checksum mismatch",
			mutate: func(m registry.Manifest) registry.Manifest {
				m.Bundles[0].Files[0].SHA256 = strings.Repeat("0", 64)
				return m
			},
			version: "0.1.0",
		},
		{
			name: "path traversal",
			mutate: func(m registry.Manifest) registry.Manifest {
				m.Bundles[0].Files[0].Path = "../outside"
				return m
			},
			version: "0.1.0",
		},
		{
			name:    "incompatible version",
			mutate:  func(m registry.Manifest) registry.Manifest { return m },
			version: "0.0.9",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			source := t.TempDir()
			dest := t.TempDir()
			statePath := filepath.Join(t.TempDir(), "bundles.yaml")
			writeBundleFile(t, filepath.Join(source, "skills", "k8s", "SKILL.md"), "skill")
			manifest, pub, priv := signedRegistry(t, source)
			manifest = tc.mutate(manifest)
			if tc.name != "bad signature" {
				manifest.Signature = signRegistryManifest(t, manifest, priv)
			}
			manifestPath := writeRegistryManifest(t, source, manifest)

			_, err := InstallVerified(InstallOptions{
				ManifestPath: manifestPath,
				DestRoot:     dest,
				StatePath:    statePath,
				PublicKey:    base64.StdEncoding.EncodeToString(pub),
				PrismVersion: tc.version,
			})
			if err == nil {
				t.Fatal("expected install failure")
			}
			state, loadErr := Load(statePath)
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			if len(state.Bundles) != 0 {
				t.Fatalf("state should not be recorded after failure: %#v", state)
			}
		})
	}
}

func signedRegistry(t *testing.T, source string) (registry.Manifest, ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifest := registry.Manifest{
		RegistryID:  "platform-sre",
		Version:     "2026.06.03",
		GeneratedAt: time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC),
		Compat:      registry.Compat{MinPrismVersion: "0.1.0"},
		Bundles: []registry.Bundle{{
			ID:              "k8s-core-triage",
			Version:         "0.1.0",
			Channel:         "stable",
			Owner:           "platform-sre",
			RiskLevel:       "read_only",
			RequiredPlugins: []string{"kubernetes"},
			Files: []registry.BundleFile{{
				Kind:   "skill",
				Path:   "skills/k8s/SKILL.md",
				SHA256: sha256BundleFile(t, filepath.Join(source, "skills", "k8s", "SKILL.md")),
			}},
		}},
	}
	manifest.Signature = signRegistryManifest(t, manifest, priv)
	return manifest, pub, priv
}

func signRegistryManifest(t *testing.T, manifest registry.Manifest, privateKey ed25519.PrivateKey) string {
	t.Helper()
	payload, err := registry.SignedPayload(manifest)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, payload))
}

func writeRegistryManifest(t *testing.T, dir string, manifest registry.Manifest) string {
	t.Helper()
	path := filepath.Join(dir, "registry.json")
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeBundleFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sha256BundleFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
