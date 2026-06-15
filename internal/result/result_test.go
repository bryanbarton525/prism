package result

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestToJSON_RoundTrip(t *testing.T) {
	r := RunResult{
		AgentID:    "github-cli",
		Model:      "llama3.1:8b",
		Status:     StatusOK,
		Summary:    "PR is clean.",
		Findings:   []Finding{{Severity: "info", Text: "All checks passed"}},
		Artifacts:  []Artifact{},
		Confidence: ConfidenceMedium,
		Usage: Usage{
			PromptTokensEstimate:     42,
			CompletionTokensEstimate: 10,
			DurationMS:               1500,
		},
		SkillsUsed:         []string{"gh-pr-triage"},
		ConstitutionSource: "path",
		ContextBudget:      6144,
	}

	data, err := r.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}

	var decoded RunResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.AgentID != r.AgentID {
		t.Errorf("AgentID: want %s, got %s", r.AgentID, decoded.AgentID)
	}
	if decoded.Status != StatusOK {
		t.Errorf("Status: want ok, got %s", decoded.Status)
	}
	if len(decoded.Findings) != 1 {
		t.Errorf("Findings: want 1, got %d", len(decoded.Findings))
	}
	if len(decoded.SkillsUsed) != 1 || decoded.SkillsUsed[0] != "gh-pr-triage" {
		t.Errorf("SkillsUsed: %v", decoded.SkillsUsed)
	}
	if decoded.ConstitutionSource != "path" {
		t.Errorf("ConstitutionSource: want path, got %s", decoded.ConstitutionSource)
	}
}

func TestToMarkdown_ContainsExpectedSections(t *testing.T) {
	r := RunResult{
		AgentID: "kubectl",
		Model:   "llama3.1:8b",
		Status:  StatusOK,
		Summary: "Pod crashlooping.",
		Findings: []Finding{
			{Severity: "error", Text: "OOMKilled"},
		},
		Artifacts: []Artifact{
			{Type: "command_output", Label: "pod logs", Content: "error: OOM"},
		},
		Confidence:            ConfidenceHigh,
		SkillsUsed:            []string{"kubectl-triage"},
		Usage:                 Usage{DurationMS: 2000},
		ContextBudgetExceeded: true,
	}

	md := r.ToMarkdown()
	for _, want := range []string{
		"# Agent: kubectl",
		"**Status:** ok",
		"**Confidence:** high",
		"**Skills:** kubectl-triage",
		"## Summary",
		"Pod crashlooping",
		"## Findings",
		"**[error]** OOMKilled",
		"## Artifacts",
		"### pod logs",
		"error: OOM",
		"context budget",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("Markdown should contain %q", want)
		}
	}
}

func TestError_Constructor(t *testing.T) {
	dur := 500 * time.Millisecond
	r := Error("myagent", "llama3.1:8b", "something failed", dur)
	if r.Status != StatusError {
		t.Errorf("Status: want error, got %s", r.Status)
	}
	if r.Usage.DurationMS != 500 {
		t.Errorf("DurationMS: want 500, got %d", r.Usage.DurationMS)
	}
	if r.Findings == nil {
		t.Error("Findings should be non-nil slice")
	}
}

func TestJSON_ContextBudgetExceededOmitted(t *testing.T) {
	r := RunResult{
		AgentID:   "x",
		Status:    StatusOK,
		Findings:  []Finding{},
		Artifacts: []Artifact{},
		// ContextBudgetExceeded is false — field should be absent from JSON
	}
	data, _ := r.ToJSON()
	if strings.Contains(string(data), `"context_budget_exceeded"`) {
		t.Error("context_budget_exceeded should be omitted when false")
	}
}
