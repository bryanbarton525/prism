package rootresolver

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestReviewWorkspaceFSRejectsSymlinkEscapes(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link.txt")); err != nil {
		t.Fatal(err)
	}
	fsys, cleanup, err := Resolve(context.Background(), root, "")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err := fs.ReadFile(fsys, "link.txt"); err == nil {
		t.Fatal("workspace symlink was followed")
	}
}
