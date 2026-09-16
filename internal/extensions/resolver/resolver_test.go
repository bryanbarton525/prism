package resolver

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
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
	if canonical != "github://owner/repo" || ref != "main" || resolved != "" {
		t.Fatalf("got canonical=%q ref=%q resolved=%q", canonical, ref, resolved)
	}
}

func TestResolveRejectsSymlinkAndSpecialFileInputs(t *testing.T) {
	for _, mode := range []os.FileMode{os.ModeSymlink, os.ModeNamedPipe} {
		source := fstest.MapFS{"bad": &fstest.MapFile{Data: []byte("outside content"), Mode: mode}}
		if _, _, _, err := digestFS(source, Bounds{MaxFiles: 1, MaxBytes: 1}); err == nil {
			t.Fatalf("mode %v escaped resolver source validation", mode)
		}
	}
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("outside content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := Resolve(context.Background(), root, "", Bounds{MaxFiles: 1, MaxBytes: 1}); err == nil {
		t.Fatal("local symlink source escaped resolver bounds")
	}
}
