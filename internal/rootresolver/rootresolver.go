// Package rootresolver resolves the --root flag value to an fs.FS and a
// cleanup function.
//
// Resolution order:
//  1. Local filesystem path — returns os.DirFS(root), no-op cleanup.
//  2. GitHub URL (github.com) + token provided — returns a github.FS that
//     reads files directly via the GitHub Contents API. No local clone is
//     needed; cleanup is a no-op.
//  3. GitHub URL + no token (or API call fails) — falls back to
//     git clone --depth 1 into a temp directory and returns os.DirFS of
//     that directory. Cleanup removes the temp dir.
//
// The token parameter should be populated by the caller (CLI layer). Passing an
// empty token triggers the clone fallback for GitHub URLs.
package rootresolver

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bryanbarton525/prism/internal/github"
)

var (
	newGitHubFS = func(owner, repo, ref, token string) fs.FS { return github.New(owner, repo, ref, token) }
	probeFS     = probe
	cloneRepo   = cloneFallback
)

// Resolve returns an fs.FS for the given root and a cleanup function.
//
// The caller must call cleanup() when the FS is no longer needed. For local
// paths and the GitHub API path cleanup is a no-op. For the clone fallback
// cleanup removes the temporary directory.
func Resolve(ctx context.Context, root, token string) (fsys fs.FS, cleanup func(), err error) {
	if !github.IsURL(root) {
		absolute, err := filepath.Abs(root)
		if err != nil {
			return nil, func() {}, fmt.Errorf("rootresolver: canonicalizing local root: %w", err)
		}
		if canonical, evalErr := filepath.EvalSymlinks(absolute); evalErr == nil {
			absolute = canonical
		}
		info, err := os.Stat(absolute)
		if err != nil {
			return nil, func() {}, fmt.Errorf("rootresolver: reading local root: %w", err)
		}
		if !info.IsDir() {
			return nil, func() {}, fmt.Errorf("rootresolver: local root is not a directory: %s", absolute)
		}
		return newConfinedDirFS(absolute), func() {}, nil
	}

	// --- GitHub URL ---
	owner, repo, ref, err := github.ParseURL(root)
	if err != nil {
		return nil, func() {}, fmt.Errorf("rootresolver: %w", err)
	}

	if token != "" {
		// Primary: GitHub Contents API.
		// We do a lightweight probe (list repo root) to verify the token and
		// URL are valid before handing the FS to the caller.
		ghFS := newGitHubFS(owner, repo, ref, token)
		probeErr := probeFS(ctx, ghFS)
		if probeErr == nil {
			return ghFS, func() {}, nil
		}
		// API probe failed (bad token, private repo, rate-limited) — fall through to clone.
		fmt.Fprintf(os.Stderr, "[prism] GitHub API probe failed (%v); falling back to git clone\n", probeErr)
	}

	// Fallback: git clone --depth 1.
	return cloneRepo(ctx, root)
}

// confinedDirFS deliberately rejects symlinks at every component. Repository
// plugins consume arbitrary walked paths via fs.ReadFile; os.DirFS otherwise
// follows a link inside the workspace and can expose files outside the root.
type confinedDirFS struct {
	root string
}

func newConfinedDirFS(root string) fs.FS {
	return confinedDirFS{root: root}
}

func (f confinedDirFS) Open(name string) (fs.File, error) {
	if name != "." && !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}
	current := f.root
	if name != "." {
		for _, component := range strings.Split(name, "/") {
			current = filepath.Join(current, component)
			info, err := os.Lstat(current)
			if err != nil {
				return nil, err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("rootresolver: path %q contains a symlink", name)
			}
		}
	}
	return os.Open(current)
}

// probe verifies that the FS is accessible by attempting to read the root directory.
func probe(ctx context.Context, fsys fs.FS) error {
	_, err := fs.ReadDir(fsys, ".")
	return err
}

// cloneFallback clones the repository into a temp directory and returns an
// confined fs.FS of the clone. The cleanup function removes the temp dir.
func cloneFallback(ctx context.Context, url string) (fs.FS, func(), error) {
	tmpDir, err := os.MkdirTemp("", "prism-root-*")
	if err != nil {
		return nil, func() {}, fmt.Errorf("rootresolver: creating temp dir: %w", err)
	}
	rmCleanup := func() { os.RemoveAll(tmpDir) }

	//nolint:gosec // url comes from the operator-controlled --root flag
	cloneURL := url
	ref := ""
	if owner, repo, parsedRef, parseErr := github.ParseURL(url); parseErr == nil {
		cloneURL = "https://github.com/" + owner + "/" + repo + ".git"
		if parsedRef != "" && parsedRef != "HEAD" {
			ref = parsedRef
		}
	}
	cmd := exec.CommandContext(ctx, "git", cloneArgsWithRef(cloneURL, ref, tmpDir)...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		rmCleanup()
		return nil, func() {}, fmt.Errorf("rootresolver: git clone %s: %w", url, err)
	}

	return newConfinedDirFS(tmpDir), rmCleanup, nil
}

func cloneArgs(url, dest string) []string {
	return []string{"clone", "--depth", "1", "--", url, dest}
}

func cloneArgsWithRef(url, ref, dest string) []string {
	if ref == "" {
		return cloneArgs(url, dest)
	}
	return []string{"clone", "--depth", "1", "--branch", ref, "--single-branch", "--", url, dest}
}
