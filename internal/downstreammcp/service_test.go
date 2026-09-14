package downstreammcp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServiceAddOrUpdateOutcomes(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "mcp-servers.yaml")
	service := NewService(path)

	initial := Server{Name: "linear", Transport: TransportCommand, Command: "npx", Args: []string{"-y", "mcp-remote", "https://mcp.linear.app/mcp"}}
	res, err := service.AddOrUpdate(ctx, initial, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != OutcomeCreated {
		t.Fatalf("outcome = %q, want %q", res.Outcome, OutcomeCreated)
	}

	res, err = service.AddOrUpdate(ctx, initial, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != OutcomeUnchanged {
		t.Fatalf("outcome = %q, want %q", res.Outcome, OutcomeUnchanged)
	}

	updated := initial
	updated.MaxBytes = 1000
	res, err = service.AddOrUpdate(ctx, updated, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != OutcomeConflict {
		t.Fatalf("outcome = %q, want %q", res.Outcome, OutcomeConflict)
	}

	res, err = service.AddOrUpdate(ctx, updated, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != OutcomeReplaced {
		t.Fatalf("outcome = %q, want %q", res.Outcome, OutcomeReplaced)
	}

	state, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	server, ok := state.Get("linear")
	if !ok {
		t.Fatal("expected linear server")
	}
	if server.MaxBytes != 1000 {
		t.Fatalf("max_bytes = %d, want 1000", server.MaxBytes)
	}
}

func TestServiceRejectsInvalidHTTPURLs(t *testing.T) {
	service := NewService(filepath.Join(t.TempDir(), "mcp-servers.yaml"))
	for _, raw := range []string{"", "/mcp", "ftp://example.com/mcp", "https:///mcp"} {
		_, err := service.AddOrUpdate(context.Background(), Server{Name: "remote", Transport: TransportSSE, URL: raw}, false)
		if err == nil {
			t.Fatalf("URL %q unexpectedly accepted", raw)
		}
	}
}

func TestServiceRemoveOutcomes(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "mcp-servers.yaml")
	service := NewService(path)

	_, err := service.AddOrUpdate(ctx, Server{Name: "linear", Transport: TransportCommand, Command: "npx"}, false)
	if err != nil {
		t.Fatal(err)
	}

	res, err := service.Remove(ctx, "linear")
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != OutcomeRemoved {
		t.Fatalf("outcome = %q, want %q", res.Outcome, OutcomeRemoved)
	}

	res, err = service.Remove(ctx, "linear")
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != OutcomeUnchanged {
		t.Fatalf("outcome = %q, want %q", res.Outcome, OutcomeUnchanged)
	}
}

func TestFileStoreWriteDetectsExternalEdit(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "mcp-servers.yaml")
	if err := Save(path, State{Servers: []Server{{Name: "one", Transport: TransportCommand, Command: "cmd"}}}); err != nil {
		t.Fatal(err)
	}
	store := NewFileStore(path)
	snapshot, err := store.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := Save(path, State{Servers: []Server{{Name: "two", Transport: TransportCommand, Command: "cmd"}}}); err != nil {
		t.Fatal(err)
	}

	snapshot.State.Upsert(Server{Name: "three", Transport: TransportCommand, Command: "cmd"})
	err = store.Write(ctx, snapshot)
	if !errors.Is(err, ErrExternalEdit) {
		t.Fatalf("err = %v, want ErrExternalEdit", err)
	}
}

func TestServiceAddOrUpdateCreatesMissingStateDirectory(t *testing.T) {
	ctx := context.Background()
	statePath := filepath.Join(t.TempDir(), "not-created", "mcp-servers.yaml")
	service := NewService(statePath)
	res, err := service.AddOrUpdate(ctx, Server{Name: "linear", Transport: TransportCommand, Command: "npx"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != OutcomeCreated {
		t.Fatalf("outcome = %q", res.Outcome)
	}
}

func TestAcquireLockRecoversStaleLockFile(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "mcp-servers.yaml.lock")
	if err := os.WriteFile(lockPath, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * staleLockAge)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatal(err)
	}
	unlock, err := acquireLock(context.Background(), lockPath)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
}
