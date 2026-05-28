package result

import (
	"strings"
	"testing"
	"time"
)

func TestToJSON(t *testing.T) {
	r := RunResult{
		AgentID:    "github-cli",
		Model:      "llama3.1:8b",
		Status:     StatusOK,
		Summary:    "All clear.",
		Confidence: ConfidenceHigh,
		Findings:   []Finding{{Severity: "info", Text: "No blockers found."}},
		Artifacts:  []Artifact{},
	}
	data, err := r.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON(): %v", err)
	}
	s := string(data)
	for _, want := range []string{"github-cli", "llama3.1:8b", "ok", "All clear.", "high"} {
		if !strings.Contains(s, want) {
			t.Errorf("JSON missing %q", want)
		}
	}
}

func TestToMarkdown(t *testing.T) {
	r := RunResult{
		AgentID:    "kubectl",
		Model:      "llama3.1:8b",
		Status:     StatusOK,
		Summary:    "Pod is healthy.",
		Confidence: ConfidenceMedium,
		Findings: []Finding{
			{Severity: "warning", Text: "Restart count elevated."},
		},
		Artifacts: []Artifact{
			{Type: "command_output", Label: "kubectl get pods", Content: "pod-1   1/1   Running"},
		},
		Usage: Usage{DurationMS: 1234, PromptTokensEstimate: 100, CompletionTokensEstimate: 50},
	}
	md := r.ToMarkdown()
	for _, want := range []string{
		"kubectl", "llama3.1:8b", "ok", "Pod is healthy.", "medium",
		"Restart count elevated.", "kubectl get pods", "1234ms",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("Markdown missing %q", want)
		}
	}
}

func TestError(t *testing.T) {
	r := Error("agent-x", "llama3.1:8b", "something went wrong", 500*time.Millisecond)
	if r.Status != StatusError {
		t.Errorf("Status: want error, got %s", r.Status)
	}
	if r.Usage.DurationMS < 400 {
		t.Errorf("DurationMS too low: %d", r.Usage.DurationMS)
	}
}
