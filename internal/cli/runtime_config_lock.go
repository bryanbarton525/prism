package cli

import (
	"context"
	"path/filepath"
	"time"

	"github.com/bryanbarton525/prism/internal/filelock"
)

// acquireRuntimeConfigLock is the outer lock for mutations spanning extension
// access policy, downstream MCP registration, and Graphify bindings. Inner
// locks are always acquired after this one.
func acquireRuntimeConfigLock(ctx context.Context) (func(), error) {
	return filelock.Acquire(ctx, filepath.Join(gf.stateDir, "runtime-config.lock"), 5*time.Second)
}
