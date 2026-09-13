package downstreammcp

import (
	"context"
	"errors"
)

const (
	OutcomeCreated   = "created"
	OutcomeUnchanged = "unchanged"
	OutcomeReplaced  = "replaced"
	OutcomeConflict  = "conflict"
	OutcomeRemoved   = "removed"
)

type MutateResult struct {
	Outcome string `json:"outcome"`
	Server  Server `json:"server,omitempty"`
}

type Service struct {
	store Store
}

func NewService(path string) *Service {
	return &Service{store: NewFileStore(path)}
}

func NewServiceWithStore(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) AddOrUpdate(ctx context.Context, server Server, replace bool) (MutateResult, error) {
	if err := server.Validate(); err != nil {
		return MutateResult{}, err
	}

	snapshot, err := s.store.Read(ctx)
	if err != nil {
		return MutateResult{}, err
	}

	current, ok := snapshot.State.Get(server.Name)
	server = server.withDefaults()
	if ok {
		if current.equals(server) {
			return MutateResult{Outcome: OutcomeUnchanged, Server: current}, nil
		}
		if !replace {
			return MutateResult{Outcome: OutcomeConflict, Server: current}, nil
		}
	}

	snapshot.State.Upsert(server)
	if err := s.store.Write(ctx, snapshot); err != nil {
		if errors.Is(err, ErrExternalEdit) {
			return MutateResult{Outcome: OutcomeConflict}, nil
		}
		return MutateResult{}, err
	}
	if ok {
		return MutateResult{Outcome: OutcomeReplaced, Server: server}, nil
	}
	return MutateResult{Outcome: OutcomeCreated, Server: server}, nil
}

func (s *Service) Remove(ctx context.Context, name string) (MutateResult, error) {
	snapshot, err := s.store.Read(ctx)
	if err != nil {
		return MutateResult{}, err
	}
	if _, ok := snapshot.State.Remove(name); !ok {
		return MutateResult{Outcome: OutcomeUnchanged}, nil
	}
	if err := s.store.Write(ctx, snapshot); err != nil {
		if errors.Is(err, ErrExternalEdit) {
			return MutateResult{Outcome: OutcomeConflict}, nil
		}
		return MutateResult{}, err
	}
	return MutateResult{Outcome: OutcomeRemoved}, nil
}
