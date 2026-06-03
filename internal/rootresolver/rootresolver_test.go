package rootresolver

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"testing/fstest"

	internalgithub "github.com/bryanbarton525/prism/internal/github"
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
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "hello.txt"), []byte("hello"), 0o644); err != nil {
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

func TestResolveGitHubTokenPath(t *testing.T) {
	origNewGitHubFS := newGitHubFS
	origProbeFS := probeFS
	origCloneRepo := cloneRepo
	t.Cleanup(func() {
		newGitHubFS = origNewGitHubFS
		probeFS = origProbeFS
		cloneRepo = origCloneRepo
	})

	ghFS := fstest.MapFS{
		"agents/github-cli.md": &fstest.MapFile{Data: []byte("agent")},
	}
	newGitHubFS = func(owner, repo, ref, token string) fs.FS {
		if owner != "owner" || repo != "repo" || ref != "main" || token != "token" {
			t.Fatalf("unexpected github ctor args: owner=%q repo=%q ref=%q token=%q", owner, repo, ref, token)
		}
		return ghFS
	}
	probeFS = func(ctx context.Context, fsys fs.FS) error {
		if _, err := fs.ReadDir(fsys, "."); err != nil {
			return err
		}
		return nil
	}
	cloneRepo = func(ctx context.Context, url string) (fs.FS, func(), error) {
		t.Fatalf("clone fallback should not be called for a healthy GitHub token path")
		return nil, func() {}, nil
	}

	fsys, cleanup, err := Resolve(context.Background(), "https://github.com/owner/repo/tree/main", "token")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	defer cleanup()

	data, err := fs.ReadFile(fsys, "agents/github-cli.md")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "agent" {
		t.Fatalf("unexpected content: %q", data)
	}
}

func TestResolveGitHubFallbackToClone(t *testing.T) {
	origNewGitHubFS := newGitHubFS
	origProbeFS := probeFS
	origCloneRepo := cloneRepo
	t.Cleanup(func() {
		newGitHubFS = origNewGitHubFS
		probeFS = origProbeFS
		cloneRepo = origCloneRepo
	})

	probeFS = func(ctx context.Context, fsys fs.FS) error {
		return errors.New("probe failed")
	}
	var cloneCalled bool
	cloneRepo = func(ctx context.Context, url string) (fs.FS, func(), error) {
		cloneCalled = true
		return fstest.MapFS{
			"cloned.txt": &fstest.MapFile{Data: []byte("cloned")},
		}, func() {}, nil
	}
	newGitHubFS = func(owner, repo, ref, token string) fs.FS {
		return fstest.MapFS{}
	}

	fsys, cleanup, err := Resolve(context.Background(), "https://github.com/owner/repo", "token")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	defer cleanup()

	if !cloneCalled {
		t.Fatal("expected clone fallback to be called")
	}
	data, err := fs.ReadFile(fsys, "cloned.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "cloned" {
		t.Fatalf("unexpected content: %q", data)
	}
}

func TestCloneFallback_ClonesLocalGitRepo(t *testing.T) {
	src := t.TempDir()
	initGitRepo(t, src)
	if err := os.WriteFile(filepath.Join(src, "README.md"), []byte("hello from source"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, src, "add", "README.md")
	runGit(t, src, "commit", "-m", "initial")

	fsys, cleanup, err := cloneFallback(context.Background(), src)
	if err != nil {
		t.Fatalf("cloneFallback: %v", err)
	}

	data, err := fs.ReadFile(fsys, "README.md")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "hello from source" {
		t.Fatalf("unexpected cloned content: %q", data)
	}

	cleanup()
	if _, err := fs.ReadFile(fsys, "README.md"); err == nil {
		t.Fatal("expected cloned temp dir to be removed after cleanup")
	}
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "codex@example.com")
	runGit(t, dir, "config", "user.name", "Codex")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

// TestResolveFSInterface verifies the returned FS satisfies fs.FS.
func TestResolveFSInterface(t *testing.T) {
	var _ fs.FS = fstest.MapFS{}

	fsys, cleanup, err := Resolve(context.Background(), ".", "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	defer cleanup()
	var _ fs.FS = fsys
}

func TestResolveRecognizesGitHubURL(t *testing.T) {
	if !internalgithub.IsURL("https://github.com/owner/repo") {
		t.Fatal("expected github URL to be recognized")
	}
	if internalgithub.IsURL("/local/path") {
		t.Fatal("expected local path not to be recognized as github URL")
	}
}
