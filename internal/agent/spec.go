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
	// Required frontmatter fields.
	ID              string   `yaml:"id"               json:"id"`
	Name            string   `yaml:"name"             json:"name"`
	Description     string   `yaml:"description"      json:"description"`
	Model           string   `yaml:"model"            json:"model"`
	ContextBudget   int      `yaml:"context_budget"   json:"context_budget"`
	AllowedSkills   []string `yaml:"allowed_skills"   json:"allowed_skills"`
	LatencyBudgetMS int      `yaml:"latency_budget_ms" json:"latency_budget_ms"`

	// Recommended frontmatter fields.
	Temperature      float64  `yaml:"temperature"       json:"temperature,omitempty"`
	Tools            []string `yaml:"tools"             json:"tools,omitempty"`
	Outputs          string   `yaml:"outputs"           json:"outputs,omitempty"`
	ConstitutionPath string   `yaml:"constitution_path" json:"constitution_path,omitempty"`

	// Optional frontmatter fields.
	Models      []string          `yaml:"models"      json:"models,omitempty"`
	TokenBudget int               `yaml:"token_budget" json:"token_budget,omitempty"`
	Metadata    map[string]string `yaml:"metadata"    json:"metadata,omitempty"`

	// Body is the Markdown text after the closing frontmatter delimiter.
	// It serves as the inline constitution when ConstitutionPath is empty.
	Body string `yaml:"-" json:"body,omitempty"`
}

// Summary is a lightweight agent descriptor suitable for list output.
type Summary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Model       string `json:"model"`
}

// ToSummary returns a lightweight view of the spec.
func (s *Spec) ToSummary() Summary {
	return Summary{
		ID:          s.ID,
		Name:        s.Name,
		Description: s.Description,
		Model:       s.Model,
	}
}

// AllowsSkill reports whether the named skill is in the agent's allowed_skills.
func (s *Spec) AllowsSkill(name string) bool {
	for _, a := range s.AllowedSkills {
		if a == name {
			return true
		}
	}
	return false
}

// ResolveConstitution returns the constitution text for this agent following
// a three-priority chain:
//
//  1. constitution_path field (resolved relative to rootDir)
//  2. inline spec body (Markdown after the frontmatter block)
//  3. legacy constitutions/<id>.md (relative to rootDir)
//
// The second return value names the source: "path", "body", or "legacy".
func (s *Spec) ResolveConstitution(rootDir string) (text, source string, err error) {
	if s.ConstitutionPath != "" {
		full := s.ConstitutionPath
		if !filepath.IsAbs(full) {
			full = filepath.Join(rootDir, full)
		}
		data, readErr := os.ReadFile(full)
		if readErr != nil {
			return "", "", fmt.Errorf("reading constitution_path %s: %w", full, readErr)
		}
		return strings.TrimSpace(string(data)), "path", nil
	}

	if s.Body != "" {
		return s.Body, "body", nil
	}

	legacyPath := filepath.Join(rootDir, "constitutions", s.ID+".md")
	data, readErr := os.ReadFile(legacyPath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return "", "none", nil
		}
		return "", "", fmt.Errorf("reading legacy constitution %s: %w", legacyPath, readErr)
	}
	return strings.TrimSpace(string(data)), "legacy", nil
}

// ParseFile reads a Markdown+frontmatter agent spec file from path.
func ParseFile(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading agent spec %s: %w", path, err)
	}
	return Parse(data, path)
}

// Parse parses raw bytes that begin with a YAML frontmatter block delimited by
// "---". The sourcePath is used only for error messages and id-stem validation.
func Parse(data []byte, sourcePath string) (*Spec, error) {
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
		return fmt.Errorf("%s: missing required frontmatter fields: %s",
			src, strings.Join(missing, ", "))
	}

	if src != "" {
		stem := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
		if stem != "." && stem != s.ID {
			return fmt.Errorf("%s: id %q does not match filename stem %q", src, s.ID, stem)
		}
	}
	return nil
}
