// Package skill loads and validates Prism Agent Skills from SKILL.md files.
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
	Name        string `yaml:"name"`
	Description string `yaml:"description"`

	// Optional frontmatter fields Prism honours when present.
	Compatibility string            `yaml:"compatibility"`
	Metadata      map[string]string `yaml:"metadata"`

	// Body is the Markdown body of SKILL.md after the frontmatter block.
	Body string `yaml:"-"`
	// Dir is the skill directory path (useful for loading references/scripts).
	Dir string `yaml:"-"`
}

// ParseFile reads a SKILL.md file and returns a Skill.
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
// It expects skills/<name>/SKILL.md to exist.
func LoadDir(skillsRoot, name string) (*Skill, error) {
	return ParseFile(filepath.Join(skillsRoot, name, "SKILL.md"))
}

// LoadMany loads all named skills from skillsRoot and returns a map keyed by
// skill name. All errors are collected so callers see every missing skill at once.
func LoadMany(skillsRoot string, names []string) (map[string]*Skill, error) {
	out := make(map[string]*Skill, len(names))
	var errs []string
	for _, name := range names {
		sk, err := LoadDir(skillsRoot, name)
		if err != nil {
			errs = append(errs, fmt.Sprintf("skill %q: %s", name, err))
			continue
		}
		out[name] = sk
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("loading skills: %s", strings.Join(errs, "; "))
	}
	return out, nil
}

// FullText returns the skill content formatted for prompt injection.
// It includes a Markdown heading, the description, optional compatibility
// note, and the full skill body.
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

// MetadataSummary returns a one-line summary for the skills index section
// ("progressive disclosure metadata phase").
func (s *Skill) MetadataSummary() string {
	return fmt.Sprintf("- **%s**: %s", s.Name, s.Description)
}

// ReferenceContent reads references/REFERENCE.md for the skill if present.
// Returns an empty string without error when the file does not exist.
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

// CollectScriptPath returns the expected path to scripts/collect.sh.
// It does not guarantee the file exists.
func (s *Skill) CollectScriptPath() string {
	return filepath.Join(s.Dir, "scripts", "collect.sh")
}

// ---------------------------------------------------------------------------
// Internal parsing
// ---------------------------------------------------------------------------

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
	if len(sk.Description) > 1024 {
		return nil, fmt.Errorf("%s: description exceeds 1024 characters (%d)",
			sourcePath, len(sk.Description))
	}

	dirName := filepath.Base(filepath.Dir(sourcePath))
	if dirName != "." && dirName != sk.Name {
		return nil, fmt.Errorf("%s: skill name %q does not match directory name %q",
			sourcePath, sk.Name, dirName)
	}

	return sk, nil
}
