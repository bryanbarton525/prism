package skill

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// ValidatePortableStructure validates the portable skill package format.
// Only SKILL.md is required; references/, scripts/, and evals/ are optional.
func ValidatePortableStructure(fsys fs.FS, name string) error {
	if _, err := fs.Stat(fsys, filepath.ToSlash(filepath.Join(name, "SKILL.md"))); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("skill %q: missing required path: SKILL.md", name)
		}
		return fmt.Errorf("skill %q: SKILL.md: %w", name, err)
	}
	for _, sub := range []string{"references", "scripts", "evals"} {
		path := filepath.ToSlash(filepath.Join(name, sub))
		info, err := fs.Stat(fsys, path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return fmt.Errorf("skill %q: %s: %w", name, sub, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("skill %q: optional path %s must be a directory when present", name, sub)
		}
	}
	return nil
}

// ValidateAuthoringStructure enforces strict bundled-skill authoring checks.
func ValidateAuthoringStructure(fsys fs.FS, name string) error {
	var missing []string
	for _, sub := range []string{"SKILL.md", "references", "scripts", "evals"} {
		path := name + "/" + sub
		if _, err := fs.Stat(fsys, path); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				missing = append(missing, sub)
				continue
			}
			return fmt.Errorf("skill %q: %s: %w", name, sub, err)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("skill %q: missing required paths: %s", name, joinStrings(missing))
	}
	return nil
}

// ValidateStructure is retained for compatibility and maps to portable checks.
func ValidateStructure(fsys fs.FS, name string) error {
	return ValidatePortableStructure(fsys, name)
}

// ExecutionLimitations reports runtime limitations separately from format validity.
func ExecutionLimitations(fsys fs.FS, name string) []string {
	var warnings []string
	if _, err := fs.Stat(fsys, filepath.ToSlash(filepath.Join(name, "scripts", "collect.sh"))); err == nil {
		warnings = append(warnings, "scripts/collect.sh present; script execution is not performed during installation")
	}
	if _, err := fs.Stat(fsys, filepath.ToSlash(filepath.Join(name, "evals"))); err != nil && errors.Is(err, fs.ErrNotExist) {
		warnings = append(warnings, "no evals/ directory; format valid but evaluation coverage unavailable")
	}
	if _, err := fs.Stat(fsys, filepath.ToSlash(filepath.Join(name, "references"))); err != nil && errors.Is(err, fs.ErrNotExist) {
		warnings = append(warnings, "no references/ directory; format valid with limited supplementary resources")
	}
	return warnings
}

// DiscoverAll loads every skill subdirectory under fsys (the skills root FS).
// Structure and frontmatter validation errors are aggregated.
func DiscoverAll(fsys fs.FS) ([]*Skill, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("reading skills directory: %w", err)
	}
	var skills []*Skill
	var errs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if err := ValidatePortableStructure(fsys, name); err != nil {
			errs = append(errs, err.Error())
			continue
		}
		sk, err := LoadDir(fsys, name)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		skills = append(skills, sk)
	}
	if len(errs) > 0 {
		return skills, fmt.Errorf("skill discovery: %s", strings.Join(errs, "; "))
	}
	return skills, nil
}
