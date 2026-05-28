// Package skill loads and validates Prism Agent Skills from the skills/ directory.
//
// Each skill is a subdirectory under a configured root that must contain:
//   - SKILL.md with valid frontmatter (name, description)
//   - references/ subdirectory
//   - scripts/ subdirectory
//
// The name field in SKILL.md frontmatter must match the directory name.
package skill

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Frontmatter holds the parsed YAML header of a SKILL.md file.
type Frontmatter struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	License     string            `yaml:"license,omitempty"`
	Compatibility string          `yaml:"compatibility,omitempty"`
	Metadata    map[string]string `yaml:"metadata,omitempty"`
	AllowedTools string           `yaml:"allowed-tools,omitempty"`
}

// Skill is a fully loaded and validated Agent Skill.
type Skill struct {
	// DirName is the name of the directory under skills/.
	DirName string
	// Dir is the absolute path to the skill directory.
	Dir string
	// Frontmatter holds validated metadata from SKILL.md.
	Frontmatter Frontmatter
	// Body is the Markdown body of SKILL.md (after the frontmatter block).
	Body string
}

// ValidationError collects one or more validation problems found in a skill.
type ValidationError struct {
	DirName string
	Errors  []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("skill %q: %s", e.DirName, strings.Join(e.Errors, "; "))
}

// LoadAll reads every subdirectory under skillsRoot, loads SKILL.md from each,
// validates the structure and frontmatter, and returns the slice of valid skills.
// Any skill that fails validation is skipped and its error is accumulated into
// the returned error (as a joined multi-error).
func LoadAll(skillsRoot string) ([]*Skill, error) {
	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		return nil, fmt.Errorf("reading skills directory %q: %w", skillsRoot, err)
	}

	var skills []*Skill
	var errs []error

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirName := entry.Name()
		s, err := Load(filepath.Join(skillsRoot, dirName))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		skills = append(skills, s)
	}

	return skills, errors.Join(errs...)
}

// Load reads a single skill directory, validates the required structure,
// parses SKILL.md frontmatter, and returns the populated Skill.
func Load(dir string) (*Skill, error) {
	dirName := filepath.Base(dir)

	if err := validateStructure(dir, dirName); err != nil {
		return nil, err
	}

	skillMD := filepath.Join(dir, "SKILL.md")
	fm, body, err := parseFrontmatter(skillMD)
	if err != nil {
		return nil, &ValidationError{DirName: dirName, Errors: []string{err.Error()}}
	}

	if err := validateFrontmatter(fm, dirName); err != nil {
		return nil, err
	}

	return &Skill{
		DirName:     dirName,
		Dir:         dir,
		Frontmatter: fm,
		Body:        body,
	}, nil
}

// validateStructure checks that the required files and subdirectories exist.
func validateStructure(dir, dirName string) error {
	var problems []string

	required := []struct {
		path    string
		isDir   bool
		label   string
	}{
		{"SKILL.md", false, "SKILL.md"},
		{"references", true, "references/ subdirectory"},
		{"scripts", true, "scripts/ subdirectory"},
	}

	for _, r := range required {
		full := filepath.Join(dir, r.path)
		info, err := os.Stat(full)
		if err != nil {
			problems = append(problems, fmt.Sprintf("missing %s", r.label))
			continue
		}
		if r.isDir && !info.IsDir() {
			problems = append(problems, fmt.Sprintf("%s must be a directory, not a file", r.label))
		}
		if !r.isDir && info.IsDir() {
			problems = append(problems, fmt.Sprintf("%s must be a file, not a directory", r.label))
		}
	}

	if len(problems) > 0 {
		return &ValidationError{DirName: dirName, Errors: problems}
	}
	return nil
}

// parseFrontmatter reads a Markdown file and splits it into YAML frontmatter
// and the body. The file must begin with "---\n".
func parseFrontmatter(path string) (Frontmatter, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Frontmatter{}, "", fmt.Errorf("reading %s: %w", path, err)
	}

	content := string(raw)
	if !strings.HasPrefix(content, "---\n") {
		return Frontmatter{}, "", fmt.Errorf("%s does not start with YAML frontmatter delimiter (---)", filepath.Base(path))
	}

	// Find the closing ---
	rest := content[4:] // skip opening "---\n"
	end := strings.Index(rest, "\n---")
	if end == -1 {
		return Frontmatter{}, "", fmt.Errorf("%s frontmatter block is not closed with ---", filepath.Base(path))
	}

	yamlBlock := rest[:end]
	body := strings.TrimPrefix(rest[end:], "\n---\n")
	body = strings.TrimPrefix(body, "\n---")
	body = strings.TrimLeft(body, "\n")

	var fm Frontmatter
	if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err != nil {
		return Frontmatter{}, "", fmt.Errorf("parsing frontmatter in %s: %w", filepath.Base(path), err)
	}

	return fm, body, nil
}

// validateFrontmatter checks required fields and the directory-name match.
func validateFrontmatter(fm Frontmatter, dirName string) error {
	var problems []string

	if strings.TrimSpace(fm.Name) == "" {
		problems = append(problems, "frontmatter field 'name' is required")
	} else if fm.Name != dirName {
		problems = append(problems, fmt.Sprintf("frontmatter 'name' (%q) must match directory name (%q)", fm.Name, dirName))
	}

	if strings.TrimSpace(fm.Description) == "" {
		problems = append(problems, "frontmatter field 'description' is required")
	} else if len(fm.Description) > 1024 {
		problems = append(problems, fmt.Sprintf("frontmatter 'description' exceeds 1024 characters (%d)", len(fm.Description)))
	}

	if len(problems) > 0 {
		return &ValidationError{DirName: dirName, Errors: problems}
	}
	return nil
}
