package downstreammcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/bryanbarton525/prism/internal/fileatomic"
	"github.com/bryanbarton525/prism/internal/filelock"
	"gopkg.in/yaml.v3"
)

const (
	lockMaxWait = 5 * time.Second
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
	return fileatomic.Replace(tmpPath, path)
}

func acquireLock(ctx context.Context, path string) (func(), error) {
	return filelock.Acquire(ctx, path, lockMaxWait)
}
