package extensions

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/bryanbarton525/prism/internal/agent"
)

type LocalAgentService struct {
	store *Store
}

func NewLocalAgentService(stateDir string) *LocalAgentService {
	return &LocalAgentService{store: NewStore(stateDir)}
}

type InstallLocalAgentRequest struct {
	Source        string
	As            string
	Replace       bool
	DryRun        bool
	SourceURI     string
	Revision      string
	Subpath       string
	Runtime       *RuntimeTarget
	Provenance    Provenance
	SkillBindings []SkillBinding
}

var agentIDLine = regexp.MustCompile(`(?m)^id:\s*.*$`)

func replaceAgentID(data []byte, identity string) ([]byte, bool) {
	data = bytes.TrimSpace(data)
	if !bytes.HasPrefix(data, []byte("---")) {
		return data, false
	}
	end := bytes.Index(data[3:], []byte("\n---"))
	if end < 0 {
		return data, false
	}
	end += 3
	frontmatter := data[:end]
	if !agentIDLine.Match(frontmatter) {
		return data, false
	}
	return append(agentIDLine.ReplaceAll(frontmatter, []byte("id: "+identity)), data[end:]...), true
}

func (s *LocalAgentService) InstallLocalAgent(ctx context.Context, req InstallLocalAgentRequest) (ManifestEntry, error) {
	info, err := os.Lstat(req.Source)
	if err != nil {
		return ManifestEntry{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return ManifestEntry{}, fmt.Errorf("agent source must not be a symlink")
	}
	if info.IsDir() {
		return s.installDirectory(ctx, req.Source, req)
	}
	if !info.Mode().IsRegular() {
		return ManifestEntry{}, fmt.Errorf("agent source must be a regular file or package directory")
	}
	data, err := os.ReadFile(req.Source)
	if err != nil {
		return ManifestEntry{}, fmt.Errorf("reading agent source: %w", err)
	}
	return s.installContent(ctx, strings.TrimSuffix(filepath.Base(req.Source), filepath.Ext(req.Source)), data, req)
}

// InstallAgentContent stores a fully translated agent definition without
// requiring a temporary source file. It is used by guided imports and copies.
func (s *LocalAgentService) InstallAgentContent(ctx context.Context, name string, data []byte, req InstallLocalAgentRequest) (ManifestEntry, error) {
	return s.installContent(ctx, strings.TrimSuffix(filepath.Base(name), filepath.Ext(name)), data, req)
}

// InstallTranslatedAgent preserves the original source and optional import
// decisions beside the normalized runtime definition in one immutable object.
func (s *LocalAgentService) InstallTranslatedAgent(ctx context.Context, name string, normalized, source []byte, sourceName string, importConfig []byte, req InstallLocalAgentRequest) (ManifestEntry, error) {
	entries, err := s.InstallTranslatedAgents(ctx, []TranslatedAgentInstall{{Name: name, Normalized: normalized, Source: source, SourceName: sourceName, ImportConfig: importConfig, Request: req}})
	if err != nil {
		return ManifestEntry{}, err
	}
	return entries[0], nil
}

type TranslatedAgentInstall struct {
	Name         string
	Normalized   []byte
	Source       []byte
	SourceName   string
	SupportFiles map[string][]byte
	ImportConfig []byte
	Request      InstallLocalAgentRequest
}

type plannedTranslatedAgent struct {
	entry   ManifestEntry
	files   map[string][]byte
	replace bool
}

// InstallTranslatedAgents activates a selected import batch atomically. Source
// bytes and import decisions are published in the same immutable objects; no
// agent from the batch becomes visible if validation or publication fails.
func (s *LocalAgentService) InstallTranslatedAgents(ctx context.Context, installs []TranslatedAgentInstall) ([]ManifestEntry, error) {
	if len(installs) == 0 {
		return []ManifestEntry{}, nil
	}
	for _, item := range installs[1:] {
		if item.Request.DryRun != installs[0].Request.DryRun {
			return nil, fmt.Errorf("batch agents must share dry-run mode")
		}
	}
	planned := make([]plannedTranslatedAgent, 0, len(installs))
	seen := map[string]bool{}
	for _, item := range installs {
		spec, err := agent.ParseManaged(item.Normalized, "")
		if err != nil {
			return nil, err
		}
		identity := spec.ID
		if item.Request.As != "" {
			identity = item.Request.As
			spec.ID = identity
			item.Normalized, err = agent.Render(spec)
			if err != nil {
				return nil, err
			}
		}
		if err := ValidateIdentity(identity); err != nil {
			return nil, err
		}
		key := strings.ToLower(identity)
		if seen[key] {
			return nil, fmt.Errorf("duplicate selected agent identity %q", identity)
		}
		seen[key] = true
		files := map[string][]byte{identity + ".md": append([]byte(nil), item.Normalized...)}
		for name, data := range item.SupportFiles {
			clean := path.Clean(strings.TrimSpace(name))
			if !fs.ValidPath(clean) || clean == "." || clean == identity+".md" || clean == "provenance" || strings.HasPrefix(clean, "provenance/") {
				return nil, fmt.Errorf("invalid agent support path %q", name)
			}
			files[clean] = append([]byte(nil), data...)
		}
		if len(item.Source) > 0 {
			base := filepath.Base(item.SourceName)
			if base == "." || base == "" {
				base = "source"
			}
			files[path.Join("provenance", "source", base)] = append([]byte(nil), item.Source...)
		}
		if len(item.ImportConfig) > 0 {
			files[path.Join("provenance", "import-config.yaml")] = append([]byte(nil), item.ImportConfig...)
		}
		digest := digestPackageFiles(files)
		sourceURI := item.Request.SourceURI
		if sourceURI == "" {
			sourceURI = "local"
		}
		entry := ManifestEntry{Identity: identity, Kind: "agent", Source: sourceURI, Revision: item.Request.Revision, Subpath: item.Request.Subpath, Digest: digest, ObjectPath: filepath.Join(s.store.ObjectsDir(), digest), Runtime: item.Request.Runtime, Provenance: item.Request.Provenance, SkillBindings: append([]SkillBinding{}, item.Request.SkillBindings...)}
		planned = append(planned, plannedTranslatedAgent{entry: entry, files: files, replace: item.Request.Replace})
	}
	entries := make([]ManifestEntry, len(planned))
	for i := range planned {
		entries[i] = planned[i].entry
	}
	if installs[0].Request.DryRun {
		return entries, nil
	}
	tx, err := s.store.BeginTransaction(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for _, item := range planned {
		if _, exists := findManifestEntry(tx.working, "agent", item.entry.Identity); exists && !item.replace {
			return nil, fmt.Errorf("agent %q already exists; pass --replace", item.entry.Identity)
		}
	}
	for i := range planned {
		digest, objectPath, putErr := s.store.putDirectoryContents(planned[i].files)
		if putErr != nil {
			return nil, putErr
		}
		planned[i].entry.Digest, planned[i].entry.ObjectPath = digest, objectPath
		tx.UpsertEntry(planned[i].entry)
		entries[i] = planned[i].entry
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *LocalAgentService) installContent(ctx context.Context, identity string, data []byte, req InstallLocalAgentRequest) (ManifestEntry, error) {
	spec, err := agent.ParseManaged(data, "")
	if err != nil {
		return ManifestEntry{}, fmt.Errorf("validate agent source: %w", err)
	}
	if req.As != "" {
		identity = req.As
	} else {
		identity = spec.ID
	}
	if err := ValidateIdentity(identity); err != nil {
		return ManifestEntry{}, err
	}
	if spec.ID != identity {
		var replaced bool
		data, replaced = replaceAgentID(data, identity)
		if !replaced {
			return ManifestEntry{}, fmt.Errorf("agent source does not contain an id field")
		}
	}
	if _, err := agent.ParseManaged(data, identity+".md"); err != nil {
		return ManifestEntry{}, fmt.Errorf("validate managed agent: %w", err)
	}
	sum := sha256.Sum256(data)
	sourceURI := req.SourceURI
	if sourceURI == "" {
		sourceURI = "local"
	}
	entry := ManifestEntry{
		Identity:      identity,
		Kind:          "agent",
		Source:        sourceURI,
		Revision:      req.Revision,
		Subpath:       req.Subpath,
		Digest:        hex.EncodeToString(sum[:]),
		ObjectPath:    filepath.Join(s.store.ObjectsDir(), hex.EncodeToString(sum[:])),
		Runtime:       req.Runtime,
		Provenance:    req.Provenance,
		SkillBindings: append([]SkillBinding{}, req.SkillBindings...),
	}
	if req.DryRun {
		return entry, nil
	}
	tx, err := s.store.BeginTransaction(ctx)
	if err != nil {
		return ManifestEntry{}, err
	}
	if _, exists := findManifestEntry(tx.working, "agent", identity); exists && !req.Replace {
		_ = tx.Rollback()
		return ManifestEntry{}, fmt.Errorf("agent %q already exists; pass --replace", identity)
	}
	digest, objectPath, err := s.store.PutObject(data)
	if err != nil {
		_ = tx.Rollback()
		return ManifestEntry{}, err
	}
	entry.Digest, entry.ObjectPath = digest, objectPath
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
	if err := ValidateIdentity(name); err != nil {
		return false, err
	}
	if dryRun {
		manifest, _, err := s.store.RecoverAndLoadManifest(ctx)
		if err != nil {
			return false, err
		}
		_, found := findManifestEntry(manifest, "agent", name)
		return found, nil
	}
	tx, err := s.store.BeginTransaction(ctx)
	if err != nil {
		return false, err
	}
	kept := make([]ManifestEntry, 0, len(tx.working.Entries))
	removed := false
	for _, entry := range tx.working.Entries {
		if strings.EqualFold(entry.Kind, "agent") && strings.EqualFold(entry.Identity, name) {
			removed = true
			continue
		}
		kept = append(kept, entry)
	}
	if !removed {
		_ = tx.Rollback()
		return false, nil
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
	if err := ValidateIdentity(from); err != nil {
		return false, err
	}
	if err := ValidateIdentity(to); err != nil {
		return false, err
	}
	if dryRun {
		manifest, _, err := s.store.RecoverAndLoadManifest(ctx)
		if err != nil {
			return false, err
		}
		if _, exists := findManifestEntry(manifest, "agent", to); exists {
			return false, fmt.Errorf("agent %q already exists", to)
		}
		_, found := findManifestEntry(manifest, "agent", from)
		return found, nil
	}
	tx, err := s.store.BeginTransaction(ctx)
	if err != nil {
		return false, err
	}
	if _, exists := findManifestEntry(tx.working, "agent", to); exists {
		_ = tx.Rollback()
		return false, fmt.Errorf("agent %q already exists", to)
	}
	next := make([]ManifestEntry, 0, len(tx.working.Entries))
	found := false
	for _, entry := range tx.working.Entries {
		if strings.EqualFold(entry.Kind, "agent") && strings.EqualFold(entry.Identity, from) {
			found = true
			entry, err = s.rewriteAgentObject(entry, to)
			if err != nil {
				_ = tx.Rollback()
				return false, err
			}
		}
		next = append(next, entry)
	}
	if !found {
		_ = tx.Rollback()
		return false, nil
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
	if err := ValidateIdentity(from); err != nil {
		return false, err
	}
	if err := ValidateIdentity(to); err != nil {
		return false, err
	}
	if dryRun {
		manifest, _, err := s.store.RecoverAndLoadManifest(ctx)
		if err != nil {
			return false, err
		}
		if _, exists := findManifestEntry(manifest, "agent", to); exists {
			return false, fmt.Errorf("agent %q already exists", to)
		}
		_, found := findManifestEntry(manifest, "agent", from)
		return found, nil
	}
	tx, err := s.store.BeginTransaction(ctx)
	if err != nil {
		return false, err
	}
	if _, exists := findManifestEntry(tx.working, "agent", to); exists {
		_ = tx.Rollback()
		return false, fmt.Errorf("agent %q already exists", to)
	}
	entry, found := findManifestEntry(tx.working, "agent", from)
	if !found {
		_ = tx.Rollback()
		return false, nil
	}
	copyEntry, err := s.rewriteAgentObject(entry, to)
	if err != nil {
		_ = tx.Rollback()
		return false, err
	}
	tx.UpsertEntry(copyEntry)
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// CopyBundledAgent creates an independent managed package from one immutable
// bundled agent, including a materialized constitution when the source uses a
// constitution_path.
func (s *LocalAgentService) CopyBundledAgent(ctx context.Context, bundle fs.FS, bundleDigest, from, to string, dryRun bool) (ManifestEntry, error) {
	if err := ValidateIdentity(from); err != nil {
		return ManifestEntry{}, err
	}
	if err := ValidateIdentity(to); err != nil {
		return ManifestEntry{}, err
	}
	data, err := fs.ReadFile(bundle, path.Join("agents", from+".md"))
	if err != nil {
		return ManifestEntry{}, fmt.Errorf("read bundled agent %q: %w", from, err)
	}
	spec, err := agent.Parse(data, from+".md")
	if err != nil {
		return ManifestEntry{}, err
	}
	spec.ID = to
	files := map[string][]byte{}
	if spec.ConstitutionPath != "" {
		constitution, err := fs.ReadFile(bundle, filepath.ToSlash(spec.ConstitutionPath))
		if err != nil {
			return ManifestEntry{}, fmt.Errorf("materialize bundled constitution: %w", err)
		}
		spec.ConstitutionPath = path.Join("constitutions", to+".md")
		files[spec.ConstitutionPath] = constitution
	}
	data, err = agent.Render(spec)
	if err != nil {
		return ManifestEntry{}, err
	}
	files[to+".md"] = data
	bindings := make([]SkillBinding, 0, len(spec.AllowedSkills))
	for _, name := range spec.AllowedSkills {
		bindings = append(bindings, SkillBinding{Name: name, Origin: "bundled"})
	}
	return s.installPackage(ctx, to+".md", files, data, InstallLocalAgentRequest{
		As:            to,
		DryRun:        dryRun,
		SourceURI:     "bundle:" + from,
		Provenance:    Provenance{CopiedFrom: from, BundleDigest: bundleDigest},
		SkillBindings: bindings,
	})
}

// UpdateManagedAgentSkills adds or removes one explicit managed-agent skill
// binding. Availability is validated by the caller against the effective
// catalog before an addition.
func (s *LocalAgentService) UpdateManagedAgentSkills(ctx context.Context, agentID, skillName string, add, dryRun bool) (ManifestEntry, error) {
	return s.UpdateManagedAgentSkillBinding(ctx, agentID, skillName, "", add, dryRun)
}

func (s *LocalAgentService) UpdateManagedAgentSkillBinding(ctx context.Context, agentID, skillName, origin string, add, dryRun bool) (ManifestEntry, error) {
	if err := ValidateIdentity(skillName); err != nil {
		return ManifestEntry{}, fmt.Errorf("invalid skill binding: %w", err)
	}
	return s.updateManagedAgent(ctx, agentID, dryRun, func(spec *agent.Spec, entry *ManifestEntry) error {
		index := -1
		for i, current := range spec.AllowedSkills {
			if strings.EqualFold(current, skillName) {
				index = i
				break
			}
		}
		if add && index < 0 {
			spec.AllowedSkills = append(spec.AllowedSkills, skillName)
			sort.Strings(spec.AllowedSkills)
			entry.SkillBindings = append(entry.SkillBindings, SkillBinding{Name: skillName, Origin: origin})
		}
		if !add && index >= 0 {
			spec.AllowedSkills = append(spec.AllowedSkills[:index], spec.AllowedSkills[index+1:]...)
			filtered := entry.SkillBindings[:0]
			for _, binding := range entry.SkillBindings {
				if !strings.EqualFold(binding.Name, skillName) {
					filtered = append(filtered, binding)
				}
			}
			entry.SkillBindings = filtered
		}
		return nil
	})
}

// SetManagedAgentRuntime updates the normalized model and its selected runtime
// target together so execution drift can be detected.
func (s *LocalAgentService) SetManagedAgentRuntime(ctx context.Context, agentID string, target RuntimeTarget, dryRun bool) (ManifestEntry, error) {
	if err := target.Validate(); err != nil {
		return ManifestEntry{}, err
	}
	return s.updateManagedAgent(ctx, agentID, dryRun, func(spec *agent.Spec, entry *ManifestEntry) error {
		spec.Model = target.Model
		entry.Runtime = &target
		return nil
	})
}

func (s *LocalAgentService) updateManagedAgent(ctx context.Context, identity string, dryRun bool, mutate func(*agent.Spec, *ManifestEntry) error) (ManifestEntry, error) {
	if err := ValidateIdentity(identity); err != nil {
		return ManifestEntry{}, err
	}
	if dryRun {
		manifest, _, err := s.store.RecoverAndLoadManifest(ctx)
		if err != nil {
			return ManifestEntry{}, err
		}
		entry, ok := findManifestEntry(manifest, "agent", identity)
		if !ok {
			return ManifestEntry{}, fmt.Errorf("managed agent %q not found", identity)
		}
		return s.mutateAgentObject(entry, mutate, false)
	}
	tx, err := s.store.BeginTransaction(ctx)
	if err != nil {
		return ManifestEntry{}, err
	}
	entry, ok := findManifestEntry(tx.working, "agent", identity)
	if !ok {
		_ = tx.Rollback()
		return ManifestEntry{}, fmt.Errorf("managed agent %q not found", identity)
	}
	entry, err = s.mutateAgentObject(entry, mutate, true)
	if err != nil {
		_ = tx.Rollback()
		return ManifestEntry{}, err
	}
	tx.UpsertEntry(entry)
	if err := tx.Commit(); err != nil {
		return ManifestEntry{}, err
	}
	return entry, nil
}

func (s *LocalAgentService) mutateAgentObject(entry ManifestEntry, mutate func(*agent.Spec, *ManifestEntry) error, publish bool) (ManifestEntry, error) {
	info, err := os.Lstat(entry.ObjectPath)
	if err != nil {
		return ManifestEntry{}, err
	}
	if !info.IsDir() {
		data, err := os.ReadFile(entry.ObjectPath)
		if err != nil {
			return ManifestEntry{}, err
		}
		spec, err := agent.ParseManaged(data, entry.Identity+".md")
		if err != nil {
			return ManifestEntry{}, err
		}
		if err := mutate(spec, &entry); err != nil {
			return ManifestEntry{}, err
		}
		data, err = agent.Render(spec)
		if err != nil {
			return ManifestEntry{}, err
		}
		sum := sha256.Sum256(data)
		entry.Digest = hex.EncodeToString(sum[:])
		entry.ObjectPath = filepath.Join(s.store.ObjectsDir(), entry.Digest)
		if publish {
			entry.Digest, entry.ObjectPath, err = s.store.PutObject(data)
		}
		return entry, err
	}
	files, err := readAgentPackage(entry.ObjectPath)
	if err != nil {
		return ManifestEntry{}, err
	}
	mainName := entry.Identity + ".md"
	data, ok := files[mainName]
	if !ok {
		return ManifestEntry{}, fmt.Errorf("managed agent package missing %s", mainName)
	}
	spec, err := agent.ParseManaged(data, mainName)
	if err != nil {
		return ManifestEntry{}, err
	}
	if err := mutate(spec, &entry); err != nil {
		return ManifestEntry{}, err
	}
	files[mainName], err = agent.Render(spec)
	if err != nil {
		return ManifestEntry{}, err
	}
	entry.Digest = digestPackageFiles(files)
	entry.ObjectPath = filepath.Join(s.store.ObjectsDir(), entry.Digest)
	if publish {
		entry.Digest, entry.ObjectPath, err = s.store.putDirectoryContents(files)
	}
	return entry, err
}

func (s *LocalAgentService) installDirectory(ctx context.Context, source string, req InstallLocalAgentRequest) (ManifestEntry, error) {
	files, err := readAgentPackage(source)
	if err != nil {
		return ManifestEntry{}, err
	}
	mainName := filepath.Base(source) + ".md"
	data, ok := files[mainName]
	if !ok {
		return ManifestEntry{}, fmt.Errorf("agent package directory missing %s", mainName)
	}
	return s.installPackage(ctx, mainName, files, data, req)
}

func (s *LocalAgentService) installPackage(ctx context.Context, mainName string, files map[string][]byte, data []byte, req InstallLocalAgentRequest) (ManifestEntry, error) {
	spec, err := agent.ParseManaged(data, "")
	if err != nil {
		return ManifestEntry{}, fmt.Errorf("validate agent source: %w", err)
	}
	identity := spec.ID
	if req.As != "" {
		identity = req.As
	}
	if err := ValidateIdentity(identity); err != nil {
		return ManifestEntry{}, err
	}
	if spec.ID != identity {
		var changed bool
		data, changed = replaceAgentID(data, identity)
		if !changed {
			return ManifestEntry{}, fmt.Errorf("agent source does not contain an id field")
		}
	}
	spec, err = agent.ParseManaged(data, identity+".md")
	if err != nil {
		return ManifestEntry{}, fmt.Errorf("validate managed agent: %w", err)
	}
	if spec.ConstitutionPath != "" {
		constitutionPath := filepath.ToSlash(filepath.Clean(spec.ConstitutionPath))
		if !safePackagePath(constitutionPath) {
			return ManifestEntry{}, fmt.Errorf("agent package has unsafe constitution path %q", spec.ConstitutionPath)
		}
		if _, ok := files[constitutionPath]; !ok {
			return ManifestEntry{}, fmt.Errorf("agent package is missing constitution path %q", spec.ConstitutionPath)
		}
	}
	delete(files, mainName)
	files[identity+".md"] = data
	sourceURI := req.SourceURI
	if sourceURI == "" {
		sourceURI = "local"
	}
	entry := ManifestEntry{Identity: identity, Kind: "agent", Source: sourceURI, Revision: req.Revision, Subpath: req.Subpath, Digest: digestPackageFiles(files), Runtime: req.Runtime, Provenance: req.Provenance, SkillBindings: append([]SkillBinding{}, req.SkillBindings...)}
	entry.ObjectPath = filepath.Join(s.store.ObjectsDir(), entry.Digest)
	if req.DryRun {
		return entry, nil
	}
	tx, err := s.store.BeginTransaction(ctx)
	if err != nil {
		return ManifestEntry{}, err
	}
	if _, exists := findManifestEntry(tx.working, "agent", identity); exists && !req.Replace {
		_ = tx.Rollback()
		return ManifestEntry{}, fmt.Errorf("agent %q already exists; pass --replace", identity)
	}
	digest, objectPath, err := s.store.putDirectoryContents(files)
	if err != nil {
		_ = tx.Rollback()
		return ManifestEntry{}, err
	}
	entry.Digest, entry.ObjectPath = digest, objectPath
	tx.UpsertEntry(entry)
	if err := tx.Commit(); err != nil {
		return ManifestEntry{}, err
	}
	return entry, nil
}

func (s *LocalAgentService) rewriteAgentObject(entry ManifestEntry, identity string) (ManifestEntry, error) {
	info, err := os.Lstat(entry.ObjectPath)
	if err != nil {
		return ManifestEntry{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return ManifestEntry{}, fmt.Errorf("managed agent object must not be a symlink")
	}
	if !info.IsDir() {
		data, err := os.ReadFile(entry.ObjectPath)
		if err != nil {
			return ManifestEntry{}, err
		}
		data, replaced := replaceAgentID(data, identity)
		if !replaced {
			return ManifestEntry{}, fmt.Errorf("managed agent %q does not contain an id field", entry.Identity)
		}
		if _, err := agent.ParseManaged(data, identity+".md"); err != nil {
			return ManifestEntry{}, err
		}
		digest, objectPath, err := s.store.PutObject(data)
		if err != nil {
			return ManifestEntry{}, err
		}
		entry.Identity, entry.Digest, entry.ObjectPath = identity, digest, objectPath
		return entry, nil
	}
	files, err := readAgentPackage(entry.ObjectPath)
	if err != nil {
		return ManifestEntry{}, err
	}
	mainName := entry.Identity + ".md"
	if _, ok := files[mainName]; !ok {
		return ManifestEntry{}, fmt.Errorf("managed agent package has no definition for %q", entry.Identity)
	}
	data, replaced := replaceAgentID(files[mainName], identity)
	if !replaced {
		return ManifestEntry{}, fmt.Errorf("managed agent %q does not contain an id field", entry.Identity)
	}
	if _, err := agent.ParseManaged(data, identity+".md"); err != nil {
		return ManifestEntry{}, err
	}
	delete(files, mainName)
	files[identity+".md"] = data
	digest, objectPath, err := s.store.putDirectoryContents(files)
	if err != nil {
		return ManifestEntry{}, err
	}
	entry.Identity, entry.Digest, entry.ObjectPath = identity, digest, objectPath
	return entry, nil
}

func readAgentPackage(root string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if filePath == root || entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("agent package symlink %q is not supported", filePath)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("agent package entry %q is not a regular file", filePath)
		}
		rel, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = data
		return nil
	})
	return files, err
}

func safePackagePath(value string) bool {
	value = filepath.ToSlash(strings.TrimSpace(value))
	clean := path.Clean(value)
	return value != "" && !path.IsAbs(value) && clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}

func digestPackageFiles(files map[string][]byte) string {
	hash := sha256.New()
	paths := make([]string, 0, len(files))
	for name := range files {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	for _, name := range paths {
		writeObjectFrame(hash, []byte(name))
		writeObjectFrame(hash, files[name])
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (s *Store) putDirectoryContents(files map[string][]byte) (string, string, error) {
	digest := digestPackageFiles(files)
	objectPath := filepath.Join(s.ObjectsDir(), digest)
	if info, err := os.Lstat(objectPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", "", fmt.Errorf("object path %q is not a directory", objectPath)
		}
		existing, err := directoryDigest(objectPath)
		if err != nil || existing != digest {
			return "", "", fmt.Errorf("object path %q is corrupt", objectPath)
		}
		return digest, objectPath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", "", err
	}
	if err := os.MkdirAll(s.ObjectsDir(), 0o755); err != nil {
		return "", "", err
	}
	staged, err := os.MkdirTemp(s.ObjectsDir(), ".staged-agent-*")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(staged)
	for name, data := range files {
		if !safePackagePath(name) {
			return "", "", fmt.Errorf("agent package path %q escapes package", name)
		}
		destination := filepath.Join(staged, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return "", "", err
		}
		if err := writeFileAtomically(destination, data); err != nil {
			return "", "", err
		}
	}
	if err := os.Rename(staged, objectPath); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return "", "", err
		}
		existing, verifyErr := directoryDigest(objectPath)
		if verifyErr != nil || existing != digest {
			return "", "", fmt.Errorf("concurrent agent object %q is incomplete or corrupt", objectPath)
		}
	}
	return digest, objectPath, nil
}
