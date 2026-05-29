// Package agent handles loading and validating agent specifications.
package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Spec is the parsed representation of an agents/*.md file.
type Spec struct {
	// Frontmatter fields
	ID              string   `yaml:"id"`
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description"`
	Model           string   `yaml:"model"`
	ContextBudget   int      `yaml:"context_budget"`
	Temperature     float64  `yaml:"temperature"`
	AllowedSkills   []string `yaml:"allowed_skills"`
	LatencyBudgetMs int      `yaml:"latency_budget_ms"`
	Tools           []string `yaml:"tools"`
	Outputs         string   `yaml:"outputs"`
	ConstitutionPath string  `yaml:"constitution_path"`

	// Body is the Markdown text after the YAML frontmatter delimiter.
	Body string `yaml:"-"`
}

// parseFrontmatter splits a Markdown document into its YAML frontmatter and body.
// The document must begin with "---\n". Returns an error if the closing
// delimiter is not found.
func parseFrontmatter(content string) (frontmatter, body string, err error) {
	content = strings.TrimPrefix(content, "\xef\xbb\xbf") // strip UTF-8 BOM
	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r\n") {
		return "", content, nil
	}
	// skip opening "---"
	rest := content[4:]
	idx := strings.Index(rest, "\n---")
	if idx == -1 {
		return "", "", fmt.Errorf("missing closing frontmatter delimiter '---'")
	}
	frontmatter = rest[:idx]
	body = strings.TrimPrefix(rest[idx+4:], "\n")
	body = strings.TrimPrefix(body, "\r\n")
	return frontmatter, body, nil
}

// LoadSpec reads and parses a single agent Markdown file.
func LoadSpec(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading agent spec %s: %w", path, err)
	}

	fm, body, err := parseFrontmatter(string(data))
	if err != nil {
		return nil, fmt.Errorf("parsing frontmatter in %s: %w", path, err)
	}

	var spec Spec
	if fm != "" {
		if err := yaml.Unmarshal([]byte(fm), &spec); err != nil {
			return nil, fmt.Errorf("decoding YAML frontmatter in %s: %w", path, err)
		}
	}

	// Derive ID from filename when not set explicitly in frontmatter.
	if spec.ID == "" {
		base := filepath.Base(path)
		spec.ID = strings.TrimSuffix(base, filepath.Ext(base))
	}

	spec.Body = body
	return &spec, nil
}

// Validate checks that all required frontmatter fields are present.
func (s *Spec) Validate() error {
	missing := []string{}
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
	if len(missing) > 0 {
		return fmt.Errorf("agent spec %q is missing required fields: %s", s.ID, strings.Join(missing, ", "))
	}
	return nil
}
