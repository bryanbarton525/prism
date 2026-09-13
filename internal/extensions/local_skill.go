package extensions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type LocalSkillService struct {
	store *Store
}

func NewLocalSkillService(stateDir string) *LocalSkillService {
	return &LocalSkillService{store: NewStore(stateDir)}
}

type DiscoverSkillsOptions struct {
	All   bool
	Names []string
}

type DiscoveredSkill struct {
	Name string
	Path string
}

func DiscoverLocalSkills(source string, opts DiscoverSkillsOptions) ([]DiscoveredSkill, error) {
	info, err := os.Stat(source)
	if err != nil {
		return nil, err
	}
	skills := []DiscoveredSkill{}
	if !info.IsDir() {
		return nil, fmt.Errorf("source must be a directory")
	}
	// Single skill source directory.
	if _, err := os.Stat(filepath.Join(source, "SKILL.md")); err == nil {
		name := filepath.Base(source)
		skills = append(skills, DiscoveredSkill{Name: name, Path: source})
	} else {
		entries, err := os.ReadDir(source)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			candidate := filepath.Join(source, e.Name())
			if _, err := os.Stat(filepath.Join(candidate, "SKILL.md")); err == nil {
				skills = append(skills, DiscoveredSkill{Name: e.Name(), Path: candidate})
			}
		}
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	if len(opts.Names) == 0 && !opts.All {
		if len(skills) == 1 {
			return skills, nil
		}
		return nil, fmt.Errorf("multiple skills discovered; pass --all or --name")
	}
	if opts.All {
		return skills, nil
	}
	set := map[string]struct{}{}
	for _, name := range opts.Names {
		set[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
	}
	filtered := []DiscoveredSkill{}
	for _, sk := range skills {
		if _, ok := set[strings.ToLower(sk.Name)]; ok {
			filtered = append(filtered, sk)
		}
	}
	if len(filtered) != len(set) {
		return nil, fmt.Errorf("one or more named skills were not discovered in %s", source)
	}
	return filtered, nil
}

type InstallLocalSkillsRequest struct {
	Source   string
	Discover DiscoverSkillsOptions
	As       string
	Replace  bool
	DryRun   bool
}

func (s *LocalSkillService) InstallLocalSkills(ctx context.Context, req InstallLocalSkillsRequest) ([]ManifestEntry, error) {
	skills, err := DiscoverLocalSkills(req.Source, req.Discover)
	if err != nil {
		return nil, err
	}
	if req.As != "" {
		if len(skills) != 1 {
			return nil, fmt.Errorf("--as requires exactly one selected skill")
		}
		skills[0].Name = req.As
	}
	manifest, _, err := s.store.RecoverAndLoadManifest(ctx)
	if err != nil {
		return nil, err
	}
	existing := map[string]ManifestEntry{}
	for _, entry := range manifest.Entries {
		if strings.EqualFold(entry.Kind, "skill") {
			existing[strings.ToLower(entry.Identity)] = entry
		}
	}
	planned := make([]ManifestEntry, 0, len(skills))
	for _, sk := range skills {
		if cur, ok := existing[strings.ToLower(sk.Name)]; ok && !req.Replace && cur.ObjectPath != sk.Path {
			return nil, fmt.Errorf("skill %q already exists; pass --replace", sk.Name)
		}
		digest, objPath, err := s.store.putDirectoryObject(sk.Path)
		if err != nil {
			return nil, err
		}
		planned = append(planned, ManifestEntry{
			Identity:   sk.Name,
			Kind:       "skill",
			Source:     "local",
			Digest:     digest,
			ObjectPath: objPath,
		})
	}
	if req.DryRun {
		return planned, nil
	}
	tx, err := s.store.BeginTransaction(ctx)
	if err != nil {
		return nil, err
	}
	for _, entry := range planned {
		tx.UpsertEntry(entry)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return planned, nil
}

func (s *LocalSkillService) ListManagedSkills(ctx context.Context) ([]ManifestEntry, error) {
	manifest, _, err := s.store.RecoverAndLoadManifest(ctx)
	if err != nil {
		return nil, err
	}
	out := []ManifestEntry{}
	for _, entry := range manifest.Entries {
		if strings.EqualFold(entry.Kind, "skill") {
			out = append(out, entry)
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Identity) < strings.ToLower(out[j].Identity) })
	return out, nil
}

func (s *LocalSkillService) RemoveManagedSkill(ctx context.Context, name string, dryRun bool) (bool, error) {
	manifest, _, err := s.store.RecoverAndLoadManifest(ctx)
	if err != nil {
		return false, err
	}
	kept := make([]ManifestEntry, 0, len(manifest.Entries))
	removed := false
	for _, entry := range manifest.Entries {
		if strings.EqualFold(entry.Kind, "skill") && strings.EqualFold(entry.Identity, name) {
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

func (s *Store) putDirectoryObject(sourceDir string) (string, string, error) {
	hash := sha256.New()
	paths := []string{}
	err := filepath.WalkDir(sourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink %q is not supported", path)
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return "", "", err
	}
	sort.Strings(paths)
	for _, rel := range paths {
		abs := filepath.Join(sourceDir, filepath.FromSlash(rel))
		content, err := os.ReadFile(abs)
		if err != nil {
			return "", "", err
		}
		_, _ = io.WriteString(hash, rel)
		_, _ = io.WriteString(hash, "\x00")
		_, _ = hash.Write(content)
		_, _ = io.WriteString(hash, "\x00")
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	objectPath := filepath.Join(s.ObjectsDir(), digest)
	if info, err := os.Stat(objectPath); err == nil && info.IsDir() {
		existingDigest, err := directoryDigest(objectPath)
		if err == nil && existingDigest == digest {
			return digest, objectPath, nil
		}
		if err := os.RemoveAll(objectPath); err != nil {
			return "", "", fmt.Errorf("remove corrupt skill object %q: %w", objectPath, err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return "", "", err
	}
	if err := os.MkdirAll(s.ObjectsDir(), 0o755); err != nil {
		return "", "", err
	}
	stagedPath, err := os.MkdirTemp(s.ObjectsDir(), ".staged-skill-*")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(stagedPath)
	for _, rel := range paths {
		src := filepath.Join(sourceDir, filepath.FromSlash(rel))
		dst := filepath.Join(stagedPath, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return "", "", err
		}
		content, err := os.ReadFile(src)
		if err != nil {
			return "", "", err
		}
		if err := writeFileAtomically(dst, content); err != nil {
			return "", "", err
		}
	}
	if err := os.Rename(stagedPath, objectPath); err != nil {
		if !os.IsExist(err) {
			return "", "", err
		}
		existingDigest, verifyErr := directoryDigest(objectPath)
		if verifyErr != nil || existingDigest != digest {
			return "", "", fmt.Errorf("concurrent skill object %q is incomplete or corrupt", objectPath)
		}
	}
	return digest, objectPath, nil
}

func directoryDigest(sourceDir string) (string, error) {
	hash := sha256.New()
	paths := []string{}
	err := filepath.WalkDir(sourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink %q is not supported", path)
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	for _, rel := range paths {
		content, err := os.ReadFile(filepath.Join(sourceDir, filepath.FromSlash(rel)))
		if err != nil {
			return "", err
		}
		_, _ = io.WriteString(hash, rel)
		_, _ = io.WriteString(hash, "\x00")
		_, _ = hash.Write(content)
		_, _ = io.WriteString(hash, "\x00")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
