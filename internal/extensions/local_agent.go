package extensions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type LocalAgentService struct {
	store *Store
}

func NewLocalAgentService(stateDir string) *LocalAgentService {
	return &LocalAgentService{store: NewStore(stateDir)}
}

type InstallLocalAgentRequest struct {
	Source  string
	As      string
	Replace bool
	DryRun  bool
}

func (s *LocalAgentService) InstallLocalAgent(ctx context.Context, req InstallLocalAgentRequest) (ManifestEntry, error) {
	source := req.Source
	info, err := os.Stat(source)
	if err != nil {
		return ManifestEntry{}, err
	}
	if info.IsDir() {
		source = filepath.Join(source, filepath.Base(source)+".md")
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return ManifestEntry{}, fmt.Errorf("reading agent source: %w", err)
	}
	identity := strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
	if req.As != "" {
		identity = req.As
	}
	manifest, _, err := s.store.RecoverAndLoadManifest(ctx)
	if err != nil {
		return ManifestEntry{}, err
	}
	for _, entry := range manifest.Entries {
		if strings.EqualFold(entry.Kind, "agent") && strings.EqualFold(entry.Identity, identity) && !req.Replace {
			return ManifestEntry{}, fmt.Errorf("agent %q already exists; pass --replace", identity)
		}
	}
	digest, objectPath, err := s.store.PutObject(data)
	if err != nil {
		return ManifestEntry{}, err
	}
	entry := ManifestEntry{
		Identity:   identity,
		Kind:       "agent",
		Source:     "local",
		Digest:     digest,
		ObjectPath: objectPath,
	}
	if req.DryRun {
		return entry, nil
	}
	tx, err := s.store.BeginTransaction(ctx)
	if err != nil {
		return ManifestEntry{}, err
	}
	tx.UpsertEntry(entry)
	if err := tx.Commit(); err != nil {
		return ManifestEntry{}, err
	}
	return entry, nil
}

func (s *LocalAgentService) ListManagedAgents(ctx context.Context) ([]ManifestEntry, error) {
	manifest, _, err := s.store.RecoverAndLoadManifest(ctx)
	if err != nil {
		return nil, err
	}
	out := []ManifestEntry{}
	for _, entry := range manifest.Entries {
		if strings.EqualFold(entry.Kind, "agent") {
			out = append(out, entry)
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Identity) < strings.ToLower(out[j].Identity) })
	return out, nil
}

func (s *LocalAgentService) RemoveManagedAgent(ctx context.Context, name string, dryRun bool) (bool, error) {
	manifest, _, err := s.store.RecoverAndLoadManifest(ctx)
	if err != nil {
		return false, err
	}
	kept := make([]ManifestEntry, 0, len(manifest.Entries))
	removed := false
	for _, entry := range manifest.Entries {
		if strings.EqualFold(entry.Kind, "agent") && strings.EqualFold(entry.Identity, name) {
			removed = true
			continue
		}
		kept = append(kept, entry)
	}
	if !removed || dryRun {
		return removed, nil
	}
	tx, err := s.store.BeginTransaction(ctx)
	if err != nil {
		return false, err
	}
	tx.working.Entries = kept
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *LocalAgentService) RenameManagedAgent(ctx context.Context, from, to string, dryRun bool) (bool, error) {
	if strings.EqualFold(strings.TrimSpace(from), strings.TrimSpace(to)) {
		return false, fmt.Errorf("rename source and target must differ")
	}
	manifest, _, err := s.store.RecoverAndLoadManifest(ctx)
	if err != nil {
		return false, err
	}
	found := false
	for _, entry := range manifest.Entries {
		if strings.EqualFold(entry.Kind, "agent") && strings.EqualFold(entry.Identity, to) {
			return false, fmt.Errorf("agent %q already exists", to)
		}
	}
	next := make([]ManifestEntry, 0, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		if strings.EqualFold(entry.Kind, "agent") && strings.EqualFold(entry.Identity, from) {
			found = true
			entry.Identity = to
		}
		next = append(next, entry)
	}
	if !found || dryRun {
		return found, nil
	}
	tx, err := s.store.BeginTransaction(ctx)
	if err != nil {
		return false, err
	}
	tx.working.Entries = next
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *LocalAgentService) CopyManagedAgent(ctx context.Context, from, to string, dryRun bool) (bool, error) {
	if strings.EqualFold(strings.TrimSpace(from), strings.TrimSpace(to)) {
		return false, fmt.Errorf("copy source and target must differ")
	}
	manifest, _, err := s.store.RecoverAndLoadManifest(ctx)
	if err != nil {
		return false, err
	}
	for _, entry := range manifest.Entries {
		if strings.EqualFold(entry.Kind, "agent") && strings.EqualFold(entry.Identity, to) {
			return false, fmt.Errorf("agent %q already exists", to)
		}
	}
	var copied *ManifestEntry
	for _, entry := range manifest.Entries {
		if strings.EqualFold(entry.Kind, "agent") && strings.EqualFold(entry.Identity, from) {
			clone := entry.Clone()
			clone.Identity = to
			copied = &clone
			break
		}
	}
	if copied == nil || dryRun {
		return copied != nil, nil
	}
	tx, err := s.store.BeginTransaction(ctx)
	if err != nil {
		return false, err
	}
	tx.UpsertEntry(*copied)
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}
