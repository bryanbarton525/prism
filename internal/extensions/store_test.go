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

func TestTransactionRollbackRestoresReplacedEntry(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.SaveManifest(Manifest{
		Version: ManifestVersion,
		Entries: []ManifestEntry{{Identity: "reviewer", Kind: "skill", Digest: "old", ObjectPath: "/tmp/old"}},
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
	tx.UpsertEntry(ManifestEntry{Identity: "reviewer", Kind: "skill", Digest: "new", ObjectPath: "/tmp/new"})
	if err := tx.Commit(); err == nil {
		t.Fatal("expected commit error")
	}
	manifest, err := store.LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Entries[0].Digest != "old" {
		t.Fatalf("digest = %q, want old", manifest.Entries[0].Digest)
	}
}

func TestTransactionAllowsCrossKindSameIdentity(t *testing.T) {
	store := NewStore(t.TempDir())
	tx, err := store.BeginTransaction(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	tx.UpsertEntry(ManifestEntry{Identity: "reviewer", Kind: "agent", Digest: "a", ObjectPath: "/tmp/a"})
	tx.UpsertEntry(ManifestEntry{Identity: "reviewer", Kind: "skill", Digest: "b", ObjectPath: "/tmp/b"})
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	manifest, err := store.LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Entries) != 2 {
		t.Fatalf("entries = %#v", manifest.Entries)
	}
}

func TestRollbackKeepsJournalOnRestoreFailure(t *testing.T) {
	store := NewStore(t.TempDir())
	tx, err := store.BeginTransaction(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	store.writeManifest = func(path string, data []byte) error { return errors.New("restore failed") }
	if err := tx.Rollback(); err == nil {
		t.Fatal("expected rollback error")
	}
	if _, err := os.Stat(filepath.Join(store.TxnDir(), "journal.yaml")); err != nil {
		t.Fatalf("expected journal to remain after rollback failure: %v", err)
	}
}

func TestBeginTransactionRecoversExistingJournalBeforeWritingNewOne(t *testing.T) {
	store := NewStore(t.TempDir())
	old := Manifest{Version: ManifestVersion, Entries: []ManifestEntry{{Identity: "old", Kind: "skill", Digest: "old", ObjectPath: "/tmp/old"}}}
	candidate := Manifest{Version: ManifestVersion, Entries: []ManifestEntry{{Identity: "candidate", Kind: "skill", Digest: "new", ObjectPath: "/tmp/new"}}}
	if err := store.SaveManifest(old); err != nil {
		t.Fatal(err)
	}
	if err := store.writeJournal(old); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveManifest(candidate); err != nil {
		t.Fatal(err)
	}
	tx, err := store.BeginTransaction(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	manifest, err := store.LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Entries) != 1 || manifest.Entries[0].Identity != "old" {
		t.Fatalf("manifest = %#v", manifest.Entries)
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
