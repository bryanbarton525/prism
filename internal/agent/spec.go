// Package agent handles loading, parsing, and validating Prism agent specifications.
package agent

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Spec represents a parsed agent specification from a Markdown+frontmatter file.
type Spec struct {
	// Frontmatter fields
	ID               string            `yaml:"id"`
	Name             string            `yaml:"name"`
	Description      string            `yaml:"description"`
	Model            string            `yaml:"model"`
	ContextBudget    int               `yaml:"context_budget"`
	Temperature      float64           `yaml:"temperature"`
	AllowedSkills    []string          `yaml:"allowed_skills"`
	LatencyBudgetMs  int               `yaml:"latency_budget_ms"`
	Tools            []string          `yaml:"tools"`
	Outputs          string            `yaml:"outputs"`
	ConstitutionPath string            `yaml:"constitution_path"`
	Models           []string          `yaml:"models"`
	TokenBudget      int               `yaml:"token_budget"`
	Metadata         map[string]string `yaml:"metadata"`

	// Body is the Markdown body after the frontmatter (or empty if constitution_path is set).
	Body string `yaml:"-"`

	// SourcePath is the absolute path to the file this spec was loaded from.
	SourcePath string `yaml:"-"`
}

// Validate checks that all required frontmatter fields are present and well-formed.
func (s *Spec) Validate() error {
	var errs []string
	if s.ID == "" {
		errs = append(errs, "missing required field: id")
	}
	if s.Name == "" {
		errs = append(errs, "missing required field: name")
	}
	if s.Description == "" {
		errs = append(errs, "missing required field: description")
	}
	if s.Model == "" {
		errs = append(errs, "missing required field: model")
	}
	if s.ContextBudget == 0 {
		errs = append(errs, "missing required field: context_budget")
	}
	if len(s.AllowedSkills) == 0 {
		errs = append(errs, "missing required field: allowed_skills (must list at least one skill)")
	}
	if s.LatencyBudgetMs == 0 {
		errs = append(errs, "missing required field: latency_budget_ms")
	}
	if len(errs) > 0 {
		return fmt.Errorf("agent spec %q validation failed:\n  %s", s.ID, strings.Join(errs, "\n  "))
	}
	return nil
}

// ParseFile loads an agent spec from a Markdown+YAML-frontmatter file.
func ParseFile(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading agent spec %q: %w", path, err)
	}
	return Parse(data, path)
}

// Parse parses agent spec content (Markdown with YAML frontmatter).
// path is used only for error messages and to populate SourcePath.
func Parse(data []byte, path string) (*Spec, error) {
	frontmatter, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, fmt.Errorf("parsing agent spec %q: %w", path, err)
	}

	var spec Spec
	if err := yaml.Unmarshal(frontmatter, &spec); err != nil {
		return nil, fmt.Errorf("decoding frontmatter in %q: %w", path, err)
	}

	spec.Body = strings.TrimSpace(string(body))
	spec.SourcePath = path

	// Default ID from filename stem if not set in frontmatter.
	if spec.ID == "" && path != "" {
		base := filepath.Base(path)
		spec.ID = strings.TrimSuffix(base, filepath.Ext(base))
	}

	return &spec, nil
}

// splitFrontmatter splits a Markdown file into YAML frontmatter and body.
// The file must begin with "---\n" and have a closing "---\n" delimiter.
func splitFrontmatter(data []byte) (frontmatter, body []byte, err error) {
	const delim = "---"

	// Must start with the opening delimiter.
	if !bytes.HasPrefix(data, []byte(delim+"\n")) {
		return nil, nil, fmt.Errorf("file does not begin with YAML frontmatter delimiter (---)")
	}

	// Find the closing delimiter.
	rest := data[len(delim)+1:] // skip past the opening "---\n"
	idx := bytes.Index(rest, []byte("\n"+delim))
	if idx == -1 {
		return nil, nil, fmt.Errorf("missing closing frontmatter delimiter (---)")
	}

	frontmatter = rest[:idx]
	// Skip past "\n---" and the optional trailing newline.
	after := rest[idx+1+len(delim):]
	if len(after) > 0 && after[0] == '\n' {
		after = after[1:]
	}
	body = after
	return frontmatter, body, nil
}
