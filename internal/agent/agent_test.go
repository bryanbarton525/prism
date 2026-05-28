package agent_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/skill"
)

const validAgentMD = `---
id: my-agent
name: My Agent
description: Does useful diagnostic work for the orchestrator.
model: llama3.1:8b
context_budget: 4096
temperature: 0.1
allowed_skills:
  - my-skill
latency_budget_ms: 30000
tools:
  - gh
outputs: summary findings confidence
---

# My agent constitution

## Mission

Run diagnostics.
`

func writeAgentFile(t *testing.T, dir, filename, content string) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestLoad_Valid(t *testing.T) {
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "my-agent.md", validAgentMD)

	a, err := agent.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.FileStem != "my-agent" {
		t.Errorf("FileStem = %q, want %q", a.FileStem, "my-agent")
	}
	if a.Frontmatter.ID != "my-agent" {
		t.Errorf("ID = %q, want %q", a.Frontmatter.ID, "my-agent")
	}
	if a.Frontmatter.Name != "My Agent" {
		t.Errorf("Name = %q, want %q", a.Frontmatter.Name, "My Agent")
	}
	if len(a.Frontmatter.AllowedSkills) != 1 || a.Frontmatter.AllowedSkills[0] != "my-skill" {
		t.Errorf("AllowedSkills = %v, want [my-skill]", a.Frontmatter.AllowedSkills)
	}
	if !strings.Contains(a.Body, "My agent constitution") {
		t.Errorf("Body does not contain expected content, got: %q", a.Body)
	}
}

func TestLoad_IDMismatch(t *testing.T) {
	md := `---
id: wrong-id
name: My Agent
description: Does something.
model: llama3.1:8b
context_budget: 4096
allowed_skills:
  - my-skill
latency_budget_ms: 30000
---
`
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "my-agent.md", md)

	_, err := agent.Load(path)
	if err == nil {
		t.Fatal("expected error for id/filestem mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "wrong-id") {
		t.Errorf("error should mention mismatched id, got: %v", err)
	}
}

func TestLoad_MissingID(t *testing.T) {
	md := `---
name: My Agent
description: Does something.
model: llama3.1:8b
context_budget: 4096
allowed_skills:
  - my-skill
latency_budget_ms: 30000
---
`
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "my-agent.md", md)

	_, err := agent.Load(path)
	if err == nil {
		t.Fatal("expected error for missing id, got nil")
	}
	if !strings.Contains(err.Error(), "'id'") {
		t.Errorf("error should mention 'id' field, got: %v", err)
	}
}

func TestLoad_MissingModel(t *testing.T) {
	md := `---
id: my-agent
name: My Agent
description: Does something.
context_budget: 4096
allowed_skills:
  - my-skill
latency_budget_ms: 30000
---
`
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "my-agent.md", md)

	_, err := agent.Load(path)
	if err == nil {
		t.Fatal("expected error for missing model, got nil")
	}
	if !strings.Contains(err.Error(), "'model'") {
		t.Errorf("error should mention 'model' field, got: %v", err)
	}
}

func TestLoad_MissingContextBudget(t *testing.T) {
	md := `---
id: my-agent
name: My Agent
description: Does something.
model: llama3.1:8b
allowed_skills:
  - my-skill
latency_budget_ms: 30000
---
`
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "my-agent.md", md)

	_, err := agent.Load(path)
	if err == nil {
		t.Fatal("expected error for missing context_budget, got nil")
	}
	if !strings.Contains(err.Error(), "'context_budget'") {
		t.Errorf("error should mention 'context_budget' field, got: %v", err)
	}
}

func TestLoad_EmptyAllowedSkills(t *testing.T) {
	md := `---
id: my-agent
name: My Agent
description: Does something.
model: llama3.1:8b
context_budget: 4096
latency_budget_ms: 30000
---
`
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "my-agent.md", md)

	_, err := agent.Load(path)
	if err == nil {
		t.Fatal("expected error for empty allowed_skills, got nil")
	}
	if !strings.Contains(err.Error(), "'allowed_skills'") {
		t.Errorf("error should mention 'allowed_skills' field, got: %v", err)
	}
}

