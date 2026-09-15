package extensions

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing/fstest"

	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/skill"
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

// DiscoverSkillsFS discovers Agent Skills from a filesystem returned by a
// bounded source resolver. Paths are slash-separated and relative to fsys.
func DiscoverSkillsFS(fsys fs.FS, rootName string, opts DiscoverSkillsOptions) ([]DiscoveredSkill, error) {
	if rootName == "" {
		rootName = "imported-skill"
	}
	skills := []DiscoveredSkill{}
	if _, err := fs.Stat(fsys, "SKILL.md"); err == nil {
		name, err := skillDocumentName(fsys, "SKILL.md")
		if err != nil {
			return nil, err
		}
		skills = append(skills, DiscoveredSkill{Name: name, Path: "."})
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	} else {
		entries, err := fs.ReadDir(fsys, ".")
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			candidate := entry.Name()
			if _, err := fs.Stat(fsys, path.Join(candidate, "SKILL.md")); err == nil {
				name, err := skillDocumentName(fsys, path.Join(candidate, "SKILL.md"))
				if err != nil {
					return nil, err
				}
				skills = append(skills, DiscoveredSkill{Name: name, Path: candidate})
			}
		}
		// Repository sources commonly place skills below a top-level skills/
		// directory. Treat that directory as a container, not as a skill.
		if len(skills) == 0 {
			if entries, err := fs.ReadDir(fsys, "skills"); err == nil {
				for _, entry := range entries {
					if !entry.IsDir() {
						continue
					}
					candidate := path.Join("skills", entry.Name())
					if _, err := fs.Stat(fsys, path.Join(candidate, "SKILL.md")); err == nil {
						name, err := skillDocumentName(fsys, path.Join(candidate, "SKILL.md"))
						if err != nil {
							return nil, err
						}
						skills = append(skills, DiscoveredSkill{Name: name, Path: candidate})
					}
				}
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			}
		}
	}
	return selectDiscoveredSkills(skills, opts, rootName)
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
		name, err := skillDocumentName(os.DirFS(source), "SKILL.md")
		if err != nil {
			return nil, err
		}
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
				name, err := skillDocumentName(os.DirFS(candidate), "SKILL.md")
				if err != nil {
					return nil, err
				}
				skills = append(skills, DiscoveredSkill{Name: name, Path: candidate})
			}
		}
	}
	return selectDiscoveredSkills(skills, opts, source)
}

