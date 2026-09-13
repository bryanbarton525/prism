package extensions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPutObjectStoresByDigest(t *testing.T) {
	store := NewStore(t.TempDir())
	digest, path, err := store.PutObject([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if digest == "" {
		t.Fatal("expected digest")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("data = %q", data)
	}
}

func TestTransactionCommitAndRollbackOnFailure(t *testing.T) {
	store := NewStore(t.TempDir())
	store.now = func() time.Time { return time.Unix(1700000000, 0).UTC() }
	if err := store.SaveManifest(Manifest{
		Version: ManifestVersion,
		Entries: []ManifestEntry{{Identity: "old", Kind: "skill", Source: "bundled", Digest: "a", ObjectPath: "/tmp/a"}},
	}); err != nil {
		t.Fatal(err)
	}

	origWrite := store.writeManifest
	failOnce := true
	store.writeManifest = func(path string, data []byte) error {
		if failOnce {
			failOnce = false
			return errors.New("injected write failure")
		}
		return origWrite(path, data)
	}

	tx, err := store.BeginTransaction(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	tx.UpsertEntry(ManifestEntry{Identity: "new", Kind: "skill", Source: "git", Digest: "b", ObjectPath: "/tmp/b"})
	if err := tx.Commit(); err == nil {
		t.Fatal("expected commit failure")
	}

	manifest, err := store.LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Entries) != 1 || manifest.Entries[0].Identity != "old" {
		t.Fatalf("manifest after rollback = %#v", manifest.Entries)
	}
}

func TestRecoverInterruptedRestoresPreviousManifest(t *testing.T) {
	store := NewStore(t.TempDir())
	old := Manifest{Version: ManifestVersion, Entries: []ManifestEntry{{Identity: "old", Kind: "skill", Source: "bundled", Digest: "a", ObjectPath: "/tmp/a"}}}
	if err := store.SaveManifest(old); err != nil {
		t.Fatal(err)
	}
	if err := store.writeJournal(old); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveManifest(Manifest{Version: ManifestVersion, Entries: []ManifestEntry{{Identity: "new", Kind: "skill", Source: "git", Digest: "b", ObjectPath: "/tmp/b"}}}); err != nil {
		t.Fatal(err)
	}

	recovered, err := store.Recover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !recovered {
		t.Fatal("expected recovery to run")
	}
	manifest, err := store.LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Entries) != 1 || manifest.Entries[0].Identity != "old" {
		t.Fatalf("recovered manifest = %#v", manifest.Entries)
	}
	if _, err := os.Stat(filepath.Join(store.TxnDir(), "journal.yaml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("journal still exists: %v", err)
	}
}
