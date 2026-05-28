package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadSkillFile loads and validates a single SKILL.md file. The file must have
// YAML frontmatter with at least the "name" and "description" fields. The name
// must match the parent directory name.
func LoadSkillFile(path string) (*SkillSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading skill file %q: %w", path, err)
	}

	sk, err := parseSkillFile(data)
	if err != nil {
		return nil, fmt.Errorf("parsing skill file %q: %w", path, err)
	}

	dirName := filepath.Base(filepath.Dir(path))
	if err := validateSkillSpec(sk, dirName); err != nil {
		return nil, fmt.Errorf("invalid skill spec %q: %w", path, err)
	}

	sk.DirPath = filepath.Dir(path)
	return sk, nil
}

// parseSkillFile splits frontmatter and body, then unmarshals the frontmatter.
func parseSkillFile(data []byte) (*SkillSpec, error) {
	fm, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}

	var sk SkillSpec
	if err := yaml.Unmarshal(fm, &sk); err != nil {
		return nil, fmt.Errorf("unmarshalling skill frontmatter: %w", err)
	}
	sk.Body = body
	return &sk, nil
}

// validateSkillSpec enforces the required Agent Skills frontmatter fields and
// the directory-name == skill-name invariant.
func validateSkillSpec(sk *SkillSpec, dirName string) error {
	var errs []string

	if sk.Name == "" {
		errs = append(errs, "missing required field: name")
	}
	if sk.Description == "" {
		errs = append(errs, "missing required field: description")
	}
	if len(sk.Description) > 1024 {
		errs = append(errs, fmt.Sprintf("description exceeds 1024 characters (%d)", len(sk.Description)))
	}

	// name must match the containing directory name.
	if sk.Name != "" && dirName != "" && sk.Name != dirName {
		errs = append(errs, fmt.Sprintf("skill name %q does not match directory name %q", sk.Name, dirName))
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// SkillRegistry holds loaded skills indexed by name.
type SkillRegistry struct {
	skills map[string]*SkillSpec
}

// NewSkillRegistry builds a SkillRegistry from a slice of skill specs.
func NewSkillRegistry(skills []*SkillSpec) *SkillRegistry {
	r := &SkillRegistry{skills: make(map[string]*SkillSpec, len(skills))}
	for _, sk := range skills {
		r.skills[sk.Name] = sk
	}
	return r
}

// Get returns the SkillSpec for the given name, or (nil, false) if not found.
func (r *SkillRegistry) Get(name string) (*SkillSpec, bool) {
	sk, ok := r.skills[name]
	return sk, ok
}

// Names returns the sorted list of all registered skill names.
func (r *SkillRegistry) Names() []string {
	names := make([]string, 0, len(r.skills))
	for n := range r.skills {
		names = append(names, n)
	}
	sortStrings(names)
	return names
}

// LoadSkillsFrom loads all skill SKILL.md files from the given directory and
// returns a SkillRegistry.
func LoadSkillsFrom(skillDir string) (*SkillRegistry, error) {
	entries, err := os.ReadDir(skillDir)
	if err != nil {
		if os.IsNotExist(err) {
			return NewSkillRegistry(nil), nil
		}
		return nil, fmt.Errorf("reading skill dir %q: %w", skillDir, err)
	}

	var skills []*SkillSpec
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(skillDir, e.Name(), "SKILL.md")
		sk, err := LoadSkillFile(path)
		if err != nil {
			return nil, err
		}
		skills = append(skills, sk)
	}
	return NewSkillRegistry(skills), nil
}

// sortStrings sorts a slice of strings in place (insertion sort, small N).
func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}
