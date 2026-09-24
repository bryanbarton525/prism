package extensions

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bryanbarton525/prism/internal/agent"
)

// RuntimeBatchObject is a validated immutable package waiting for one manifest
// publication. Files are relative to the content-addressed object root.
type RuntimeBatchObject struct {
	Entry          ManifestEntry
	Files          map[string][]byte
	Replace        bool
	ExpectedDigest string
}

type RuntimeBatchRequest struct {
	Objects   []RuntimeBatchObject
	MCPAccess *MCPAccessState
	DryRun    bool
}

func PlanSkillObject(fsys fs.FS, root, identity, source, revision, subpath, sourceDigest string, replace bool) (RuntimeBatchObject, error) {
	if err := ValidateIdentity(identity); err != nil {
		return RuntimeBatchObject{}, err
	}
	packageFS, err := rewrittenSkillFS(fsys, root, identity)
	if err != nil {
		return RuntimeBatchObject{}, err
	}
	files, err := snapshotPackageFS(packageFS, ".")
	if err != nil {
		return RuntimeBatchObject{}, err
	}
	digest := digestPackageFiles(files)
	return RuntimeBatchObject{
		Entry: ManifestEntry{Identity: identity, Kind: "skill", Source: source, Revision: revision, Subpath: subpath, Digest: digest, Provenance: Provenance{SourceDigest: sourceDigest}},
		Files: files, Replace: replace,
	}, nil
}

