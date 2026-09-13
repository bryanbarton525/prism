package downstreammcp

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
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
