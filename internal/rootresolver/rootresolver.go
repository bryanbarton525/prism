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
// The token parameter should be populated from the GITHUB_TOKEN environment
// variable by the caller (CLI layer). Passing an empty token triggers the
// clone fallback for GitHub URLs.
package rootresolver

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"

	"github.com/bryanbarton525/prism/internal/github"
)

// Resolve returns an fs.FS for the given root and a cleanup function.
//
// The caller must call cleanup() when the FS is no longer needed. For local
// paths and the GitHub API path cleanup is a no-op. For the clone fallback
// cleanup removes the temporary directory.
func Resolve(ctx context.Context, root, token string) (fsys fs.FS, cleanup func(), err error) {
	if !github.IsURL(root) {
		// Local filesystem path — fast path, no network.
		return os.DirFS(root), func() {}, nil
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
		ghFS := github.New(owner, repo, ref, token)
		probeErr := probe(ctx, ghFS)
		if probeErr == nil {
			return ghFS, func() {}, nil
		}
		// API probe failed (bad token, private repo, rate-limited) — fall through to clone.
		fmt.Fprintf(os.Stderr, "[prism] GitHub API probe failed (%v); falling back to git clone\n", probeErr)
	}

	// Fallback: git clone --depth 1.
	return cloneFallback(ctx, root)
}

// probe verifies that the FS is accessible by attempting to read the root directory.
func probe(ctx context.Context, fsys fs.FS) error {
	_, err := fs.ReadDir(fsys, ".")
	return err
}

// cloneFallback clones the repository into a temp directory and returns an
// os.DirFS of the clone. The cleanup function removes the temp dir.
func cloneFallback(ctx context.Context, url string) (fs.FS, func(), error) {
	tmpDir, err := os.MkdirTemp("", "prism-root-*")
	if err != nil {
		return nil, func() {}, fmt.Errorf("rootresolver: creating temp dir: %w", err)
	}
	rmCleanup := func() { os.RemoveAll(tmpDir) }

	//nolint:gosec // url comes from the operator-controlled --root flag
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", url, tmpDir)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		rmCleanup()
		return nil, func() {}, fmt.Errorf("rootresolver: git clone %s: %w", url, err)
	}

	return os.DirFS(tmpDir), rmCleanup, nil
}