func PlanTranslatedAgentObject(item TranslatedAgentInstall) (RuntimeBatchObject, error) {
	spec, err := agent.ParseManaged(item.Normalized, "")
	if err != nil {
		return RuntimeBatchObject{}, err
	}
	identity := spec.ID
	if item.Request.As != "" {
		identity = item.Request.As
		spec.ID = identity
		item.Normalized, err = agent.Render(spec)
		if err != nil {
			return RuntimeBatchObject{}, err
		}
	}
	if err := ValidateIdentity(identity); err != nil {
		return RuntimeBatchObject{}, err
	}
	files := map[string][]byte{identity + ".md": append([]byte(nil), item.Normalized...)}
	for name, data := range item.SupportFiles {
		clean := path.Clean(strings.TrimSpace(name))
		if !fs.ValidPath(clean) || clean == "." || clean == identity+".md" || clean == "provenance" || strings.HasPrefix(clean, "provenance/") {
			return RuntimeBatchObject{}, fmt.Errorf("invalid agent support path %q", name)
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
	source := item.Request.SourceURI
	if source == "" {
		source = "local"
	}
	digest := digestPackageFiles(files)
	return RuntimeBatchObject{
		Entry: ManifestEntry{Identity: identity, Kind: "agent", Source: source, Revision: item.Request.Revision, Subpath: item.Request.Subpath, Digest: digest, Runtime: item.Request.Runtime, Provenance: item.Request.Provenance, SkillBindings: append([]SkillBinding{}, item.Request.SkillBindings...)},
		Files: files, Replace: item.Request.Replace,
	}, nil
}

func PlanBundledAgentCopy(bundle fs.FS, bundleDigest, from, to string) (RuntimeBatchObject, error) {
	return PlanBundledAgentCopyWithOptions(bundle, bundleDigest, from, to, "", nil, false)
}

// PlanBundledAgentCopyWithOptions creates an independent managed copy while
// applying the operator's final display-name and skill choices before digesting
// the package.
func PlanBundledAgentCopyWithOptions(bundle fs.FS, bundleDigest, from, to, displayName string, skills []string, overrideSkills bool) (RuntimeBatchObject, error) {
	if err := ValidateIdentity(from); err != nil {
		return RuntimeBatchObject{}, err
	}
	if err := ValidateIdentity(to); err != nil {
		return RuntimeBatchObject{}, err
	}
	data, err := fs.ReadFile(bundle, path.Join("agents", from+".md"))
	if err != nil {
		return RuntimeBatchObject{}, err
	}
	spec, err := agent.Parse(data, from+".md")
	if err != nil {
		return RuntimeBatchObject{}, err
	}
	spec.ID = to
	if strings.TrimSpace(displayName) != "" {
		spec.Name = strings.TrimSpace(displayName)
	}
	if overrideSkills {
		spec.AllowedSkills = append([]string{}, skills...)
	}
	files := map[string][]byte{}
	if spec.ConstitutionPath != "" {
		constitution, err := fs.ReadFile(bundle, filepath.ToSlash(spec.ConstitutionPath))
		if err != nil {
			return RuntimeBatchObject{}, err
		}
		spec.ConstitutionPath = path.Join("constitutions", to+".md")
		files[spec.ConstitutionPath] = constitution
	}
	data, err = agent.Render(spec)
	if err != nil {
		return RuntimeBatchObject{}, err
	}
	files[to+".md"] = data
	bindings := make([]SkillBinding, 0, len(spec.AllowedSkills))
	for _, name := range spec.AllowedSkills {
		bindings = append(bindings, SkillBinding{Name: name, Origin: "bundled"})
	}
	digest := digestPackageFiles(files)
	return RuntimeBatchObject{
		Entry: ManifestEntry{Identity: to, Kind: "agent", Source: "bundle:" + from, Digest: digest, Provenance: Provenance{CopiedFrom: from, BundleDigest: bundleDigest}, SkillBindings: bindings},
		Files: files,
	}, nil
}

// PlanExistingAgentUpdate includes an existing managed package in a runtime
// batch, retaining its support/provenance files and replacing only explicitly
// selected agent fields.
func PlanExistingAgentUpdate(stateDir, identity, displayName string, skills []string, overrideSkills bool, bindings []SkillBinding) (RuntimeBatchObject, error) {
	manifest, _, err := NewStore(stateDir).RecoverAndLoadManifest(context.Background())
	if err != nil {
		return RuntimeBatchObject{}, err
	}
	entry, ok := findManifestEntry(manifest, "agent", identity)
	if !ok {
		return RuntimeBatchObject{}, fmt.Errorf("managed agent %q is not installed", identity)
	}
	expectedDigest := entry.Digest
	mainName := entry.Identity + ".md"
	files := map[string][]byte{}
	info, err := os.Stat(entry.ObjectPath)
	if err != nil {
		return RuntimeBatchObject{}, err
	}
	if info.IsDir() {
		files, err = readAgentPackage(entry.ObjectPath)
	} else {
		var data []byte
		data, err = os.ReadFile(entry.ObjectPath)
		files[mainName] = data
	}
	if err != nil {
		return RuntimeBatchObject{}, err
	}
	data, ok := files[mainName]
	if !ok {
		return RuntimeBatchObject{}, fmt.Errorf("managed agent package has no definition for %q", entry.Identity)
	}
	spec, err := agent.ParseManaged(data, mainName)
	if err != nil {
		return RuntimeBatchObject{}, err
	}
	if strings.TrimSpace(displayName) != "" {
		spec.Name = strings.TrimSpace(displayName)
	}
	if overrideSkills {
		spec.AllowedSkills = append([]string{}, skills...)
		entry.SkillBindings = append([]SkillBinding{}, bindings...)
	}
	files[mainName], err = agent.Render(spec)
	if err != nil {
		return RuntimeBatchObject{}, err
	}
	entry.Digest = digestPackageFiles(files)
	return RuntimeBatchObject{Entry: entry, Files: files, Replace: true, ExpectedDigest: expectedDigest}, nil
}

func ActivateRuntimeBatch(ctx context.Context, stateDir string, req RuntimeBatchRequest) ([]ManifestEntry, error) {
	seen := map[string]bool{}
	entries := make([]ManifestEntry, len(req.Objects))
	for i, object := range req.Objects {
		if err := ValidateIdentity(object.Entry.Identity); err != nil {
			return nil, err
		}
		kind := normalizeKind(object.Entry.Kind)
		if kind != "agent" && kind != "skill" {
			return nil, fmt.Errorf("unsupported runtime batch kind %q", object.Entry.Kind)
		}
		key := kind + ":" + strings.ToLower(object.Entry.Identity)
		if seen[key] {
			return nil, fmt.Errorf("duplicate runtime batch object %s", key)
		}
		seen[key] = true
		if digestPackageFiles(object.Files) != object.Entry.Digest {
			return nil, fmt.Errorf("runtime batch object %s digest changed after planning", key)
		}
		entries[i] = object.Entry
	}
	if req.DryRun {
		return entries, nil
	}
	store := NewStore(stateDir)
	tx, err := store.BeginTransaction(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	currentEntries := make([]*ManifestEntry, len(req.Objects))
	for i, object := range req.Objects {
		current, exists := findManifestEntry(tx.working, object.Entry.Kind, object.Entry.Identity)
		if object.ExpectedDigest != "" && (!exists || !strings.EqualFold(current.Digest, object.ExpectedDigest)) {
			return nil, fmt.Errorf("%s %q changed after planning; retry", object.Entry.Kind, object.Entry.Identity)
		}
		if exists {
			copy := current
			currentEntries[i] = &copy
			if !object.Replace && !strings.EqualFold(current.Digest, object.Entry.Digest) {
				return nil, fmt.Errorf("%s %q already exists; pass --replace", object.Entry.Kind, object.Entry.Identity)
			}
		}
	}
	for i, object := range req.Objects {
		if currentEntries[i] != nil && strings.EqualFold(currentEntries[i].Digest, object.Entry.Digest) {
			entries[i] = *currentEntries[i]
			continue
		}
		digest, objectPath, err := store.putDirectoryContents(object.Files)
		if err != nil {
			return nil, err
		}
		entry := object.Entry
		entry.Digest, entry.ObjectPath = digest, objectPath
		tx.UpsertEntry(entry)
		entries[i] = entry
	}
	if req.MCPAccess != nil {
		if err := tx.StageMCPAccess(*req.MCPAccess); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return entries, nil
}

func snapshotPackageFS(fsys fs.FS, root string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := fs.WalkDir(fsys, root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == root || entry.IsDir() {
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("package entry %q is a symlink", name)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("package entry %q is not a regular file", name)
		}
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		rel := name
		if root != "." {
			rel = strings.TrimPrefix(name, strings.TrimSuffix(root, "/")+"/")
		}
		files[rel] = data
		return nil
	})
	return files, err
}
