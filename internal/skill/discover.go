package skill

import (
	"errors"
	"fmt"
	"io/fs"
)

// ValidateStructure ensures a skill directory has SKILL.md, references/, and scripts/.
func ValidateStructure(fsys fs.FS, name string) error {
	var missing []string
	for _, sub := range []string{"SKILL.md", "references", "scripts"} {
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
		if err := ValidateStructure(fsys, name); err != nil {
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
		return skills, fmt.Errorf("skill discovery: %s", joinStrings(errs))
	}
	return skills, nil
}