func TestLoad_MissingLatencyBudget(t *testing.T) {
	md := `---
id: my-agent
name: My Agent
description: Does something.
model: llama3.1:8b
context_budget: 4096
allowed_skills:
  - my-skill
---
`
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "my-agent.md", md)

	_, err := agent.Load(path)
	if err == nil {
		t.Fatal("expected error for missing latency_budget_ms, got nil")
	}
	if !strings.Contains(err.Error(), "'latency_budget_ms'") {
		t.Errorf("error should mention 'latency_budget_ms' field, got: %v", err)
	}
}

func TestLoadAll_SkipsREADME(t *testing.T) {
	dir := t.TempDir()
	writeAgentFile(t, dir, "README.md", "# Readme file")
	writeAgentFile(t, dir, "my-agent.md", validAgentMD)

	agents, err := agent.LoadAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("expected 1 agent (README should be skipped), got %d", len(agents))
	}
}

func TestLoadAll_PartialErrors(t *testing.T) {
	dir := t.TempDir()

	// Valid agent
	writeAgentFile(t, dir, "my-agent.md", validAgentMD)

	// Invalid agent: missing required fields
	bad := `---
id: bad-agent
---
`
	writeAgentFile(t, dir, "bad-agent.md", bad)

	agents, err := agent.LoadAll(dir)
	if err == nil {
		t.Fatal("expected error for invalid agent, got nil")
	}
	if len(agents) != 1 {
		t.Errorf("expected 1 valid agent, got %d", len(agents))
	}
}

func TestValidateSkillAllowlists_AllPresent(t *testing.T) {
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "my-agent.md", validAgentMD)
	a, err := agent.Load(path)
	if err != nil {
		t.Fatalf("load agent: %v", err)
	}

	skillIndex := map[string]*skill.Skill{
		"my-skill": {DirName: "my-skill"},
	}
	if err := agent.ValidateSkillAllowlists([]*agent.Agent{a}, skillIndex); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateSkillAllowlists_MissingSkill(t *testing.T) {
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "my-agent.md", validAgentMD)
	a, err := agent.Load(path)
	if err != nil {
		t.Fatalf("load agent: %v", err)
	}

	// Empty skill index — "my-skill" is unresolvable.
	skillIndex := map[string]*skill.Skill{}
	err = agent.ValidateSkillAllowlists([]*agent.Agent{a}, skillIndex)
	if err == nil {
		t.Fatal("expected error for missing skill reference, got nil")
	}
	if !strings.Contains(err.Error(), "my-skill") {
		t.Errorf("error should mention missing skill name, got: %v", err)
	}
}

func TestBuildSkillIndex(t *testing.T) {
	skills := []*skill.Skill{
		{DirName: "skill-a"},
		{DirName: "skill-b"},
	}
	idx := agent.BuildSkillIndex(skills)
	if len(idx) != 2 {
		t.Errorf("expected 2 entries, got %d", len(idx))
	}
	if _, ok := idx["skill-a"]; !ok {
		t.Error("expected skill-a in index")
	}
	if _, ok := idx["skill-b"]; !ok {
		t.Error("expected skill-b in index")
	}
}

func TestLoadAll_RealAgentsDirectory(t *testing.T) {
	agentsDir := "../../agents"
	if _, err := os.Stat(agentsDir); os.IsNotExist(err) {
		t.Skipf("agents dir %q not found, skipping integration test", agentsDir)
	}

	agents, err := agent.LoadAll(agentsDir)
	if err != nil {
		t.Errorf("real agents directory contains validation errors: %v", err)
	}
	if len(agents) == 0 {
		t.Error("expected at least one agent to be loaded from real agents directory")
	}
	for _, a := range agents {
		t.Logf("loaded agent: %s (%s)", a.FileStem, a.Frontmatter.Name)
	}
}

func TestValidateSkillAllowlists_RealRepository(t *testing.T) {
	agentsDir := "../../agents"
	skillsRoot := "../../skills"

	for _, d := range []string{agentsDir, skillsRoot} {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			t.Skipf("directory %q not found, skipping integration test", d)
		}
	}

	skills, skillErr := skill.LoadAll(skillsRoot)
	if skillErr != nil {
		t.Fatalf("skill loading errors: %v", skillErr)
	}
	skillIndex := agent.BuildSkillIndex(skills)

	agents, agentErr := agent.LoadAll(agentsDir)
	if agentErr != nil {
		t.Fatalf("agent loading errors: %v", agentErr)
	}

	if err := agent.ValidateSkillAllowlists(agents, skillIndex); err != nil {
		t.Errorf("allowlist validation errors: %v", err)
	}
}
