// Package result defines the normalized output envelope for all Prism agents.
package result

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Status values for RunResult.
const (
	StatusOK             = "ok"
	StatusError          = "error"
	StatusValidationFail = "validation_fail"
	StatusTimeout        = "timeout"
)

// Confidence levels emitted by the local model.
const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
	ConfidenceNone   = ""
)

// Usage holds token and timing estimates from one agent invocation.
type Usage struct {
	PromptTokensEstimate     int   `json:"prompt_tokens_estimate"`
	CompletionTokensEstimate int   `json:"completion_tokens_estimate"`
	DurationMS               int64 `json:"duration_ms"`
}

// Finding is one discrete observation or diagnostic item.
type Finding struct {
	Severity string `json:"severity,omitempty"` // info | warning | error
	Text     string `json:"text"`
}

// Artifact is an inline data artifact returned by the model (command output,
// snippet, URL, etc.).
type Artifact struct {
	Type    string `json:"type,omitempty"` // snippet | url | file | command_output
	Label   string `json:"label,omitempty"`
	Content string `json:"content"`
}

// RunResult is the normalized envelope returned by every agent invocation.
type RunResult struct {
	AgentID    string     `json:"agent_id"`
	Model      string     `json:"model"`
	Status     string     `json:"status"`
	Summary    string     `json:"summary"`
	Findings   []Finding  `json:"findings"`
	Artifacts  []Artifact `json:"artifacts"`
	Confidence string     `json:"confidence,omitempty"`
	Usage      Usage      `json:"usage"`

	// RawOutput stores the unmodified model response for debugging.
	RawOutput string `json:"raw_output,omitempty"`
	// ValidationError is set when the model output did not match the expected
	// schema but the raw output is still preserved.
	ValidationError string `json:"validation_error,omitempty"`
}

// Error constructs an error RunResult.
func Error(agentID, model, message string, dur time.Duration) RunResult {
	return RunResult{
		AgentID:  agentID,
		Model:    model,
		Status:   StatusError,
		Summary:  message,
		Findings: []Finding{},
		Artifacts: []Artifact{},
		Usage:    Usage{DurationMS: dur.Milliseconds()},
	}
}

// ToJSON serializes r to indented JSON.
func (r RunResult) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// ToMarkdown renders r as a human-readable Markdown string.
func (r RunResult) ToMarkdown() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Agent: %s\n\n", r.AgentID))
	b.WriteString(fmt.Sprintf("**Model:** %s  \n", r.Model))
	b.WriteString(fmt.Sprintf("**Status:** %s  \n", r.Status))
	if r.Confidence != "" {
		b.WriteString(fmt.Sprintf("**Confidence:** %s  \n", r.Confidence))
	}
	b.WriteString("\n")

	b.WriteString("## Summary\n\n")
	b.WriteString(r.Summary)
	b.WriteString("\n\n")

	if len(r.Findings) > 0 {
		b.WriteString("## Findings\n\n")
		for _, f := range r.Findings {
			sev := f.Severity
			if sev == "" {
				sev = "info"
			}
			b.WriteString(fmt.Sprintf("- **[%s]** %s\n", sev, f.Text))
		}
		b.WriteString("\n")
	}

	if len(r.Artifacts) > 0 {
		b.WriteString("## Artifacts\n\n")
		for _, a := range r.Artifacts {
			label := a.Label
			if label == "" {
				label = a.Type
			}
			b.WriteString(fmt.Sprintf("### %s\n\n", label))
			b.WriteString("```\n")
			b.WriteString(a.Content)
			b.WriteString("\n```\n\n")
		}
	}

	b.WriteString(fmt.Sprintf("---\n_Duration: %dms | Prompt tokens (est.): %d | Completion tokens (est.): %d_\n",
		r.Usage.DurationMS, r.Usage.PromptTokensEstimate, r.Usage.CompletionTokensEstimate))

	return b.String()
}
