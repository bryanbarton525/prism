// Package agent loads and validates Prism agent specifications.
package agent

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
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
	// Temperature is a pointer so an explicit `temperature: 0` (deterministic
	// sampling) is distinguishable from an omitted key (engine default).
	Temperature      *float64 `yaml:"temperature"       json:"temperature,omitempty"`
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

// Render serializes a normalized Prism agent definition. Managed mutations use
// this form so identity, model, and skill bindings remain consistent with the
// immutable object bytes that are activated.
func Render(spec *Spec) ([]byte, error) {
	frontmatter, err := yaml.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("marshal agent frontmatter: %w", err)
	}
	var out bytes.Buffer
	out.WriteString("---\n")
	out.Write(frontmatter)
	out.WriteString("---\n")
	out.WriteString(strings.TrimSpace(spec.Body))
	if spec.Body != "" {
		out.WriteByte('\n')
	}
	return out.Bytes(), nil
}

// Summary is a lightweight agent descriptor suitable for list output.
type Summary struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Model           string   `json:"model"`
	AllowedSkills   []string `json:"allowed_skills,omitempty"`
	LatencyBudgetMS int      `json:"latency_budget_ms,omitempty"`
}

// ToSummary returns a lightweight view of the spec.
func (s *Spec) ToSummary() Summary {
	return Summary{
		ID:              s.ID,
		Name:            s.Name,
		Description:     s.Description,
		Model:           s.Model,
		AllowedSkills:   s.AllowedSkills,
		LatencyBudgetMS: s.LatencyBudgetMS,
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
//  1. constitution_path field (resolved relative to the FS root)
//  2. inline spec body (Markdown after the frontmatter block)
//  3. legacy constitutions/<id>.md (relative to the FS root)
//
// The second return value names the source: "path", "body", or "legacy".
// fsys should be the project root FS (not the agents sub-FS).
func (s *Spec) ResolveConstitution(fsys fs.FS) (text, source string, err error) {
	if s.ConstitutionPath != "" {
		// Reject absolute paths — they cannot be resolved via fs.FS.
		if len(s.ConstitutionPath) > 0 && s.ConstitutionPath[0] == '/' {
			return "", "", fmt.Errorf("constitution_path must be relative, got %q", s.ConstitutionPath)
		}
		data, readErr := fs.ReadFile(fsys, s.ConstitutionPath)
		if readErr != nil {
			return "", "", fmt.Errorf("reading constitution_path %s: %w", s.ConstitutionPath, readErr)
		}
		return strings.TrimSpace(string(data)), "path", nil
	}

	if s.Body != "" {
		return s.Body, "body", nil
	}

	legacyPath := "constitutions/" + s.ID + ".md"
	data, readErr := fs.ReadFile(fsys, legacyPath)
	if readErr != nil {
		if errors.Is(readErr, fs.ErrNotExist) {
			return "", "none", nil
		}
		return "", "", fmt.Errorf("reading legacy constitution %s: %w", legacyPath, readErr)
	}
	return strings.TrimSpace(string(data)), "legacy", nil
}

// Parse parses raw bytes that begin with a YAML frontmatter block delimited by
// "---". The sourcePath is used only for error messages and id-stem validation.
func Parse(data []byte, sourcePath string) (*Spec, error) {
	return parseWithOptions(data, sourcePath, false)
}

// ParseManaged parses a user-managed agent. Managed agents must declare the
// allowed_skills field, but an explicit empty list is valid.
func ParseManaged(data []byte, sourcePath string) (*Spec, error) {
	return parseWithOptions(data, sourcePath, true)
}

func parseWithOptions(data []byte, sourcePath string, allowEmptySkills bool) (*Spec, error) {
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
	if allowEmptySkills {
		if spec.ContextBudget == 0 {
			spec.ContextBudget = 8192
		}
		if spec.LatencyBudgetMS == 0 {
			spec.LatencyBudgetMS = 30000
		}
	}
	spec.Body = body

	hasAllowedSkills := false
	var fields map[string]any
	if err := yaml.Unmarshal([]byte(frontmatter), &fields); err == nil {
		_, hasAllowedSkills = fields["allowed_skills"]
	}
	if err := validate(spec, sourcePath, allowEmptySkills, hasAllowedSkills); err != nil {
		return nil, err
	}
	return spec, nil
}

func validate(s *Spec, src string, allowEmptySkills, hasAllowedSkills bool) error {
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
	if !hasAllowedSkills || (!allowEmptySkills && len(s.AllowedSkills) == 0) {
		missing = append(missing, "allowed_skills")
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s: missing required frontmatter fields: %s",
			src, strings.Join(missing, ", "))
	}
	if s.ContextBudget < 0 || s.LatencyBudgetMS < 0 {
		return fmt.Errorf("%s: context_budget and latency_budget_ms must be positive", src)
	}

	if src != "" {
		stem := stemFromName(src)
		if stem != "." && stem != s.ID {
			return fmt.Errorf("%s: id %q does not match filename stem %q", src, s.ID, stem)
		}
	}
	return nil
}

// stemFromName returns the filename stem (no extension) from a path or bare filename.
func stemFromName(name string) string {
	// Strip directory prefix.
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	// Strip extension.
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[:i]
	}
	return name
}
