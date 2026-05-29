// Package agent provides loading, validation, and registry of Prism agent
// specs and their associated Agent Skills.
package agent

// Spec holds the normalized runtime representation of a Prism agent loaded
// from an agents/<id>.md file (Markdown with YAML frontmatter).
type Spec struct {
	// --- Required frontmatter ---

	// ID is the stable agent identifier; must match the file stem.
	ID string `yaml:"id"`
	// Name is the display name shown in CLI and orchestrator output.
	Name string `yaml:"name"`
	// Description explains when to delegate work to this agent.
	Description string `yaml:"description"`
	// Model is the default Ollama model tag.
	Model string `yaml:"model"`
	// ContextBudget is the maximum prompt/input size for the local model (tokens).
	ContextBudget int `yaml:"context_budget"`
	// AllowedSkills lists skill directory names this agent may use at run time.
	AllowedSkills []string `yaml:"allowed_skills"`
	// LatencyBudgetMS is the hard deadline for benchmark and runtime warnings (ms).
	LatencyBudgetMS int `yaml:"latency_budget_ms"`

	// --- Recommended frontmatter ---

	// Temperature is the conservative sampling default for this agent.
	Temperature float64 `yaml:"temperature,omitempty"`
	// Tools enumerates the Prism-local CLI tools this agent may invoke.
	Tools []string `yaml:"tools,omitempty"`
	// Outputs describes the expected response schema sections.
	Outputs string `yaml:"outputs,omitempty"`
	// ConstitutionPath is an optional path to a separate constitution file.
	// When set, the Markdown body of the agent spec is not the constitution.
	ConstitutionPath string `yaml:"constitution_path,omitempty"`

	// --- Optional frontmatter ---

	// Models lists alternate Ollama tags when the default is unavailable.
	Models []string `yaml:"models,omitempty"`
	// TokenBudget is a soft target for assembled prompt size (orchestrator
	// planning hint, separate from ContextBudget).
	TokenBudget int `yaml:"token_budget,omitempty"`
	// Metadata holds arbitrary string key-value pairs for integrations.
	Metadata map[string]string `yaml:"metadata,omitempty"`

	// --- Resolved fields (not from frontmatter) ---

	// FilePath is the absolute or repo-relative path of the source file.
	FilePath string `yaml:"-"`
	// Body is the raw Markdown body that follows the frontmatter block.
	Body string `yaml:"-"`
	// Constitution is the resolved behavior contract text. It is populated by
	// LoaderOptions.ResolveConstitution and may come from Body, ConstitutionPath,
	// or the legacy constitutions/ directory.
	Constitution string `yaml:"-"`
}

// SkillSpec holds the normalized runtime representation of an Agent Skill
// loaded from a skills/<name>/SKILL.md file.
type SkillSpec struct {
	// --- Required frontmatter per Agent Skills spec ---

	// Name is the lowercase-hyphenated identifier that must match the
	// containing directory name.
	Name string `yaml:"name"`
	// Description explains what the skill does and when to use it (≤1024 chars).
	Description string `yaml:"description"`

	// --- Optional frontmatter ---

	// License identifies the license of the skill content.
	License string `yaml:"license,omitempty"`
	// Compatibility describes environment requirements (e.g. "Requires ollama").
	Compatibility string `yaml:"compatibility,omitempty"`
	// Metadata holds arbitrary string key-value pairs.
	Metadata map[string]string `yaml:"metadata,omitempty"`
	// AllowedTools is an experimental space-separated tool pre-approval list.
	AllowedTools string `yaml:"allowed-tools,omitempty"`

	// --- Resolved fields ---

	// DirPath is the directory that contains SKILL.md.
	DirPath string `yaml:"-"`
	// Body is the full Markdown body of SKILL.md.
	Body string `yaml:"-"`
}

// Summary returns a concise view of the agent for list display.
type Summary struct {
	ID          string
	Name        string
	Description string
	Model       string
}

// ToSummary converts a Spec into a Summary.
func (s *Spec) ToSummary() Summary {
	return Summary{
		ID:          s.ID,
		Name:        s.Name,
		Description: s.Description,
		Model:       s.Model,
	}
}
