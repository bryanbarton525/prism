// Package skill loads and validates Prism Agent Skills.
package skill

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill holds the parsed SKILL.md frontmatter and body for one Agent Skill.
type Skill struct {
	// Required frontmatter fields per agentskills.io spec.
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Compatibility string          `yaml:"compatibility"`
	Metadata    map[string]string `yaml:"metadata"`

	// Populated after parsing.
	Body string `yaml:"-"`
	// Dir is the skill directory path, available for reference/script loading.
	Dir string `yaml:"-"`
}

// ParseFile reads a SKILL.md file and parses it.
func ParseFile(skillMDPath string) (*Skill, error) {
	data, err := os.ReadFile(skillMDPath)
	if err != nil {
		return nil, fmt.Errorf("reading skill file %s: %w", skillMDPath, err)
	}
	sk, err := parse(data, skillMDPath)
	if err != nil {
		return nil, err
	}
	sk.Dir = filepath.Dir(skillMDPath)
	return sk, nil
}

// LoadDir resolves a skill by name from the skills root directory.
// It expects a subdirectory named after the skill containing SKILL.md.
func LoadDir(skillsRoot, name string) (*Skill, error) {
	path := filepath.Join(skillsRoot, name, "SKILL.md")
	return ParseFile(path)
}

func parse(data []byte, sourcePath string) (*Skill, error) {
	const delim = "---"

	content := strings.TrimSpace(string(data))
	if !strings.HasPrefix(content, delim) {
		return nil, fmt.Errorf("%s: missing frontmatter delimiter", sourcePath)
	}

	rest := strings.TrimPrefix(content, delim)
	idx := strings.Index(rest, "\n"+delim)
	if idx < 0 {
		return nil, fmt.Errorf("%s: unclosed frontmatter block", sourcePath)
	}

	frontmatter := strings.TrimSpace(rest[:idx])
	body := strings.TrimSpace(rest[idx+len("\n"+delim):])

	sk := &Skill{}
	dec := yaml.NewDecoder(bytes.NewBufferString(frontmatter))
	dec.KnownFields(false)
	if err := dec.Decode(sk); err != nil {
		return nil, fmt.Errorf("%s: YAML parse error: %w", sourcePath, err)
	}
	sk.Body = body

	// Validate required fields per agentskills.io spec.
	var missing []string
	if sk.Name == "" {
		missing = append(missing, "name")
	}
	if sk.Description == "" {
		missing = append(missing, "description")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("%s: missing required frontmatter fields: %s",
			sourcePath, strings.Join(missing, ", "))
	}

	// Validate that the name matches the directory name.
	dirName := filepath.Base(filepath.Dir(sourcePath))
	if dirName != "." && dirName != sk.Name {
		return nil, fmt.Errorf("%s: skill name %q does not match directory name %q",
			sourcePath, sk.Name, dirName)
	}

	return sk, nil
}

// ValidateSkillsDir checks that each name in names has a loadable SKILL.md
// under skillsRoot. It returns a map of loaded skills on success.
func ValidateSkillsDir(skillsRoot string, names []string) (map[string]*Skill, error) {
	out := make(map[string]*Skill, len(names))
	for _, name := range names {
		sk, err := LoadDir(skillsRoot, name)
		if err != nil {
			return nil, fmt.Errorf("skill %q: %w", name, err)
		}
		out[name] = sk
	}
	return out, nil
}

// ReferenceContent reads the references/REFERENCE.md for a skill if present.
func (s *Skill) ReferenceContent() (string, error) {
	path := filepath.Join(s.Dir, "references", "REFERENCE.md")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading skill reference %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// CollectScriptPath returns the path to scripts/collect.sh for a skill.
// It does not guarantee the file exists.
func (s *Skill) CollectScriptPath() string {
	return filepath.Join(s.Dir, "scripts", "collect.sh")
}

// FullText returns the combined SKILL.md content including frontmatter header
// as a string suitable for injection into a prompt.
func (s *Skill) FullText() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## Skill: %s\n\n", s.Name))
	b.WriteString(fmt.Sprintf("**Description:** %s\n\n", s.Description))
	if s.Compatibility != "" {
		b.WriteString(fmt.Sprintf("**Compatibility:** %s\n\n", s.Compatibility))
	}
	b.WriteString(s.Body)
	return b.String()
}
