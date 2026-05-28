package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bryanbarton525/prism/internal/result"
	"github.com/bryanbarton525/prism/internal/skill"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func makeAgentDir(t *testing.T, agents map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for filename, content := range agents {
		if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644); err != nil {
			t.Fatalf("writing agent spec: %v", err)
		}
	}
	return dir
}

func makeSkillsDir(t *testing.T, skills map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range skills {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("creating skill dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("writing SKILL.md: %v", err)
		}
	}
	return root
}

func githubCLISpec() string {
	return `---
id: github-cli
name: GitHub CLI
description: Diagnose PRs with gh.
model: llama3.1:8b
context_budget: 6144
allowed_skills:
  - gh-pr-triage
latency_budget_ms: 30000
temperature: 0.1
---

# GitHub CLI agent

Inspect GitHub state.
`
}

func ghPRTriageSkill() string {
	return `---
name: gh-pr-triage
description: Triage pull requests with gh.
---

1. Gather PR metadata.
2. Return findings.
`
}

// mockOllama starts a test HTTP server that returns a canned chat response.
func mockOllama(t *testing.T, responseText string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version":"0.1.0"}`))
		case "/api/tags":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"models":[{"name":"llama3.1:8b"}]}`))
		case "/api/chat":
			resp := map[string]interface{}{
				"model": "llama3.1:8b",
				"message": map[string]string{
					"role":    "assistant",
					"content": responseText,
				},
				"done":              true,
				"prompt_eval_count": 42,
				"eval_count":        10,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		default:
			http.NotFound(w, r)
		}
	}))
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestRunner_ListAgents(t *testing.T) {
	agentDir := makeAgentDir(t, map[string]string{
		"github-cli.md": githubCLISpec(),
	})
	skillsDir := makeSkillsDir(t, map[string]string{})

	srv := mockOllama(t, "")
	defer srv.Close()

	runner, err := New(Config{AgentDir: agentDir, SkillsDir: skillsDir, OllamaHost: srv.URL})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	summaries, err := runner.ListAgents(context.Background())
	if err != nil {
		t.Fatalf("ListAgents(): %v", err)
	}
	if len(summaries) != 1 || summaries[0].ID != "github-cli" {
		t.Errorf("unexpected summaries: %v", summaries)
	}
}

func TestRunner_Run_SkillValidation_NoSkills(t *testing.T) {
	agentDir := makeAgentDir(t, map[string]string{"github-cli.md": githubCLISpec()})
	skillsDir := makeSkillsDir(t, map[string]string{})
	srv := mockOllama(t, "some response")
	defer srv.Close()

	runner, _ := New(Config{AgentDir: agentDir, SkillsDir: skillsDir, OllamaHost: srv.URL})
	res, err := runner.Run(context.Background(), RunRequest{
		AgentID:    "github-cli",
		Task:       "check pr",
		SkillNames: nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != result.StatusValidationFail {
		t.Errorf("status: want %s, got %s", result.StatusValidationFail, res.Status)
	}
}

func TestRunner_Run_SkillValidation_NotAllowed(t *testing.T) {
	agentDir := makeAgentDir(t, map[string]string{"github-cli.md": githubCLISpec()})
	skillsDir := makeSkillsDir(t, map[string]string{})
	srv := mockOllama(t, "some response")
	defer srv.Close()

	runner, _ := New(Config{AgentDir: agentDir, SkillsDir: skillsDir, OllamaHost: srv.URL})
	res, err := runner.Run(context.Background(), RunRequest{
		AgentID:    "github-cli",
		Task:       "check pr",
		SkillNames: []string{"kubectl-triage"}, // not in github-cli's allowed_skills
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != result.StatusValidationFail {
		t.Errorf("status: want %s, got %s", result.StatusValidationFail, res.Status)
	}
}

func TestRunner_Run_Success(t *testing.T) {
	agentDir := makeAgentDir(t, map[string]string{"github-cli.md": githubCLISpec()})
	skillsDir := makeSkillsDir(t, map[string]string{"gh-pr-triage": ghPRTriageSkill()})
	srv := mockOllama(t, "PR #42 is clean with passing checks.")
	defer srv.Close()

	runner, err := New(Config{AgentDir: agentDir, SkillsDir: skillsDir, OllamaHost: srv.URL})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	res, err := runner.Run(context.Background(), RunRequest{
		AgentID:    "github-cli",
		Task:       "Is PR #42 ready to merge?",
		SkillNames: []string{"gh-pr-triage"},
	})
	if err != nil {
		t.Fatalf("Run(): %v", err)
	}
	if res.Status != result.StatusOK {
		t.Errorf("status: want ok, got %s (summary: %s)", res.Status, res.Summary)
	}
	if res.AgentID != "github-cli" {
		t.Errorf("AgentID: want github-cli, got %s", res.AgentID)
	}
	if !strings.Contains(res.Summary, "PR #42") {
		t.Errorf("Summary should contain model response, got: %s", res.Summary)
	}
	if res.Usage.PromptTokensEstimate != 42 {
		t.Errorf("PromptTokensEstimate: want 42, got %d", res.Usage.PromptTokensEstimate)
	}
}

func TestRunner_Run_UnknownAgent(t *testing.T) {
	agentDir := makeAgentDir(t, map[string]string{})
	skillsDir := makeSkillsDir(t, map[string]string{})
	srv := mockOllama(t, "")
	defer srv.Close()

	runner, _ := New(Config{AgentDir: agentDir, SkillsDir: skillsDir, OllamaHost: srv.URL})
	res, err := runner.Run(context.Background(), RunRequest{
		AgentID:    "nonexistent",
		Task:       "task",
		SkillNames: []string{"some-skill"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != result.StatusError {
		t.Errorf("status: want error, got %s", res.Status)
	}
}

func TestAssemblePrompt_ContainsSkillContent(t *testing.T) {
	agentDir := makeAgentDir(t, map[string]string{"github-cli.md": githubCLISpec()})
	skillsDir := makeSkillsDir(t, map[string]string{"gh-pr-triage": ghPRTriageSkill()})
	srv := mockOllama(t, "response")
	defer srv.Close()

	runner, err := New(Config{AgentDir: agentDir, SkillsDir: skillsDir, OllamaHost: srv.URL})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	spec, err := runner.GetSpec(context.Background(), "github-cli")
	if err != nil {
		t.Fatalf("GetSpec(): %v", err)
	}

	skills, err := skill.ValidateSkillsDir(skillsDir, []string{"gh-pr-triage"})
	if err != nil {
		t.Fatalf("loading skills: %v", err)
	}

	system, user := assemblePrompt(spec, skills, "Is PR ready?")

	if !strings.Contains(system, "GitHub CLI agent") {
		t.Error("system prompt should contain constitution body")
	}
	if !strings.Contains(system, "gh-pr-triage") {
		t.Error("system prompt should contain skill name")
	}
	if user != "Is PR ready?" {
		t.Errorf("user prompt: want %q, got %q", "Is PR ready?", user)
	}
}
