// Package agent loads and validates Prism agent specifications from the agents/
// directory. Each agent is a Markdown file with YAML frontmatter. The package
// also validates that allowed_skills references resolve to real loaded skills.
package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/bryanbarton525/prism/internal/skill"
)

// Frontmatter holds the required and recommended fields from an agent spec.
type Frontmatter struct {
	ID              string   `yaml:"id"`
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description"`
	Model           string   `yaml:"model"`
	ContextBudget   int      `yaml:"context_budget"`
	AllowedSkills   []string `yaml:"allowed_skills"`
	LatencyBudgetMS int      `yaml:"latency_budget_ms"`

	// Recommended
	Temperature     float64  `yaml:"temperature,omitempty"`
	Tools           []string `yaml:"tools,omitempty"`
	Outputs         string   `yaml:"outputs,omitempty"`
	ConstitutionPath string  `yaml:"constitution_path,omitempty"`

	// Optional
	Models     []string          `yaml:"models,omitempty"`
	TokenBudget int              `yaml:"token_budget,omitempty"`
	Metadata   map[string]string `yaml:"metadata,omitempty"`
}

// Agent is a fully loaded and validated Prism agent specification.
type Agent struct {
	// FileName is the base name of the source Markdown file (e.g. "github-cli.md").
	FileName string
	// FileStem is the filename without extension (e.g. "github-cli").
	FileStem string
	// Frontmatter holds validated metadata parsed from the file.
	Frontmatter Frontmatter
	// Body is the Markdown body of the spec file.
	Body string
}

// ValidationError collects one or more validation problems found in an agent spec.
type ValidationError struct {
	FileName string
	Errors   []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("agent %q: %s", e.FileName, strings.Join(e.Errors, "; "))
}

// LoadAll reads every .md file (excluding README.md) in agentsDir, parses and
// validates each spec, and returns the slice of valid agents. Any file that
// fails validation is skipped and its error is accumulated into the returned
// error (as a joined multi-error).
func LoadAll(agentsDir string) ([]*Agent, error) {
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return nil, fmt.Errorf("reading agents directory %q: %w", agentsDir, err)
	}

	var agents []*Agent
	var errs []error

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") || strings.EqualFold(name, "README.md") {
			continue
		}
		a, err := Load(filepath.Join(agentsDir, name))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		agents = append(agents, a)
	}

	return agents, errors.Join(errs...)
}

// Load reads a single agent Markdown file, validates the frontmatter, and
// returns the populated Agent.
func Load(path string) (*Agent, error) {
	fileName := filepath.Base(path)
	fileStem := strings.TrimSuffix(fileName, ".md")

	fm, body, err := parseFrontmatter(path)
	if err != nil {
		return nil, &ValidationError{FileName: fileName, Errors: []string{err.Error()}}
	}

	if err := validateFrontmatter(fm, fileStem); err != nil {
		return nil, err
	}

	return &Agent{
		FileName:    fileName,
		FileStem:    fileStem,
		Frontmatter: fm,
		Body:        body,
	}, nil
}

// ValidateSkillAllowlists checks that every skill name in each agent's
// allowed_skills list resolves to a real skill in the provided index.
// The skillIndex maps skill directory names to loaded Skills.
func ValidateSkillAllowlists(agents []*Agent, skillIndex map[string]*skill.Skill) error {
	var errs []error
	for _, a := range agents {
		var problems []string
		for _, sk := range a.Frontmatter.AllowedSkills {
			if _, ok := skillIndex[sk]; !ok {
				problems = append(problems, fmt.Sprintf("allowed_skill %q not found in skills directory", sk))
			}
		}
		if len(problems) > 0 {
			errs = append(errs, &ValidationError{FileName: a.FileName, Errors: problems})
		}
	}
	return errors.Join(errs...)
}

// BuildSkillIndex returns a map keyed by skill directory name (Skill.DirName)
// for fast allowlist lookup.
func BuildSkillIndex(skills []*skill.Skill) map[string]*skill.Skill {
	idx := make(map[string]*skill.Skill, len(skills))
	for _, s := range skills {
		idx[s.DirName] = s
	}
	return idx
}

// parseFrontmatter reads a Markdown file and splits it into YAML frontmatter
// and the body. The file must begin with "---\n".
func parseFrontmatter(path string) (Frontmatter, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Frontmatter{}, "", fmt.Errorf("reading %s: %w", path, err)
	}

	content := string(raw)
	if !strings.HasPrefix(content, "---\n") {
		return Frontmatter{}, "", fmt.Errorf("%s does not start with YAML frontmatter delimiter (---)", filepath.Base(path))
	}

	rest := content[4:]
	end := strings.Index(rest, "\n---")
	if end == -1 {
		return Frontmatter{}, "", fmt.Errorf("%s frontmatter block is not closed with ---", filepath.Base(path))
	}

	yamlBlock := rest[:end]
	body := strings.TrimPrefix(rest[end:], "\n---\n")
	body = strings.TrimPrefix(body, "\n---")
	body = strings.TrimLeft(body, "\n")

	var fm Frontmatter
	if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err != nil {
		return Frontmatter{}, "", fmt.Errorf("parsing frontmatter in %s: %w", filepath.Base(path), err)
	}

	return fm, body, nil
}

// validateFrontmatter checks required fields and the id-to-filestem match.
func validateFrontmatter(fm Frontmatter, fileStem string) error {
	var problems []string

	if strings.TrimSpace(fm.ID) == "" {
		problems = append(problems, "frontmatter field 'id' is required")
	} else if fm.ID != fileStem {
		problems = append(problems, fmt.Sprintf("frontmatter 'id' (%q) must match file stem (%q)", fm.ID, fileStem))
	}

	if strings.TrimSpace(fm.Name) == "" {
		problems = append(problems, "frontmatter field 'name' is required")
	}

	if strings.TrimSpace(fm.Description) == "" {
		problems = append(problems, "frontmatter field 'description' is required")
	}

	if strings.TrimSpace(fm.Model) == "" {
		problems = append(problems, "frontmatter field 'model' is required")
	}

	if fm.ContextBudget <= 0 {
		problems = append(problems, "frontmatter field 'context_budget' must be a positive integer")
	}

	if len(fm.AllowedSkills) == 0 {
		problems = append(problems, "frontmatter field 'allowed_skills' must list at least one skill")
	}

	if fm.LatencyBudgetMS <= 0 {
		problems = append(problems, "frontmatter field 'latency_budget_ms' must be a positive integer")
	}

	if len(problems) > 0 {
		return &ValidationError{FileName: fileStem + ".md", Errors: problems}
	}
	return nil
}
