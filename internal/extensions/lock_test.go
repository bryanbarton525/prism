package extensions

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireFileLockRecoversStaleLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "extensions", ".lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * staleLockAge)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatal(err)
	}
	unlock, err := acquireFileLock(context.Background(), lockPath)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
}
