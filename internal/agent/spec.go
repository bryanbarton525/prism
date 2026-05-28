// Package agent loads and validates Prism agent specifications.
package agent

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Spec holds the parsed YAML frontmatter and Markdown body of an agent file.
type Spec struct {
	// Frontmatter fields (required)
	ID               string   `yaml:"id"`
	Name             string   `yaml:"name"`
	Description      string   `yaml:"description"`
	Model            string   `yaml:"model"`
	ContextBudget    int      `yaml:"context_budget"`
	AllowedSkills    []string `yaml:"allowed_skills"`
	LatencyBudgetMS  int      `yaml:"latency_budget_ms"`

	// Frontmatter fields (recommended)
	Temperature     float64  `yaml:"temperature"`
	Tools           []string `yaml:"tools"`
	Outputs         string   `yaml:"outputs"`
	ConstitutionPath string  `yaml:"constitution_path"`

	// Populated after parsing
	Body string `yaml:"-"`
}

// Summary is a lightweight view of a Spec for listing agents.
type Summary struct {
	ID          string
	Name        string
	Description string
	Model       string
}

// ToSummary converts a Spec to a Summary.
func (s *Spec) ToSummary() Summary {
	return Summary{
		ID:          s.ID,
		Name:        s.Name,
		Description: s.Description,
		Model:       s.Model,
	}
}

// AllowsSkill reports whether skill is in the agent's allowed_skills list.
func (s *Spec) AllowsSkill(skill string) bool {
	for _, a := range s.AllowedSkills {
		if a == skill {
			return true
		}
	}
	return false
}

// ParseFile reads a Markdown+frontmatter agent spec file.
func ParseFile(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading agent spec %s: %w", path, err)
	}
	return Parse(data, path)
}

// Parse parses raw bytes that contain YAML frontmatter delimited by "---".
func Parse(data []byte, sourcePath string) (*Spec, error) {
	const delim = "---"

	content := strings.TrimSpace(string(data))
	if !strings.HasPrefix(content, delim) {
		return nil, fmt.Errorf("%s: missing frontmatter delimiter", sourcePath)
	}

	// Strip the leading ---
	rest := strings.TrimPrefix(content, delim)
	// Find the closing ---
	idx := strings.Index(rest, "\n"+delim)
	if idx < 0 {
		return nil, fmt.Errorf("%s: unclosed frontmatter block", sourcePath)
	}

	frontmatter := strings.TrimSpace(rest[:idx])
	body := strings.TrimSpace(rest[idx+len("\n"+delim):])

	spec := &Spec{}
	dec := yaml.NewDecoder(bytes.NewBufferString(frontmatter))
	dec.KnownFields(false)
	if err := dec.Decode(spec); err != nil {
		return nil, fmt.Errorf("%s: YAML parse error: %w", sourcePath, err)
	}

	spec.Body = body

	if err := validate(spec, sourcePath); err != nil {
		return nil, err
	}

	return spec, nil
}

func validate(s *Spec, src string) error {
	var missing []string
	if s.ID == "" {
		missing = append(missing, "id")
	}
	if s.Name == "" {
		missing = append(missing, "name")
	}
	if s.Description == "" {
		missing = append(missing, "description")
	}
	if s.Model == "" {
		missing = append(missing, "model")
	}
	if s.ContextBudget == 0 {
		missing = append(missing, "context_budget")
	}
	if len(s.AllowedSkills) == 0 {
		missing = append(missing, "allowed_skills")
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s: missing required frontmatter fields: %s", src, strings.Join(missing, ", "))
	}

	// Validate that the id matches the filename stem if a path was supplied.
	if src != "" {
		stem := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
		if stem != s.ID && stem != "." {
			return fmt.Errorf("%s: id %q does not match filename stem %q", src, s.ID, stem)
		}
	}

	return nil
}
