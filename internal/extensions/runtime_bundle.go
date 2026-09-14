package extensions

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing/fstest"
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
	info, err := os.Stat(item.ObjectPath)
	if err != nil {
		return fmt.Errorf("managed agent %s object path: %w", item.ID, err)
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
	if _, err := os.Stat(agentFile); err == nil {
		data, readErr := os.ReadFile(agentFile)
		if readErr != nil {
			return fmt.Errorf("reading managed agent %s: %w", item.ID, readErr)
		}
		overlay[targetPath] = &fstest.MapFile{Data: data, Mode: 0o644}
	} else {
		return fmt.Errorf("managed agent %s directory missing %s", item.ID, filepath.Base(agentFile))
	}
	constitutionsRoot := filepath.Join(item.ObjectPath, "constitutions")
	if _, err := os.Stat(constitutionsRoot); err == nil {
		if err := filepath.WalkDir(constitutionsRoot, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			rel, relErr := filepath.Rel(constitutionsRoot, path)
			if relErr != nil {
				return relErr
			}
			overlay[filepath.ToSlash(filepath.Join("constitutions", rel))] = &fstest.MapFile{Data: data, Mode: 0o644}
			return nil
		}); err != nil {
			return fmt.Errorf("reading managed constitutions for %s: %w", item.ID, err)
		}
	}
	return nil
}

func overlayManagedSkill(overlay fstest.MapFS, item CatalogItem) error {
	info, err := os.Stat(item.ObjectPath)
	if err != nil {
		return fmt.Errorf("managed skill %s object path: %w", item.ID, err)
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
	return filepath.WalkDir(item.ObjectPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(item.ObjectPath, path)
		if err != nil {
			return err
		}
		clean := filepath.ToSlash(filepath.Join(skillRoot, rel))
		if strings.Contains(clean, "..") {
			return fmt.Errorf("managed skill %s has invalid path %s", item.ID, rel)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		overlay[clean] = &fstest.MapFile{Data: data, Mode: 0o644}
		return nil
	})
}
