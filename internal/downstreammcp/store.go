package downstreammcp

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

const (
	lockRetryInterval = 25 * time.Millisecond
	lockMaxWait       = 5 * time.Second
	staleLockAge      = 30 * time.Second
)

var ErrExternalEdit = errors.New("downstream MCP state changed on disk")

type Snapshot struct {
	State    State
	Revision string
}

type Store interface {
	Read(ctx context.Context) (Snapshot, error)
	Write(ctx context.Context, snapshot Snapshot) error
}

type FileStore struct {
	path string
}

func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

func (s *FileStore) Read(_ context.Context) (Snapshot, error) {
	state, revision, err := readStateWithRevision(s.path)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{State: state, Revision: revision}, nil
}

func (s *FileStore) Write(ctx context.Context, snapshot Snapshot) error {
	lockPath := s.path + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return err
	}
	unlock, err := acquireLock(ctx, lockPath)
	if err != nil {
		return err
	}
	defer unlock()

	current, currentRevision, err := readStateWithRevision(s.path)
	if err != nil {
		return err
	}
	_ = current
	if snapshot.Revision != currentRevision {
		return ErrExternalEdit
	}
	return writeStateAtomically(s.path, snapshot.State)
}

func readStateWithRevision(path string) (State, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{}, "", nil
		}
		return State{}, "", err
	}
	var state State
	if err := yaml.Unmarshal(data, &state); err != nil {
		return State{}, "", err
	}
	sum := sha256.Sum256(data)
	return state, hex.EncodeToString(sum[:]), nil
}

func writeStateAtomically(path string, state State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(state)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".mcp-servers-*.tmp")
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

func acquireLock(ctx context.Context, path string) (func(), error) {
	started := time.Now()
	for {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = file.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("creating lock file: %w", err)
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
