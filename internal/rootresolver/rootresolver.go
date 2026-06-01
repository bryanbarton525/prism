// Package rootresolver resolves the --root flag value to a local filesystem
// path. When root is a remote git URL it is cloned with `git clone --depth 1`
// into a temporary directory; otherwise the value is returned unchanged.
package rootresolver

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// isRemoteURL reports whether root looks like a remote git URL.
func isRemoteURL(root string) bool {
	return strings.HasPrefix(root, "https://") ||
		strings.HasPrefix(root, "http://") ||
		strings.HasPrefix(root, "git@") ||
		strings.HasPrefix(root, "ssh://")
}

// Resolve returns a local path ready to use as RootDir.
//
// If root is already a local filesystem path it is returned unchanged and
// cleanup is a no-op.
//
// If root looks like a remote git URL (https://, http://, git@, ssh://) it is
// cloned with `git clone --depth 1` into a temporary directory. The caller
// must call cleanup() when the local path is no longer needed to remove the
// temporary directory.
//
// Requires git on PATH. Private repos must have credentials already configured
// in the environment (SSH key or HTTPS credential helper).
func Resolve(ctx context.Context, root string) (localPath string, cleanup func(), err error) {
	if !isRemoteURL(root) {
		return root, func() {}, nil
	}

	tmpDir, err := os.MkdirTemp("", "prism-root-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("rootresolver: creating temp dir: %w", err)
	}

	rmCleanup := func() { os.RemoveAll(tmpDir) }

	//nolint:gosec // git and the URL come from the operator-controlled --root flag
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", root, tmpDir)
	cmd.Stdout = os.Stderr // clone progress goes to stderr so stdout stays clean
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		rmCleanup()
		return "", func() {}, fmt.Errorf("rootresolver: cloning %s: %w", root, err)
	}

	return tmpDir, rmCleanup, nil
}
