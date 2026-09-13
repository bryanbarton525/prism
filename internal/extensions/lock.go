package extensions

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	lockRetryInterval = 25 * time.Millisecond
	lockMaxWait       = 5 * time.Second
	staleLockAge      = 30 * time.Second
)

func acquireFileLock(ctx context.Context, path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	started := time.Now()
	for {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = file.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("create lock %s: %w", path, err)
		}
		if stale, staleErr := staleLockDetected(path); staleErr == nil && stale {
			_ = os.Remove(path)
			continue
		}
		if time.Since(started) >= lockMaxWait {
			return nil, fmt.Errorf("waiting for lock %s exceeded %s; remove stale lock if no Prism process is running", path, lockMaxWait)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(lockRetryInterval):
		}
	}
}

func staleLockDetected(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return time.Since(info.ModTime()) > staleLockAge, nil
}
