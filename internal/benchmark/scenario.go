package benchmark

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Scenario defines a benchmark fixture with eight delegations (full skill coverage).
type Scenario struct {
	ID          string `yaml:"id"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	BriefFile   string `yaml:"brief_file"`
	Delegations []Delegation `yaml:"delegations"`
	Assertions  Assertions   `yaml:"assertions"`
	Synthesis   Synthesis    `yaml:"synthesis"`
	dir         string
}

// Delegation is one Prism agent invocation in the delegated benchmark path.
type Delegation struct {
	ID               string   `yaml:"id"`
	AgentID          string   `yaml:"agent_id"`
	SkillNames       []string `yaml:"skill_names"`
	TaskFile         string   `yaml:"task_file"`
	EvidenceFiles    []string `yaml:"evidence_files"`
	MockResponseFile string   `yaml:"mock_response_file"`
}

// Assertions lists required phrases in the final incident report.
type Assertions struct {
	RequiredPhrases []string `yaml:"required_phrases"`
}

// Synthesis holds prompt templates for final report assembly.
type Synthesis struct {
	OrchestratorPromptFile string `yaml:"orchestrator_prompt_file"`
	DelegatedPromptFile      string `yaml:"delegated_prompt_file"`
}

// LoadScenario reads scenario.yaml from testdata/benchmarks/scenarios/<id>/.
func LoadScenario(root, id string) (*Scenario, error) {
	dir := filepath.Join(root, "testdata", "benchmarks", "scenarios", id)
	data, err := os.ReadFile(filepath.Join(dir, "scenario.yaml"))
	if err != nil {
		return nil, fmt.Errorf("reading scenario %q: %w", id, err)
	}
	var s Scenario
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing scenario %q: %w", id, err)
	}
	if s.ID == "" {
		s.ID = id
	}
	s.dir = dir
	return &s, nil
}

func (s *Scenario) Dir() string { return s.dir }

func (s *Scenario) ReadBrief() (string, error) {
	return readFile(filepath.Join(s.dir, s.BriefFile))
}

func (s *Scenario) ReadTask(rel string) (string, error) {
	return readFile(filepath.Join(s.dir, rel))
}

func (d *Delegation) BuildTask(scenarioDir string) (string, error) {
	tmpl, err := readFile(filepath.Join(scenarioDir, d.TaskFile))
	if err != nil {
		return "", err
	}
	var evidence strings.Builder
	for _, f := range d.EvidenceFiles {
		body, err := readFile(filepath.Join(scenarioDir, f))
		if err != nil {
			return "", err
		}
		evidence.WriteString("--- ")
		evidence.WriteString(f)
		evidence.WriteString(" ---\n")
		evidence.WriteString(body)
		evidence.WriteString("\n\n")
	}
	return strings.ReplaceAll(tmpl, "{{EVIDENCE}}", strings.TrimSpace(evidence.String())), nil
}

func (d *Delegation) MockResponse(scenarioDir string) (string, error) {
	return readFile(filepath.Join(scenarioDir, d.MockResponseFile))
}

func (s *Scenario) LoadAllEvidence() (string, error) {
	seen := map[string]struct{}{}
	var b strings.Builder
	for _, d := range s.Delegations {
		for _, f := range d.EvidenceFiles {
			if _, ok := seen[f]; ok {
				continue
			}
			seen[f] = struct{}{}
			body, err := readFile(filepath.Join(s.dir, f))
			if err != nil {
				return "", err
			}
			b.WriteString("## ")
			b.WriteString(f)
			b.WriteString("\n\n")
			b.WriteString(body)
			b.WriteString("\n\n")
		}
	}
	return strings.TrimSpace(b.String()), nil
}

func (s *Scenario) SynthesisResponse() (string, error) {
	return readFile(filepath.Join(s.dir, "responses", "synthesis.txt"))
}

func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	return string(data), nil
}

// FindRepoRoot walks up from cwd to locate the prism repository root.
func FindRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "agents")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "testdata", "benchmarks")); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("repository root not found")
		}
		dir = parent
	}
}
