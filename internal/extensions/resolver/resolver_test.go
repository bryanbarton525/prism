package resolver

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLocalAndDigest(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skills", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skills", "demo", "SKILL.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Resolve(context.Background(), root, "", Bounds{MaxFiles: 10, MaxBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	defer res.Cleanup()
	if res.Digest == "" || res.FileCount != 1 {
		t.Fatalf("result = %#v", res)
	}
}

func TestResolveBoundsExceeded(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("aaaaaaaaaa"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Resolve(context.Background(), root, "", Bounds{MaxFiles: 1, MaxBytes: 4})
	if err == nil {
		t.Fatal("expected bounds error")
	}
}

func TestDescribeGitHubSource(t *testing.T) {
	canonical, ref, resolved := describeSource("https://github.com/owner/repo/tree/main")
	if canonical != "github://owner/repo" || ref != "main" || resolved != "main" {
		t.Fatalf("got canonical=%q ref=%q resolved=%q", canonical, ref, resolved)
	}
}
