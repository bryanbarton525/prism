package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SkillSpec is the parsed representation of a skills/<name>/SKILL.md file.
type SkillSpec struct {
	Name          string   `yaml:"name"`
	Description   string   `yaml:"description"`
	Compatibility string   `yaml:"compatibility"`
	License       string   `yaml:"license"`
	AllowedTools  []string `yaml:"allowed-tools"`
	Metadata      map[string]string `yaml:"metadata"`

	// Body is the instructional text after the YAML frontmatter.
	Body string `yaml:"-"`
	// Dir is the directory containing the skill.
	Dir string `yaml:"-"`
}

// LoadSkill reads and parses a single SKILL.md file.
func LoadSkill(path string) (*SkillSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading skill spec %s: %w", path, err)
	}

	fm, body, err := parseFrontmatter(string(data))
	if err != nil {
		return nil, fmt.Errorf("parsing frontmatter in %s: %w", path, err)
	}

	var skill SkillSpec
	if fm != "" {
		if err := yaml.Unmarshal([]byte(fm), &skill); err != nil {
			return nil, fmt.Errorf("decoding YAML frontmatter in %s: %w", path, err)
		}
	}

	if skill.Name == "" {
		skill.Name = filepath.Base(filepath.Dir(path))
	}

	skill.Body = body
	skill.Dir = filepath.Dir(path)
	return &skill, nil
}

// LoadSkillsFromDir reads all SKILL.md files one level deep under skillsDir.
// Returns a map keyed by skill name.
func LoadSkillsFromDir(skillsDir string) (map[string]*SkillSpec, error) {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]*SkillSpec{}, nil
		}
		return nil, fmt.Errorf("reading skills directory %s: %w", skillsDir, err)
	}

	skills := make(map[string]*SkillSpec)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillFile := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		if _, statErr := os.Stat(skillFile); os.IsNotExist(statErr) {
			continue
		}
		skill, err := LoadSkill(skillFile)
		if err != nil {
			return nil, err
		}
		key := strings.ToLower(skill.Name)
		skills[key] = skill
	}
	return skills, nil
}
