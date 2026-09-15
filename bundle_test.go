package prism

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/bryanbarton525/prism/internal/graphify"
)

func TestEmbeddedBundle(t *testing.T) {
	for _, path := range []string{
		"agents/github-cli.md",
		"agents/repo-investigator.md",
		"constitutions/github-cli.md",
		"constitutions/repo-investigator.md",
		"skills/gh-pr-triage/SKILL.md",
		"skills/gh-pr-triage/references/REFERENCE.md",
		"skills/gh-pr-triage/scripts/collect.sh",
		"skills/gh-pr-triage/evals/smoke.yaml",
		"skills/graphify-query/SKILL.md",
		"skills/graphify-query/references/REFERENCE.md",
		"skills/graphify-query/references/GRAPHIFY-RELEASE.json",
		"skills/graphify-query/references/managed-environment/pyproject.toml",
		"skills/graphify-query/references/managed-environment/uv.lock",
		"skills/graphify-query/scripts/collect.sh",
		"skills/graphify-query/evals/smoke.yaml",
	} {
		if _, err := fs.Stat(BundleFS(), path); err != nil {
			t.Fatalf("embedded asset %s: %v", path, err)
		}
	}
	if got := BundleDigest(); len(got) != 64 {
		t.Fatalf("digest length = %d, want 64: %q", len(got), got)
	}
}

func TestGraphifyReleaseMetadataIsEmbeddedAndPinned(t *testing.T) {
	path := "skills/graphify-query/references/GRAPHIFY-RELEASE.json"
	data, err := fs.ReadFile(BundleFS(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := graphify.ValidateReleaseMetadata(data); err != nil {
		t.Fatal(err)
	}
	manifest := Manifest()
	found := false
	for _, item := range manifest.Files {
		if item.Path == path && len(item.SHA256) == 64 {
			found = true
		}
	}
	if !found {
		t.Fatalf("embedded Graphify metadata %q is missing from manifest: %#v", path, manifest.Files)
	}
}

func TestGraphifyManagedEnvironmentLockIsEmbeddedAndPinned(t *testing.T) {
	data, err := fs.ReadFile(BundleFS(), "skills/graphify-query/references/managed-environment/uv.lock")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != "15bb19dab6b284ccd0b0941c3096bf405873882f48bf9a5d20bb2ee64fd2997b" {
		t.Fatalf("managed lock digest = %s", got)
	}
}

func TestDigestChangesWithContent(t *testing.T) {
	one := fstest.MapFS{"skills/demo/SKILL.md": &fstest.MapFile{Data: []byte("one")}}
	two := fstest.MapFS{"skills/demo/SKILL.md": &fstest.MapFile{Data: []byte("two")}}
	if DigestFS(one) == DigestFS(two) {
		t.Fatal("bundle digest did not change with embedded content")
	}
}

func TestManifestContainsEmbeddedFiles(t *testing.T) {
	manifest := Manifest()
	if manifest.Digest != BundleDigest() || len(manifest.Files) == 0 {
		t.Fatalf("manifest = %#v", manifest)
	}
	for _, item := range manifest.Files {
		if len(item.SHA256) != 64 {
			t.Fatalf("invalid file digest for %s", item.Path)
		}
	}
}
