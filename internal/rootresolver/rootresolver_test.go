package rootresolver

import (
	"context"
	"io/fs"
	"os"
	"testing"
	"testing/fstest"
)

func TestResolveLocal(t *testing.T) {
	cases := []struct {
		name string
		root string
	}{
		{"absolute path", "/some/local/path"},
		{"dot", "."},
		{"relative", "../prism"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fsys, cleanup, err := Resolve(context.Background(), tc.root, "")
			if err != nil {
				t.Fatalf("Resolve(%q): unexpected error: %v", tc.root, err)
			}
			defer cleanup()
			if fsys == nil {
				t.Fatal("expected non-nil fs.FS")
			}
		})
	}
}

func TestResolveLocal_ReturnsOSDirFS(t *testing.T) {
	// Create a real temp dir with a file, verify the returned FS can read it.
	tmp := t.TempDir()
	if err := os.WriteFile(tmp+"/hello.txt", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	fsys, cleanup, err := Resolve(context.Background(), tmp, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	defer cleanup()

	data, err := fs.ReadFile(fsys, "hello.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("content: want %q, got %q", "hello", data)
	}
}

// TestResolveFSInterface verifies the returned FS satisfies fs.FS.
func TestResolveFSInterface(t *testing.T) {
	// Use fstest.MapFS as a stand-in to test the interface contract.
	var _ fs.FS = fstest.MapFS{}
	// Resolve of a local path must return something that implements fs.FS.
	fsys, cleanup, err := Resolve(context.Background(), ".", "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	defer cleanup()
	var _ fs.FS = fsys // compile-time interface check
}
