package extensions

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing/fstest"

	"github.com/bryanbarton525/prism/internal/agent"
)

// MaterializeRuntimeBundle overlays active managed extension content onto the
// immutable bundled runtime so agent/skill registries can execute managed items.
func MaterializeRuntimeBundle(base fs.FS, snapshot CatalogSnapshot) (fs.FS, error) {
	overlay := fstest.MapFS{}
	if err := fs.WalkDir(base, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != "." {
				overlay[path] = &fstest.MapFile{Mode: fs.ModeDir | 0o755}
			}
			return nil
		}
		info, statErr := fs.Stat(base, path)
		if statErr != nil {
			return statErr
		}
		if info.IsDir() {
			return nil
		}
		data, readErr := fs.ReadFile(base, path)
		if readErr != nil {
			return readErr
		}
		overlay[path] = &fstest.MapFile{Data: append([]byte{}, data...), Mode: 0o644}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("materializing bundled runtime: %w", err)
	}
	for _, requiredDir := range []string{"agents", "skills", "constitutions"} {
		if _, ok := overlay[requiredDir]; !ok {
			overlay[requiredDir] = &fstest.MapFile{Mode: fs.ModeDir | 0o755}
		}
	}

	for _, item := range snapshot.Agents {
		if item.Origin != "managed" || !item.Active {
			continue
		}
		if err := overlayManagedAgent(overlay, item); err != nil {
			return nil, err
		}
	}
	for _, item := range snapshot.Skills {
		if item.Origin != "managed" || !item.Active {
			continue
		}
		if err := overlayManagedSkill(overlay, item); err != nil {
			return nil, err
		}
	}
	return overlay, nil
}

func overlayManagedAgent(overlay fstest.MapFS, item CatalogItem) error {
	info, err := verifyManagedObject(item)
	if err != nil {
		return fmt.Errorf("managed agent %s object: %w", item.ID, err)
	}
	targetPath := filepath.ToSlash(filepath.Join("agents", item.ID+".md"))
	if !info.IsDir() {
		data, err := os.ReadFile(item.ObjectPath)
		if err != nil {
			return fmt.Errorf("reading managed agent %s: %w", item.ID, err)
		}
		overlay[targetPath] = &fstest.MapFile{Data: data, Mode: 0o644}
		return nil
	}
	agentFile := filepath.Join(item.ObjectPath, item.ID+".md")
	data, readErr := os.ReadFile(agentFile)
	if readErr != nil {
		return fmt.Errorf("managed agent %s directory missing %s", item.ID, filepath.Base(agentFile))
	}
	spec, err := agent.ParseManaged(data, item.ID+".md")
	if err != nil {
		return fmt.Errorf("parse managed agent %s: %w", item.ID, err)
	}
	if spec.ConstitutionPath != "" {
		if !safeObjectRelativePath(spec.ConstitutionPath) {
			return fmt.Errorf("managed agent %s has unsafe constitution path %q", item.ID, spec.ConstitutionPath)
		}
		data = replaceConstitutionPath(data, path.Join("managed", item.ID, filepath.ToSlash(spec.ConstitutionPath)))
	}
	overlay[targetPath] = &fstest.MapFile{Data: data, Mode: 0o644}
	if err := copyManagedDirectory(overlay, item.ObjectPath, path.Join("managed", item.ID), item.ID+".md"); err != nil {
		return fmt.Errorf("reading managed agent support files for %s: %w", item.ID, err)
	}
	return nil
}

func overlayManagedSkill(overlay fstest.MapFS, item CatalogItem) error {
	info, err := verifyManagedObject(item)
	if err != nil {
		return fmt.Errorf("managed skill %s object: %w", item.ID, err)
	}
	skillRoot := filepath.Join("skills", item.ID)
	if !info.IsDir() {
		data, err := os.ReadFile(item.ObjectPath)
		if err != nil {
			return fmt.Errorf("reading managed skill %s: %w", item.ID, err)
		}
		overlay[filepath.ToSlash(filepath.Join(skillRoot, "SKILL.md"))] = &fstest.MapFile{Data: data, Mode: 0o644}
		return nil
	}
	return copyManagedDirectory(overlay, item.ObjectPath, filepath.ToSlash(skillRoot), "")
}

func verifyManagedObject(item CatalogItem) (os.FileInfo, error) {
	if err := ValidateIdentity(item.ID); err != nil {
		return nil, err
	}
	if len(item.Digest) != sha256.Size*2 {
		return nil, fmt.Errorf("invalid digest %q", item.Digest)
	}
	if _, err := hex.DecodeString(item.Digest); err != nil {
		return nil, fmt.Errorf("invalid digest %q: %w", item.Digest, err)
	}
	if strings.TrimSpace(item.ObjectPath) == "" {
		return nil, fmt.Errorf("object path is required")
	}
	info, err := os.Lstat(item.ObjectPath)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("object path must not be a symlink")
	}
	if item.ObjectRoot != "" {
		root, err := filepath.Abs(item.ObjectRoot)
		if err != nil {
			return nil, err
		}
		objectPath, err := filepath.Abs(item.ObjectPath)
		if err != nil {
			return nil, err
		}
		if filepath.Clean(objectPath) != filepath.Join(filepath.Clean(root), item.Digest) {
			return nil, fmt.Errorf("object path is outside the content-addressed object store")
		}
		realRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			return nil, err
		}
		realObject, err := filepath.EvalSymlinks(objectPath)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(realRoot, realObject)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("object path escapes the content-addressed object store")
		}
	}
	if info.IsDir() {
		digest, err := directoryDigest(item.ObjectPath)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(digest, item.Digest) {
			return nil, fmt.Errorf("directory digest %s does not match manifest digest %s", digest, item.Digest)
		}
		return info, nil
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("object path is not a regular file")
	}
	data, err := os.ReadFile(item.ObjectPath)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), item.Digest) {
		return nil, fmt.Errorf("object digest does not match manifest digest")
	}
	return info, nil
}

func copyManagedDirectory(overlay fstest.MapFS, source, targetRoot, skip string) error {
	return filepath.WalkDir(source, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if filePath == source || entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink %q is not supported", filePath)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("object entry %q is not a regular file", filePath)
		}
		rel, err := filepath.Rel(source, filePath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == skip {
			return nil
		}
		if !safeObjectRelativePath(rel) {
			return fmt.Errorf("object entry %q escapes its package", rel)
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		overlay[path.Join(targetRoot, rel)] = &fstest.MapFile{Data: data, Mode: 0o644}
		return nil
	})
}

func safeObjectRelativePath(value string) bool {
	value = filepath.ToSlash(strings.TrimSpace(value))
	clean := path.Clean(value)
	return value != "" && !path.IsAbs(value) && clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}

var constitutionPathLine = regexp.MustCompile(`(?m)^constitution_path:\s*.*$`)

func replaceConstitutionPath(data []byte, value string) []byte {
	trimmed := bytes.TrimSpace(data)
	end := bytes.Index(trimmed[3:], []byte("\n---"))
	if end < 0 {
		return data
	}
	end += 3
	frontmatter := trimmed[:end]
	replacement := []byte("constitution_path: " + value)
	return append(constitutionPathLine.ReplaceAll(frontmatter, replacement), trimmed[end:]...)
}
