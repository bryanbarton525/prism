package extensions

import (
	"context"
	"time"

	"github.com/bryanbarton525/prism/internal/filelock"
)

const (
	lockMaxWait = 5 * time.Second
)

func acquireFileLock(ctx context.Context, path string) (func(), error) {
	return filelock.Acquire(ctx, path, lockMaxWait)
}
