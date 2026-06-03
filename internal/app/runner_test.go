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
	"time"

	"github.com/bryanbarton525/prism/internal/plugins"
	"github.com/bryanbarton525/prism/internal/result"
	"github.com/bryanbarton525/prism/pkg/observe"
)

// ---------------------------------------------------------------------------
// Fixture helpers
// ---------------------------------------------------------------------------

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// makeTestRoot creates a temporary project root with agents/ and skills/
// directories always present, plus any supplied spec and skill files.
func makeTestRoot(t *testing.T, agents map[string]string, skills map[string]string) string {
	t.Helper()
	root := t.TempDir()
	// Always create the baseline directories so New() succeeds.
	for _, d := range []string{"agents", "skills"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	for name, content := range agents {
		writeFile(t, filepath.Join(root, "agents", name), content)
	}
	for name, content := range skills {
		writeFile(t, filepath.Join(root, "skills", name, "SKILL.md"), content)
	}
	return root
}

// mockOllama starts a test HTTP server returning a canned chat response.
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

// mockOllamaError starts a server that always returns HTTP 500 for /api/chat.
func mockOllamaError(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
}

// ---------------------------------------------------------------------------
// Spec / skill fixture text
// ---------------------------------------------------------------------------

func githubCLISpec() string {
	return `---
id: github-cli
name: GitHub CLI
description: Diagnose PRs with gh.
model: llama3.1:8b
context_budget: 6144
allowed_skills:
  - gh-pr-triage
  - gh-actions-diagnostics
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

func kubectlSpec() string {
	return `---
id: kubectl
name: Kubernetes kubectl
description: Diagnose Kubernetes state.
model: llama3.1:8b
context_budget: 8192
allowed_skills:
  - kubectl-triage
latency_budget_ms: 30000
temperature: 0.1
tools:
  - kubernetes
---

# Kubernetes agent
`
}

func kubectlTriageSkill() string {
	return `---
name: kubectl-triage
description: Triage Kubernetes workloads.
---

Use read-only kubectl evidence.
`
}

type fakeRuntimePlugin struct {
	name    string
	content string
}

func (f fakeRuntimePlugin) Name() string {
	return f.name
}

func (f fakeRuntimePlugin) Tools() []plugins.ToolSpec {
	return []plugins.ToolSpec{{Name: "kubernetes.collect_diagnostics", ReadOnly: true}}
}

func (f fakeRuntimePlugin) Call(_ context.Context, call plugins.ToolCall) (plugins.ToolResult, error) {
	return plugins.ToolResult{
		Label:   "runtime-plugin:" + f.name,
		Content: "tool=" + call.Tool + " namespace=" + call.Args["namespace"] + "\n" + f.content,
	}, nil
}

type captureSink struct {
	events []observe.RunEvent
}

func (c *captureSink) ObserveRun(_ context.Context, event observe.RunEvent) error {
	c.events = append(c.events, event)
	return nil
}

// ---------------------------------------------------------------------------
// Runner construction
// ---------------------------------------------------------------------------

func TestNew_ValidConfig(t *testing.T) {
	root := makeTestRoot(t, map[string]string{"github-cli.md": githubCLISpec()}, nil)
	srv := mockOllama(t, "")
	defer srv.Close()

	runner, err := New(Config{RootDir: root, OllamaHost: srv.URL})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	if runner == nil {
		t.Fatal("runner should not be nil")
	}
}

func TestNew_BadAgentDir(t *testing.T) {
	_, err := New(Config{RootDir: "/nonexistent"})
	if err == nil {
		t.Fatal("expected error for missing agent directory")
	}
}

// ---------------------------------------------------------------------------
// ListAgents
// ---------------------------------------------------------------------------

func TestRunner_ListAgents(t *testing.T) {
	root := makeTestRoot(t, map[string]string{"github-cli.md": githubCLISpec()}, nil)
	srv := mockOllama(t, "")
	defer srv.Close()

	runner, _ := New(Config{RootDir: root, OllamaHost: srv.URL})
	list, err := runner.ListAgents(context.Background())
	if err != nil {
		t.Fatalf("ListAgents(): %v", err)
	}
	if len(list) != 1 || list[0].ID != "github-cli" {
		t.Errorf("unexpected list: %v", list)
	}
}

// ---------------------------------------------------------------------------
// GetConstitution
// ---------------------------------------------------------------------------

func TestRunner_GetConstitution_Body(t *testing.T) {
	root := makeTestRoot(t, map[string]string{"github-cli.md": githubCLISpec()}, nil)
	srv := mockOllama(t, "")
	defer srv.Close()

	runner, _ := New(Config{RootDir: root, OllamaHost: srv.URL})
	c, err := runner.GetConstitution(context.Background(), "github-cli")
	if err != nil {
		t.Fatalf("GetConstitution(): %v", err)
	}
	if c.AgentID != "github-cli" {
		t.Errorf("AgentID: want github-cli, got %s", c.AgentID)
	}
	if c.Source != "body" {
		t.Errorf("Source: want body, got %s", c.Source)
	}
	if !strings.Contains(c.Text, "GitHub CLI agent") {
		t.Errorf("constitution text missing expected content: %s", c.Text)
	}
}

func TestRunner_GetConstitution_Path(t *testing.T) {
	root := makeTestRoot(t, map[string]string{
		"github-cli.md": `---
id: github-cli
name: GitHub CLI
description: desc
model: llama3.1:8b
context_budget: 6144
allowed_skills: [gh-pr-triage]
latency_budget_ms: 30000
constitution_path: constitutions/github-cli.md
---
`,
	}, nil)
	writeFile(t, filepath.Join(root, "constitutions", "github-cli.md"),
		"# External constitution\nMission text.")
	srv := mockOllama(t, "")
	defer srv.Close()

	runner, _ := New(Config{RootDir: root, OllamaHost: srv.URL})
	c, err := runner.GetConstitution(context.Background(), "github-cli")
	if err != nil {
		t.Fatalf("GetConstitution(): %v", err)
	}
	if c.Source != "path" {
		t.Errorf("Source: want path, got %s", c.Source)
	}
	if !strings.Contains(c.Text, "External constitution") {
		t.Errorf("constitution text: %s", c.Text)
	}
	if c.Path == "" {
		t.Error("Path should be set for source=path")
	}
}

func TestRunner_GetConstitution_NotFound(t *testing.T) {
	root := makeTestRoot(t, map[string]string{}, nil)
	srv := mockOllama(t, "")
	defer srv.Close()

	runner, _ := New(Config{RootDir: root, OllamaHost: srv.URL})
	_, err := runner.GetConstitution(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

// ---------------------------------------------------------------------------
// Run — validation failures
// ---------------------------------------------------------------------------

func TestRunner_Run_NoSkills(t *testing.T) {
	root := makeTestRoot(t, map[string]string{"github-cli.md": githubCLISpec()}, nil)
	srv := mockOllama(t, "response")
	defer srv.Close()

	runner, _ := New(Config{RootDir: root, OllamaHost: srv.URL})
	res, err := runner.Run(context.Background(), RunRequest{
		AgentID:    "github-cli",
		Task:       "task",
		SkillNames: nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != result.StatusValidationFail {
		t.Errorf("status: want validation_fail, got %s", res.Status)
	}
}

func TestRunner_Run_SkillNotAllowed(t *testing.T) {
	root := makeTestRoot(t, map[string]string{"github-cli.md": githubCLISpec()}, nil)
	srv := mockOllama(t, "response")
	defer srv.Close()

	sink := &captureSink{}
	runner, _ := New(Config{RootDir: root, OllamaHost: srv.URL, EventSink: sink})
	res, err := runner.Run(context.Background(), RunRequest{
		AgentID:    "github-cli",
		Task:       "task",
		SkillNames: []string{"kubectl-triage"}, // not in allowed_skills
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != result.StatusValidationFail {
		t.Errorf("status: want validation_fail, got %s", res.Status)
	}
	if !strings.Contains(res.Summary, "allowed_skills") {
		t.Errorf("error message should reference allowed_skills: %s", res.Summary)
	}
	if len(sink.events) != 1 {
		t.Fatalf("expected one event, got %d", len(sink.events))
	}
	if sink.events[0].ValidationError == "" {
		t.Fatalf("validation error should be captured: %#v", sink.events[0])
	}
}

func TestRunner_Run_UnknownAgent(t *testing.T) {
	root := makeTestRoot(t, map[string]string{}, nil)
	srv := mockOllama(t, "")
	defer srv.Close()

	runner, _ := New(Config{RootDir: root, OllamaHost: srv.URL})
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

func TestRunner_Run_MissingSkillFile(t *testing.T) {
	root := makeTestRoot(t, map[string]string{"github-cli.md": githubCLISpec()}, nil)
	// gh-pr-triage is allowed but the skills dir is empty.
	srv := mockOllama(t, "response")
	defer srv.Close()

	runner, _ := New(Config{RootDir: root, OllamaHost: srv.URL})
	res, err := runner.Run(context.Background(), RunRequest{
		AgentID:    "github-cli",
		Task:       "task",
		SkillNames: []string{"gh-pr-triage"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != result.StatusError {
		t.Errorf("status: want error for missing skill file, got %s", res.Status)
	}
}

// ---------------------------------------------------------------------------
// Run — successful invocation
// ---------------------------------------------------------------------------

func TestRunner_Run_Success(t *testing.T) {
	root := makeTestRoot(t,
		map[string]string{"github-cli.md": githubCLISpec()},
		map[string]string{"gh-pr-triage": ghPRTriageSkill()},
	)
	srv := mockOllama(t, "PR #42 is clean with passing checks.")
	defer srv.Close()

	runner, err := New(Config{RootDir: root, OllamaHost: srv.URL})
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
	if len(res.SkillsUsed) != 1 || res.SkillsUsed[0] != "gh-pr-triage" {
		t.Errorf("SkillsUsed: %v", res.SkillsUsed)
	}
	if res.ConstitutionSource != "body" {
		t.Errorf("ConstitutionSource: want body, got %s", res.ConstitutionSource)
	}
	if res.ContextBudget != 6144 {
		t.Errorf("ContextBudget: want 6144, got %d", res.ContextBudget)
	}
	if res.PromptSizeEstimate == 0 {
		t.Error("PromptSizeEstimate should be > 0")
	}
	if res.RawOutput == "" {
		t.Error("RawOutput should be non-empty")
	}
}

func TestRunner_Run_EmitsRunEvent(t *testing.T) {
	root := makeTestRoot(t,
		map[string]string{"github-cli.md": githubCLISpec()},
		map[string]string{"gh-pr-triage": ghPRTriageSkill()},
	)
	srv := mockOllama(t, "analysis complete")
	defer srv.Close()

	sink := &captureSink{}
	runner, err := New(Config{RootDir: root, OllamaHost: srv.URL, EventSink: sink})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	res, err := runner.Run(context.Background(), RunRequest{
		AgentID:    "github-cli",
		Task:       "check pr",
		SkillNames: []string{"gh-pr-triage"},
		Metadata: observe.Metadata{
			ActorID:       "alice",
			WorkspaceID:   "platform",
			Source:        "team",
			CorrelationID: "corr-1",
		},
	})
	if err != nil {
		t.Fatalf("Run(): %v", err)
	}
	if res.Status != result.StatusOK {
		t.Fatalf("status = %s", res.Status)
	}
	if len(sink.events) != 1 {
		t.Fatalf("expected one event, got %d", len(sink.events))
	}
	event := sink.events[0]
	if event.RunID == "" {
		t.Fatal("RunID should be set")
	}
	if event.Timestamp.IsZero() {
		t.Fatal("Timestamp should be set")
	}
	if event.ActorID != "alice" || event.WorkspaceID != "platform" || event.Source != "team" || event.CorrelationID != "corr-1" {
		t.Fatalf("metadata not preserved: %#v", event.Metadata)
	}
	if event.AgentID != "github-cli" || event.Status != result.StatusOK || event.Model != "llama3.1:8b" {
		t.Fatalf("unexpected event identity/status: %#v", event)
	}
	if event.PromptTokensEstimate != 42 || event.CompletionTokensEstimate != 10 {
		t.Fatalf("unexpected token estimates: %#v", event)
	}
	if len(event.Skills) != 1 || event.Skills[0] != "gh-pr-triage" {
		t.Fatalf("skills not captured: %#v", event.Skills)
	}
}

func TestRunner_Run_MetadataPopulated(t *testing.T) {
	root := makeTestRoot(t,
		map[string]string{"github-cli.md": githubCLISpec()},
		map[string]string{"gh-pr-triage": ghPRTriageSkill()},
	)
	srv := mockOllama(t, "analysis complete")
	defer srv.Close()

	runner, _ := New(Config{RootDir: root, OllamaHost: srv.URL})
	res, err := runner.Run(context.Background(), RunRequest{
		AgentID:    "github-cli",
		Task:       "check pr",
		SkillNames: []string{"gh-pr-triage"},
	})
	if err != nil {
		t.Fatalf("Run(): %v", err)
	}

	// Verify orchestrator metadata fields.
	if res.ConstitutionSource == "" {
		t.Error("ConstitutionSource should be set")
	}
	if res.ContextBudget == 0 {
		t.Error("ContextBudget should be set")
	}
	if res.PromptSizeEstimate == 0 {
		t.Error("PromptSizeEstimate should be > 0")
	}
}

func TestRunner_Run_KubectlToolCollectsEvidence(t *testing.T) {
	root := makeTestRoot(t,
		map[string]string{"kubectl.md": kubectlSpec()},
		map[string]string{"kubectl-triage": kubectlTriageSkill()},
	)
	runtimePlugins := plugins.NewRegistry(fakeRuntimePlugin{
		name:    "kubernetes",
		content: "pod/temporal-frontend Init:2/3",
	})

	srv := mockOllama(t, `{"summary":"used evidence","findings":["ok"],"confidence":"high"}`)
	defer srv.Close()

	runner, _ := New(Config{RootDir: root, OllamaHost: srv.URL, RuntimePlugins: runtimePlugins})
	res, err := runner.Run(context.Background(), RunRequest{
		AgentID:    "kubectl",
		Task:       "Namespace: temporal. Diagnose Temporal.",
		SkillNames: []string{"kubectl-triage"},
	})
	if err != nil {
		t.Fatalf("Run(): %v", err)
	}

	var found bool
	for _, artifact := range res.Artifacts {
		if artifact.Label == "runtime-plugin:kubernetes" {
			found = true
			if !strings.Contains(artifact.Content, "namespace=temporal") {
				t.Fatalf("runtime evidence missing namespace argument:\n%s", artifact.Content)
			}
			if !strings.Contains(artifact.Content, "pod/temporal-frontend Init:2/3") {
				t.Fatalf("runtime evidence missing app output:\n%s", artifact.Content)
			}
		}
	}
	if !found {
		t.Fatalf("expected runtime-plugin:kubernetes artifact, got %#v", res.Artifacts)
	}
}

// ---------------------------------------------------------------------------
// Run — Ollama error handling
// ---------------------------------------------------------------------------

func TestRunner_Run_OllamaError(t *testing.T) {
	root := makeTestRoot(t,
		map[string]string{"github-cli.md": githubCLISpec()},
		map[string]string{"gh-pr-triage": ghPRTriageSkill()},
	)
	srv := mockOllamaError(t)
	defer srv.Close()

	runner, _ := New(Config{RootDir: root, OllamaHost: srv.URL})
	res, err := runner.Run(context.Background(), RunRequest{
		AgentID:    "github-cli",
		Task:       "task",
		SkillNames: []string{"gh-pr-triage"},
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if res.Status != result.StatusError {
		t.Errorf("status: want error, got %s", res.Status)
	}
	if res.SkillsUsed == nil {
		t.Error("SkillsUsed should be populated even on error")
	}
}

// ---------------------------------------------------------------------------
// Run — timeout / context deadline
// ---------------------------------------------------------------------------

func TestRunner_Run_Timeout(t *testing.T) {
	root := makeTestRoot(t,
		map[string]string{
			"slow-agent.md": `---
id: slow-agent
name: Slow
description: d
model: llama3.1:8b
context_budget: 4096
allowed_skills: [gh-pr-triage]
latency_budget_ms: 1
temperature: 0.1
---
body`,
		},
		map[string]string{"gh-pr-triage": ghPRTriageSkill()},
	)

	// The server sleeps long enough that the 1 ms latency_budget_ms will fire.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			time.Sleep(200 * time.Millisecond)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"model":   "llama3.1:8b",
				"message": map[string]string{"role": "assistant", "content": "late"},
				"done":    true,
			})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	runner, _ := New(Config{RootDir: root, OllamaHost: srv.URL})
	res, err := runner.Run(context.Background(), RunRequest{
		AgentID:    "slow-agent",
		Task:       "task",
		SkillNames: []string{"gh-pr-triage"},
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if res.Status != result.StatusTimeout && res.Status != result.StatusError {
		t.Errorf("expected timeout or error status, got %s: %s", res.Status, res.Summary)
	}
}

// ---------------------------------------------------------------------------
// Run — context budget exceeded
// ---------------------------------------------------------------------------

func TestRunner_Run_ContextBudgetExceeded(t *testing.T) {
	// Tiny context budget that the assembled prompt will exceed.
	spec := `---
id: tiny-agent
name: Tiny
description: d
model: llama3.1:8b
context_budget: 10
allowed_skills: [gh-pr-triage]
latency_budget_ms: 30000
temperature: 0.1
---

` + strings.Repeat("X", 500)

	root := makeTestRoot(t,
		map[string]string{"tiny-agent.md": spec},
		map[string]string{"gh-pr-triage": ghPRTriageSkill()},
	)
	srv := mockOllama(t, "response")
	defer srv.Close()

	sink := &captureSink{}
	runner, _ := New(Config{RootDir: root, OllamaHost: srv.URL, EventSink: sink})
	res, err := runner.Run(context.Background(), RunRequest{
		AgentID:    "tiny-agent",
		Task:       "task",
		SkillNames: []string{"gh-pr-triage"},
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	// Should still succeed but flag the budget exceedance in metadata.
	if res.Status == result.StatusValidationFail {
		t.Errorf("budget exceedance should not prevent invocation, got: %s", res.Summary)
	}
	if !res.ContextBudgetExceeded {
		t.Error("ContextBudgetExceeded should be true when prompt exceeds budget")
	}
	if len(sink.events) != 1 {
		t.Fatalf("expected one event, got %d", len(sink.events))
	}
	if !sink.events[0].ContextBudgetExceeded {
		t.Fatalf("event should capture ContextBudgetExceeded: %#v", sink.events[0])
	}
}

// ---------------------------------------------------------------------------
// AgentRunner interface compliance
// ---------------------------------------------------------------------------

func TestRunnerImplementsAgentRunner(t *testing.T) {
	// Compile-time check is enough; this test just makes it explicit.
	root := makeTestRoot(t, map[string]string{"github-cli.md": githubCLISpec()}, nil)
	srv := mockOllama(t, "")
	defer srv.Close()

	var _ AgentRunner = &Runner{}
	r, err := New(Config{RootDir: root, OllamaHost: srv.URL})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	var runner AgentRunner = r
	if runner == nil {
		t.Fatal("Runner must satisfy AgentRunner interface")
	}
}
