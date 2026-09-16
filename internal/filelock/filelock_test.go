package filelock

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestActiveAdvisoryLockIsNotReclaimedByOldMtime(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "state", ".lock")
	unlock, err := Acquire(context.Background(), lockPath, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	locked := true
	t.Cleanup(func() {
		if locked {
			unlock()
		}
	})
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if secondUnlock, err := Acquire(ctx, lockPath, time.Second); !errors.Is(err, context.DeadlineExceeded) {
		if secondUnlock != nil {
			secondUnlock()
		}
		t.Fatalf("active aged lock was stolen: err=%v", err)
	}
	unlock()
	locked = false
	secondUnlock, err := Acquire(context.Background(), lockPath, time.Second)
	if err != nil {
		t.Fatalf("released advisory lock remained held: %v", err)
	}
	secondUnlock()
}
