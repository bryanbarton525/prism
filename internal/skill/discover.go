package skill

import (
	"fmt"
	"os"
	"path/filepath"
)

// ValidateStructure ensures a skill directory has SKILL.md, references/, and scripts/.
func ValidateStructure(dir string) error {
	dirName := filepath.Base(dir)
	var missing []string
	for _, name := range []string{"SKILL.md", "references", "scripts"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			if os.IsNotExist(err) {
				missing = append(missing, name)
				continue
			}
			return fmt.Errorf("skill %q: %s: %w", dirName, name, err)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("skill %q: missing required paths: %s", dirName, joinStrings(missing))
	}
	return nil
}

// DiscoverAll loads every skill subdirectory under skillsRoot.
// Structure and frontmatter validation errors are aggregated.
func DiscoverAll(skillsRoot string) ([]*Skill, error) {
	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		return nil, fmt.Errorf("reading skills directory %q: %w", skillsRoot, err)
	}
	var skills []*Skill
	var errs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(skillsRoot, e.Name())
		if err := ValidateStructure(dir); err != nil {
			errs = append(errs, err.Error())
			continue
		}
		sk, err := ParseFile(filepath.Join(dir, "SKILL.md"))
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

func joinStrings(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += "; " + parts[i]
	}
	return out
}
