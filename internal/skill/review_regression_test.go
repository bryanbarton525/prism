package skill

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"unicode/utf8"
)

func TestReviewResourceBoundsMediaAndSymlinkSafety(t *testing.T) {
	fsys := fstest.MapFS{
		"demo/ref.txt":  {Data: []byte("a€b")},
		"demo/data.bin": {Data: []byte("plain ASCII")},
	}
	res, err := ReadResource(fsys, "demo", "ref.txt", ReadResourceOptions{Offset: 2, Limit: 3, MaxReadBytes: math.MaxInt64})
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.ValidString(res.Content) || res.Content != "€" || res.Offset != 1 {
		t.Fatalf("rune-aligned result = %#v", res)
	}
	if _, err := ReadResource(fsys, "demo", "data.bin", ReadResourceOptions{}); !errors.Is(err, ErrUnsupportedTextResource) {
		t.Fatalf("binary media error = %v", err)
	}
	if _, err := ReadResource(fsys, ".", "ref.txt", ReadResourceOptions{}); err == nil {
		t.Fatal("unsafe skill identity was accepted")
	}

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "demo", "link.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadResource(os.DirFS(root), "demo", "link.txt", ReadResourceOptions{}); err == nil {
		t.Fatal("resource symlink was followed")
	}
}