func selectDiscoveredSkills(skills []DiscoveredSkill, opts DiscoverSkillsOptions, source string) ([]DiscoveredSkill, error) {
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	for i := 1; i < len(skills); i++ {
		if strings.EqualFold(skills[i-1].Name, skills[i].Name) {
			return nil, fmt.Errorf("ambiguous duplicate skill identity %q in %s", skills[i].Name, source)
		}
	}
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

func skillDocumentName(fsys fs.FS, name string) (string, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return "", err
	}
	sk, err := skill.ParseDocument(data, name)
	if err != nil {
		return "", err
	}
	return sk.Name, nil
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
	planned := make([]ManifestEntry, 0, len(skills))
	for _, sk := range skills {
		if err := ValidateIdentity(sk.Name); err != nil {
			return nil, fmt.Errorf("validate skill identity: %w", err)
		}
		if err := validateLocalSkillPackage(sk.Path); err != nil {
			return nil, err
		}
		packageFS, err := rewrittenSkillFS(os.DirFS(sk.Path), ".", sk.Name)
		if err != nil {
			return nil, err
		}
		digest, err := fsDirectoryDigest(packageFS, ".")
		if err != nil {
			return nil, err
		}
		planned = append(planned, ManifestEntry{
			Identity:   sk.Name,
			Kind:       "skill",
			Source:     "local",
			Digest:     digest,
			ObjectPath: filepath.Join(s.store.ObjectsDir(), digest),
		})
	}
	if req.DryRun {
		return planned, nil
	}
	tx, err := s.store.BeginTransaction(ctx)
	if err != nil {
		return nil, err
	}
	changed := false
	for index := range planned {
		entry := &planned[index]
		if current, ok := findManifestEntry(tx.working, "skill", entry.Identity); ok {
			if strings.EqualFold(current.Digest, entry.Digest) {
				if _, err := verifyManagedObject(CatalogItem{ID: current.Identity, Digest: current.Digest, ObjectPath: current.ObjectPath, ObjectRoot: s.store.ObjectRoot()}); err != nil {
					_ = tx.Rollback()
					return nil, fmt.Errorf("existing skill %q object integrity: %w", entry.Identity, err)
				}
				*entry = current
				continue
			}
			if !req.Replace {
				_ = tx.Rollback()
				return nil, fmt.Errorf("skill %q already exists; pass --replace", entry.Identity)
			}
		}
		packageFS, err := rewrittenSkillFS(os.DirFS(skills[index].Path), ".", entry.Identity)
		if err != nil {
			_ = tx.Rollback()
			return nil, err
		}
		digest, objectPath, err := s.store.putFSDirectory(packageFS, ".")
		if err != nil {
			_ = tx.Rollback()
			return nil, err
		}
		entry.Digest, entry.ObjectPath = digest, objectPath
		tx.UpsertEntry(*entry)
		changed = true
	}
	if !changed {
		return planned, tx.AbortUnchanged()
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return planned, nil
}

// InstallResolvedSkills materializes selected skills from a resolver-owned
// filesystem into the content-addressed store before the resolver is cleaned
// up. It supports local and remote resolvers without exposing source paths in
// runtime object references.
func (s *LocalSkillService) InstallResolvedSkills(ctx context.Context, fsys fs.FS, rootName, source string, opts DiscoverSkillsOptions, replace, dryRun bool) ([]ManifestEntry, error) {
	return s.InstallResolvedSkillsWithProvenance(ctx, fsys, rootName, source, "", "", "", opts, replace, dryRun)
}

func (s *LocalSkillService) InstallResolvedSkillsWithProvenance(ctx context.Context, fsys fs.FS, rootName, source, revision, subpath, sourceDigest string, opts DiscoverSkillsOptions, replace, dryRun bool) ([]ManifestEntry, error) {
	skills, err := DiscoverSkillsFS(fsys, rootName, opts)
	if err != nil {
		return nil, err
	}
	planned := make([]ManifestEntry, 0, len(skills))
	for _, skill := range skills {
		if err := ValidateIdentity(skill.Name); err != nil {
			return nil, fmt.Errorf("validate skill identity: %w", err)
		}
		if err := validateFSSkillPackage(fsys, skill.Path); err != nil {
			return nil, err
		}
		packageFS, err := rewrittenSkillFS(fsys, skill.Path, skill.Name)
		if err != nil {
			return nil, err
		}
		digest, err := fsDirectoryDigest(packageFS, ".")
		if err != nil {
			return nil, err
		}
		planned = append(planned, ManifestEntry{Identity: skill.Name, Kind: "skill", Source: source, Revision: revision, Subpath: path.Join(subpath, skill.Path), Digest: digest, ObjectPath: filepath.Join(s.store.ObjectsDir(), digest), Provenance: Provenance{SourceDigest: sourceDigest}})
	}
	if dryRun {
		return planned, nil
	}
	tx, err := s.store.BeginTransaction(ctx)
	if err != nil {
		return nil, err
	}
	changed := false
	for index := range planned {
		entry := &planned[index]
		if current, ok := findManifestEntry(tx.working, "skill", entry.Identity); ok {
			if strings.EqualFold(current.Digest, entry.Digest) {
				if _, err := verifyManagedObject(CatalogItem{ID: current.Identity, Digest: current.Digest, ObjectPath: current.ObjectPath, ObjectRoot: s.store.ObjectRoot()}); err != nil {
					_ = tx.Rollback()
					return nil, fmt.Errorf("existing skill %q object integrity: %w", entry.Identity, err)
				}
				*entry = current
				continue
			}
			if !replace {
				_ = tx.Rollback()
				return nil, fmt.Errorf("skill %q already exists; pass --replace", entry.Identity)
			}
		}
		packageFS, err := rewrittenSkillFS(fsys, skills[index].Path, entry.Identity)
		if err != nil {
			_ = tx.Rollback()
			return nil, err
		}
		digest, objectPath, err := s.store.putFSDirectory(packageFS, ".")
		if err != nil {
			_ = tx.Rollback()
			return nil, err
		}
		entry.Digest, entry.ObjectPath = digest, objectPath
		tx.UpsertEntry(*entry)
		changed = true
	}
	if !changed {
		return planned, tx.AbortUnchanged()
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
	if err := ValidateIdentity(name); err != nil {
		return false, err
	}
	if dryRun {
		manifest, _, err := s.store.RecoverAndLoadManifest(ctx)
		if err != nil {
			return false, err
		}
		for _, entry := range manifest.Entries {
			if strings.EqualFold(entry.Kind, "skill") && strings.EqualFold(entry.Identity, name) {
				return true, nil
			}
		}
		return false, nil
	}
	tx, err := s.store.BeginTransaction(ctx)
	if err != nil {
		return false, err
	}
	if err := ensureSkillUnbound(tx.working, name); err != nil {
		_ = tx.Rollback()
		return false, err
	}
	kept := make([]ManifestEntry, 0, len(tx.working.Entries))
	removed := false
	for _, entry := range tx.working.Entries {
		if strings.EqualFold(entry.Kind, "skill") && strings.EqualFold(entry.Identity, name) {
			removed = true
			continue
		}
		kept = append(kept, entry)
	}
	if !removed {
		_ = tx.Rollback()
		return removed, nil
	}
	tx.working.Entries = kept
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// RenameManagedSkill creates a new immutable object whose SKILL.md name
// matches the manifest identity. Existing managed-agent bindings are updated
// in the same transaction.
func (s *LocalSkillService) RenameManagedSkill(ctx context.Context, from, to string, dryRun bool) (bool, error) {
	if strings.EqualFold(strings.TrimSpace(from), strings.TrimSpace(to)) {
		return false, fmt.Errorf("rename source and target must differ")
	}
	if err := ValidateIdentity(from); err != nil {
		return false, err
	}
	if err := ValidateIdentity(to); err != nil {
		return false, err
	}
	manifest, _, err := s.store.RecoverAndLoadManifest(ctx)
	if err != nil {
		return false, err
	}
	entry, found := findManifestEntry(manifest, "skill", from)
	if !found {
		return false, nil
	}
	if _, exists := findManifestEntry(manifest, "skill", to); exists {
		return false, fmt.Errorf("skill %q already exists", to)
	}
	packageFS, err := rewrittenSkillFS(os.DirFS(entry.ObjectPath), ".", to)
	if err != nil {
		return false, err
	}
	if dryRun {
		return true, nil
	}
	tx, err := s.store.BeginTransaction(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	entry, found = findManifestEntry(tx.working, "skill", from)
	if !found {
		return false, nil
	}
	if _, exists := findManifestEntry(tx.working, "skill", to); exists {
		return false, fmt.Errorf("skill %q already exists", to)
	}
	packageFS, err = rewrittenSkillFS(os.DirFS(entry.ObjectPath), ".", to)
	if err != nil {
		return false, err
	}
	digest, objectPath, err := s.store.putFSDirectory(packageFS, ".")
	if err != nil {
		return false, err
	}
	for i := range tx.working.Entries {
		current := &tx.working.Entries[i]
		if strings.EqualFold(current.Kind, "skill") && strings.EqualFold(current.Identity, from) {
			current.Identity, current.Digest, current.ObjectPath = to, digest, objectPath
			continue
		}
		if !strings.EqualFold(current.Kind, "agent") {
			continue
		}
		spec, parseErr := managedAgentSpec(*current)
		if parseErr != nil {
			return false, fmt.Errorf("parse managed agent %q: %w", current.Identity, parseErr)
		}
		changed := false
		for j, name := range spec.AllowedSkills {
			if strings.EqualFold(name, from) {
				spec.AllowedSkills[j] = to
				changed = true
			}
		}
		if changed {
			updatedSkills := append([]string(nil), spec.AllowedSkills...)
			*current, err = (&LocalAgentService{store: s.store}).mutateAgentObject(*current, func(target *agent.Spec, _ *ManifestEntry) error {
				target.AllowedSkills = updatedSkills
				return nil
			}, true)
			if err != nil {
				return false, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func rewrittenSkillFS(fsys fs.FS, root, identity string) (fs.FS, error) {
	sub, err := fs.Sub(fsys, root)
	if err != nil {
		return nil, err
	}
	result := fstest.MapFS{}
	err = fs.WalkDir(sub, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(sub, name)
		if err != nil {
			return err
		}
		if name == "SKILL.md" {
			sk, err := skill.ParseDocument(data, name)
			if err != nil {
				return err
			}
			sk.Name = identity
			data, err = skill.Render(sk)
			if err != nil {
				return err
			}
		}
		result[name] = &fstest.MapFile{Data: data, Mode: 0o444}
		return nil
	})
	return result, err
}

func ensureSkillUnbound(manifest Manifest, name string) error {
	for _, entry := range manifest.Entries {
		if !strings.EqualFold(entry.Kind, "agent") {
			continue
		}
		spec, err := managedAgentSpec(entry)
		if err != nil {
			return fmt.Errorf("parse managed agent %q before removing skill: %w", entry.Identity, err)
		}
		for _, allowed := range spec.AllowedSkills {
			if strings.EqualFold(allowed, name) {
				return fmt.Errorf("skill %q is still referenced by managed agent %q", name, entry.Identity)
			}
		}
	}
	return nil
}

func managedAgentSpec(entry ManifestEntry) (*agent.Spec, error) {
	info, err := os.Lstat(entry.ObjectPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		data, err := os.ReadFile(entry.ObjectPath)
		if err != nil {
			return nil, err
		}
		return agent.ParseManaged(data, "")
	}
	files, err := readAgentPackage(entry.ObjectPath)
	if err != nil {
		return nil, err
	}
	if data, ok := files[entry.Identity+".md"]; ok {
		return agent.ParseManaged(data, entry.Identity+".md")
	}
	return nil, fmt.Errorf("agent package does not contain identity %q", entry.Identity)
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
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("package entry %q is not a regular file", path)
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
		writeObjectFrame(hash, []byte(rel))
		writeObjectFrame(hash, content)
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	objectPath := filepath.Join(s.ObjectsDir(), digest)
	if exists, err := verifyExistingDirectoryObject(objectPath, digest); err != nil {
		return "", "", err
	} else if exists {
		return digest, objectPath, nil
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
		if exists, verifyErr := verifyExistingDirectoryObject(objectPath, digest); verifyErr != nil || !exists {
			return "", "", fmt.Errorf("publish skill object %q: %w (existing object verification: %v)", objectPath, err, verifyErr)
		}
	}
	return digest, objectPath, nil
}

func (s *Store) putFSDirectory(fsys fs.FS, root string) (string, string, error) {
	sub, err := fs.Sub(fsys, root)
	if err != nil {
		return "", "", err
	}
	paths := []string{}
	if err := fs.WalkDir(sub, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("symlink %q is not supported", name)
		}
		if !entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("package entry %q is not a regular file", name)
			}
			paths = append(paths, filepath.ToSlash(name))
		}
		return nil
	}); err != nil {
		return "", "", err
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, name := range paths {
		data, err := fs.ReadFile(sub, name)
		if err != nil {
			return "", "", err
		}
		writeObjectFrame(hash, []byte(name))
		writeObjectFrame(hash, data)
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	objectPath := filepath.Join(s.ObjectsDir(), digest)
	if exists, err := verifyExistingDirectoryObject(objectPath, digest); err != nil {
		return "", "", err
	} else if exists {
		return digest, objectPath, nil
	}
	if err := os.MkdirAll(s.ObjectsDir(), 0o755); err != nil {
		return "", "", err
	}
	staged, err := os.MkdirTemp(s.ObjectsDir(), ".staged-skill-*")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(staged)
	for _, name := range paths {
		data, err := fs.ReadFile(sub, name)
		if err != nil {
			return "", "", err
		}
		dest := filepath.Join(staged, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return "", "", err
		}
		if err := writeFileAtomically(dest, data); err != nil {
			return "", "", err
		}
	}
	if err := os.Rename(staged, objectPath); err != nil {
		if exists, verifyErr := verifyExistingDirectoryObject(objectPath, digest); verifyErr != nil || !exists {
			return "", "", fmt.Errorf("publish skill object %q: %w (existing object verification: %v)", objectPath, err, verifyErr)
		}
	}
	return digest, objectPath, nil
}

func verifyExistingDirectoryObject(objectPath, digest string) (bool, error) {
	info, err := os.Lstat(objectPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("existing skill object %q is not a regular directory", objectPath)
	}
	actual, err := directoryDigest(objectPath)
	if err != nil {
		return false, fmt.Errorf("verify existing skill object %q: %w", objectPath, err)
	}
	if actual != digest {
		return false, fmt.Errorf("existing skill object %q digest mismatch: got %s want %s", objectPath, actual, digest)
	}
	return true, nil
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
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("package entry %q is not a regular file", path)
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
		writeObjectFrame(hash, []byte(rel))
		writeObjectFrame(hash, content)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// DigestDirectory returns the immutable object digest for a local package.
func DigestDirectory(sourceDir string) (string, error) {
	return directoryDigest(sourceDir)
}

func findManifestEntry(manifest Manifest, kind, identity string) (ManifestEntry, bool) {
	for _, entry := range manifest.Entries {
		if strings.EqualFold(entry.Kind, kind) && strings.EqualFold(entry.Identity, identity) {
			return entry, true
		}
	}
	return ManifestEntry{}, false
}

func validateLocalSkillPackage(source string) error {
	root := filepath.Dir(source)
	name := filepath.Base(source)
	fsys := os.DirFS(root)
	if err := skill.ValidatePortableStructure(fsys, name); err != nil {
		return err
	}
	_, err := skill.LoadDir(fsys, name)
	return err
}

func validateFSSkillPackage(fsys fs.FS, root string) error {
	sub, err := fs.Sub(fsys, root)
	if err != nil {
		return err
	}
	if err := skill.ValidatePortableStructure(sub, "."); err != nil {
		return err
	}
	data, err := fs.ReadFile(sub, "SKILL.md")
	if err != nil {
		return err
	}
	return skill.ValidateDocument(data, "SKILL.md")
}

func fsDirectoryDigest(fsys fs.FS, root string) (string, error) {
	sub, err := fs.Sub(fsys, root)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	paths := []string{}
	if err := fs.WalkDir(sub, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("symlink %q is not supported", name)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("package entry %q is not a regular file", name)
		}
		paths = append(paths, filepath.ToSlash(name))
		return nil
	}); err != nil {
		return "", err
	}
	sort.Strings(paths)
	for _, name := range paths {
		data, err := fs.ReadFile(sub, name)
		if err != nil {
			return "", err
		}
		writeObjectFrame(hash, []byte(name))
		writeObjectFrame(hash, data)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func writeObjectFrame(writer io.Writer, value []byte) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = writer.Write(size[:])
	_, _ = writer.Write(value)
}
