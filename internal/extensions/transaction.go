package extensions

import (
	"context"
	"fmt"
	"strings"
)

type Transaction struct {
	store    *Store
	unlock   func()
	previous Manifest
	working  Manifest
	closed   bool
}

func (s *Store) BeginTransaction(ctx context.Context) (*Transaction, error) {
	unlock, err := acquireFileLock(ctx, s.LockPath())
	if err != nil {
		return nil, err
	}
	if _, err := s.recoverInterrupted(); err != nil {
		unlock()
		return nil, fmt.Errorf("recovering interrupted transaction: %w", err)
	}
	previous, err := s.LoadManifest()
	if err != nil {
		unlock()
		return nil, err
	}
	if err := s.writeJournal(previous); err != nil {
		unlock()
		return nil, err
	}
	return &Transaction{
		store:    s,
		unlock:   unlock,
		previous: previous.Clone(),
		working:  previous.Clone(),
	}, nil
}

func (tx *Transaction) UpsertEntry(entry ManifestEntry) {
	entry.InstalledAt = tx.store.now().UTC()
	keyKind := normalizeKind(entry.Kind)
	keyIdentity := strings.ToLower(strings.TrimSpace(entry.Identity))
	if keyKind == "" || keyIdentity == "" {
		return
	}
	for _, existing := range tx.working.Entries {
		if normalizeKind(existing.Kind) == keyKind && strings.ToLower(strings.TrimSpace(existing.Identity)) == keyIdentity {
			continue
		}
		if normalizeKind(existing.Kind) == keyKind && strings.EqualFold(existing.Identity, entry.Identity) {
			entry.Diagnostics = append(entry.Diagnostics, ActivationDiagnostic{
				Code:    "duplicate_kind_identity",
				Message: fmt.Sprintf("duplicate %s identity %q", keyKind, entry.Identity),
			})
		}
	}
	for i := range tx.working.Entries {
		if normalizeKind(tx.working.Entries[i].Kind) == keyKind && strings.EqualFold(tx.working.Entries[i].Identity, entry.Identity) {
			tx.working.Entries[i] = entry
			return
		}
	}
	tx.working.Entries = append(tx.working.Entries, entry)
}

func (tx *Transaction) Commit() error {
	if tx.closed {
		return fmt.Errorf("transaction already closed")
	}
	if err := tx.store.SaveManifest(tx.working); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("commit failed: %w (rollback failed: %v)", err, rbErr)
		}
		return err
	}
	if err := tx.store.clearJournal(); err != nil {
		tx.unlock()
		tx.closed = true
		return err
	}
	tx.unlock()
	tx.closed = true
	return nil
}

func (tx *Transaction) Rollback() error {
	if tx.closed {
		return nil
	}
	err := tx.store.SaveManifest(tx.previous)
	if err == nil {
		if clearErr := tx.store.clearJournal(); clearErr != nil {
			err = clearErr
		}
	}
	tx.unlock()
	tx.closed = true
	return err
}

func normalizeKind(kind string) string {
	return strings.ToLower(strings.TrimSpace(kind))
}
