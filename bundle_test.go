package prism

import (
	"io/fs"
	"testing"
	"testing/fstest"
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
