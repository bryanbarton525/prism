package filelock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const retryInterval = 25 * time.Millisecond

// Acquire obtains an operating-system advisory lock for path. The file may
// remain after use; ownership is tied to the open handle and the kernel
// releases it if Prism exits unexpectedly.
func Acquire(ctx context.Context, path string, maxWait time.Duration) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock %s: %w", path, err)
	}
	started := time.Now()
	for {
		unlockOS, lockErr := tryExclusive(file)
		if lockErr == nil {
			return func() {
				_ = unlockOS()
				_ = file.Close()
			}, nil
		}
		if !errors.Is(lockErr, errWouldBlock) {
			_ = file.Close()
			return nil, fmt.Errorf("lock %s: %w", path, lockErr)
		}
		if maxWait > 0 && time.Since(started) >= maxWait {
			_ = file.Close()
			return nil, fmt.Errorf("waiting for lock %s exceeded %s; another Prism process may still be active", path, maxWait)
		}
		select {
		case <-ctx.Done():
			_ = file.Close()
			return nil, ctx.Err()
		case <-time.After(retryInterval):
		}
	}
}
