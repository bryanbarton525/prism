package extensions

import (
	"context"
	"fmt"
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
		previous: previous,
		working:  previous,
	}, nil
}

func (tx *Transaction) UpsertEntry(entry ManifestEntry) {
	entry.InstalledAt = tx.store.now().UTC()
	for i := range tx.working.Entries {
		if tx.working.Entries[i].Identity == entry.Identity {
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
		_ = tx.Rollback()
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
	if clearErr := tx.store.clearJournal(); err == nil {
		err = clearErr
	}
	tx.unlock()
	tx.closed = true
	return err
}
