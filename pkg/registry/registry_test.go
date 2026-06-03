package registry

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVerifySignatureFilesAndInstall(t *testing.T) {
	source := t.TempDir()
	dest := t.TempDir()
	writeRegistryFile(t, filepath.Join(source, "skills", "kubectl-triage", "SKILL.md"), "skill body")
	writeRegistryFile(t, filepath.Join(source, "agents", "kubectl.md"), "agent body")

	manifest := Manifest{
		RegistryID:  "platform-sre",
		Version:     "2026.06.01",
		GeneratedAt: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
		Compat:      Compat{MinPrismVersion: "0.1.0"},
		Bundles: []Bundle{{
			ID:      "kubernetes",
			Version: "1.0.0",
			Files: []BundleFile{
				{Kind: "skill", Path: "skills/kubectl-triage/SKILL.md", SHA256: sha256File(t, filepath.Join(source, "skills", "kubectl-triage", "SKILL.md"))},
				{Kind: "agent", Path: "agents/kubectl.md", SHA256: sha256File(t, filepath.Join(source, "agents", "kubectl.md"))},
			},
		}},
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Signature = signManifest(t, manifest, priv)

	if err := VerifySignature(manifest, pub); err != nil {
		t.Fatalf("VerifySignature(): %v", err)
	}
	if err := VerifyCompat("0.1.0", manifest.Compat); err != nil {
		t.Fatalf("VerifyCompat(): %v", err)
	}
	if err := VerifyFiles(source, manifest); err != nil {
		t.Fatalf("VerifyFiles(): %v", err)
	}
	if err := VerifyManifest(source, manifest, pub, "0.1.0"); err != nil {
		t.Fatalf("VerifyManifest(): %v", err)
	}
	if err := Install(source, dest, manifest, pub, "0.1.0"); err != nil {
		t.Fatalf("Install(): %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "skills", "kubectl-triage", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "skill body" {
		t.Fatalf("installed skill = %q", data)
	}
}

func TestRegistryRejectsBadSignatureChecksumAndTraversal(t *testing.T) {
	source := t.TempDir()
	writeRegistryFile(t, filepath.Join(source, "skills", "x", "SKILL.md"), "body")
	manifest := Manifest{
		RegistryID:  "r",
		Version:     "1",
		GeneratedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Bundles: []Bundle{{ID: "b", Version: "1", Files: []BundleFile{{
			Kind:   "skill",
			Path:   "skills/x/SKILL.md",
			SHA256: sha256File(t, filepath.Join(source, "skills", "x", "SKILL.md")),
		}}}},
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Signature = signManifest(t, manifest, priv)

	tampered := manifest
	tampered.Version = "2"
	if err := VerifySignature(tampered, pub); err == nil {
		t.Fatal("expected bad signature after tampering")
	}

	badChecksum := cloneManifest(manifest)
	badChecksum.Bundles[0].Files[0].SHA256 = strings.Repeat("0", 64)
	if err := VerifyFiles(source, badChecksum); err == nil {
		t.Fatal("expected checksum mismatch")
	}

	traversal := cloneManifest(manifest)
	traversal.Bundles[0].Files[0].Path = "../outside"
	if err := VerifyFiles(source, traversal); err == nil {
		t.Fatal("expected traversal rejection")
	}
}

func TestVerifyCompatRejectsIncompatibleVersions(t *testing.T) {
	compat := Compat{
		MinPrismVersion: "0.2.0",
		MaxPrismVersion: "1.0.0",
	}
	if err := VerifyCompat("0.1.9", compat); err == nil {
		t.Fatal("expected min version rejection")
	}
	if err := VerifyCompat("1.0.1", compat); err == nil {
		t.Fatal("expected max version rejection")
	}
	if err := VerifyCompat("v0.2.0", compat); err != nil {
		t.Fatalf("expected v-prefix version to pass: %v", err)
	}
}

func TestInstallRejectsBadSignatureAndCompat(t *testing.T) {
	source := t.TempDir()
	dest := t.TempDir()
	writeRegistryFile(t, filepath.Join(source, "skills", "x", "SKILL.md"), "body")

	manifest := Manifest{
		RegistryID:  "r",
		Version:     "1",
		GeneratedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Compat:      Compat{MinPrismVersion: "0.2.0"},
		Bundles: []Bundle{{ID: "b", Version: "1", Files: []BundleFile{{
			Kind:   "skill",
			Path:   "skills/x/SKILL.md",
			SHA256: sha256File(t, filepath.Join(source, "skills", "x", "SKILL.md")),
		}}}},
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Signature = signManifest(t, manifest, priv)

	tampered := cloneManifest(manifest)
	tampered.Signature = ""
	if err := Install(source, dest, tampered, pub, "0.2.0"); err == nil {
		t.Fatal("expected install to reject bad signature")
	}
	if err := Install(source, dest, manifest, pub, "0.1.0"); err == nil {
		t.Fatal("expected install to reject incompatible Prism version")
	}
}

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.json")
	manifest := Manifest{RegistryID: "r", Version: "1", GeneratedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest(): %v", err)
	}
	if loaded.RegistryID != "r" || loaded.Version != "1" {
		t.Fatalf("unexpected manifest: %#v", loaded)
	}
}

func writeRegistryFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sha256File(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func signManifest(t *testing.T, manifest Manifest, privateKey ed25519.PrivateKey) string {
	t.Helper()
	payload, err := SignedPayload(manifest)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, payload))
}

func cloneManifest(manifest Manifest) Manifest {
	cloned := manifest
	cloned.Bundles = append([]Bundle{}, manifest.Bundles...)
	for i := range cloned.Bundles {
		cloned.Bundles[i].Files = append([]BundleFile{}, manifest.Bundles[i].Files...)
	}
	return cloned
}
