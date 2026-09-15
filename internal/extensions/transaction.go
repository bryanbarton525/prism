package extensions

import (
	"context"
	"fmt"
	"reflect"
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

// ReplaceManifest stages a complete, previously validated manifest. It is
// intended for compensating a multi-step activation after a later step fails.
func (tx *Transaction) ReplaceManifest(manifest Manifest) {
	tx.working = manifest.Clone()
}

// StageMCPAccess includes the access policy in the same recovery journal and
// publication boundary as the extension manifest.
func (tx *Transaction) StageMCPAccess(state MCPAccessState) error {
	state, err := normalizeMCPAccess(state)
	if err != nil {
		return err
	}
	tx.working.MCPAccess = &state
	return nil
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
		// The manifest has already been durably committed. Mark the journal as
		// committed before returning so recovery never restores its before-image.
		if markErr := tx.store.markJournalCommitted(); markErr != nil {
			tx.unlock()
			tx.closed = true
			return fmt.Errorf("commit succeeded but finalizing recovery journal failed: %v (mark failed: %w)", err, markErr)
		}
		tx.unlock()
		tx.closed = true
		// The committed marker makes the remaining cleanup idempotent and
		// prevents callers from compensating a state that was already published.
		return nil
	}
	tx.unlock()
	tx.closed = true
	return nil
}

func (tx *Transaction) Rollback() error {
	if tx.closed {
		return nil
	}
	err := tx.store.restoreJournalState()
	if err == nil {
		if clearErr := tx.store.clearJournal(); clearErr != nil {
			err = clearErr
		}
	}
	tx.unlock()
	tx.closed = true
	return err
}

// AbortUnchanged releases a transaction that planned no mutable changes
// without rewriting the manifest or creating a new activation timestamp.
func (tx *Transaction) AbortUnchanged() error {
	if tx.closed {
		return fmt.Errorf("transaction already closed")
	}
	if !reflect.DeepEqual(tx.working, tx.previous) {
		return fmt.Errorf("cannot abort a changed transaction without rollback")
	}
	err := tx.store.clearJournal()
	tx.unlock()
	tx.closed = true
	return err
}

func normalizeKind(kind string) string {
	return strings.ToLower(strings.TrimSpace(kind))
}
