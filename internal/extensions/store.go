package extensions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Store struct {
	stateDir      string
	writeManifest func(path string, data []byte) error
	now           func() time.Time
}

func NewStore(stateDir string) *Store {
	return &Store{
		stateDir:      stateDir,
		writeManifest: writeFileAtomically,
		now:           time.Now,
	}
}

func (s *Store) ManifestPath() string {
	return filepath.Join(s.stateDir, "extensions.yaml")
}

func (s *Store) ObjectsDir() string {
	return filepath.Join(s.stateDir, "extensions", "objects")
}

func (s *Store) LockPath() string {
	return filepath.Join(s.stateDir, "extensions", ".lock")
}

func (s *Store) TxnDir() string {
	return filepath.Join(s.stateDir, "extensions", ".txn")
}

func (s *Store) journalPath() string {
	return filepath.Join(s.TxnDir(), "journal.yaml")
}

func (s *Store) LoadManifest() (Manifest, error) {
	data, err := os.ReadFile(s.ManifestPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return EmptyManifest(), nil
		}
		return Manifest{}, err
	}
	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}
	if manifest.Version == 0 {
		manifest.Version = ManifestVersion
	}
	if manifest.Entries == nil {
		manifest.Entries = []ManifestEntry{}
	}
	return manifest, nil
}

func (s *Store) SaveManifest(manifest Manifest) error {
	if manifest.Version == 0 {
		manifest.Version = ManifestVersion
	}
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return err
	}
	return s.writeManifest(s.ManifestPath(), data)
}

// EnsureInitialized creates an empty manifest through the normal transaction
// path. It intentionally creates no objects or extension bindings.
func (s *Store) EnsureInitialized(ctx context.Context) error {
	tx, err := s.BeginTransaction(ctx)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) PutObject(content []byte) (digest string, objectPath string, err error) {
	sum := sha256.Sum256(content)
	digest = hex.EncodeToString(sum[:])
	objectPath = filepath.Join(s.ObjectsDir(), digest)
	if err := os.MkdirAll(s.ObjectsDir(), 0o755); err != nil {
		return "", "", err
	}
	existing, readErr := os.ReadFile(objectPath)
	if readErr == nil {
		other := sha256.Sum256(existing)
		if digest != hex.EncodeToString(other[:]) {
			return "", "", fmt.Errorf("object digest collision at %s", objectPath)
		}
		return digest, objectPath, nil
	}
	if !errors.Is(readErr, os.ErrNotExist) {
		return "", "", readErr
	}
	if err := writeFileAtomically(objectPath, content); err != nil {
		return "", "", err
	}
	return digest, objectPath, nil
}

type journal struct {
	PreviousManifest Manifest `yaml:"previous_manifest"`
}

func (s *Store) writeJournal(previous Manifest) error {
	if err := os.MkdirAll(s.TxnDir(), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(journal{PreviousManifest: previous})
	if err != nil {
		return err
	}
	return writeFileAtomically(s.journalPath(), data)
}

func (s *Store) clearJournal() error {
	err := os.Remove(s.journalPath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *Store) recoverInterrupted() (bool, error) {
	data, err := os.ReadFile(s.journalPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	var j journal
	if err := yaml.Unmarshal(data, &j); err != nil {
		return false, fmt.Errorf("read extension recovery journal: %w", err)
	}
	if err := s.SaveManifest(j.PreviousManifest); err != nil {
		return false, err
	}
	if err := s.clearJournal(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) Recover(ctx context.Context) (bool, error) {
	unlock, err := acquireFileLock(ctx, s.LockPath())
	if err != nil {
		return false, err
	}
	defer unlock()
	return s.recoverInterrupted()
}

func (s *Store) RecoverAndLoadManifest(ctx context.Context) (Manifest, bool, error) {
	unlock, err := acquireFileLock(ctx, s.LockPath())
	if err != nil {
		return Manifest{}, false, err
	}
	defer unlock()
	recovered, err := s.recoverInterrupted()
	if err != nil {
		return Manifest{}, recovered, err
	}
	manifest, err := s.LoadManifest()
	if err != nil {
		return Manifest{}, recovered, err
	}
	return manifest, recovered, nil
}

func writeFileAtomically(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
