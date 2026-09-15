package extensions

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireFileLockRecoversStaleLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "extensions", ".lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	unlock, err := acquireFileLock(context.Background(), lockPath)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
}
