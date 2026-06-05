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

// Confidence levels that local models may emit.
const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
	ConfidenceNone   = ""
)

// Usage holds token-count and timing estimates from one agent invocation.
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

// Artifact is an inline data artifact returned by the model.
type Artifact struct {
	Type    string `json:"type,omitempty"` // snippet | url | file | command_output
	Label   string `json:"label,omitempty"`
	Content string `json:"content"`
}

// RunResult is the normalized envelope returned by every agent invocation.
// All fields are stable across CLI and MCP adapters so orchestrators can
// depend on the schema regardless of how they call Prism.
type RunResult struct {
	AgentID    string     `json:"agent_id"`
	Model      string     `json:"model"`
	Status     string     `json:"status"`
	Summary    string     `json:"summary"`
	Findings   []Finding  `json:"findings"`
	Artifacts  []Artifact `json:"artifacts"`
	Confidence string     `json:"confidence,omitempty"`
	Usage      Usage      `json:"usage"`

	// Metadata fields for the orchestrator to judge usefulness.

	// SkillsUsed lists the skill names that were attached to this invocation.
	SkillsUsed []string `json:"skills_used,omitempty"`
	// ConstitutionSource identifies how the constitution was resolved:
	// "path" (constitution_path field), "body" (inline spec body), or
	// "legacy" (constitutions/<id>.md fallback).
	ConstitutionSource string `json:"constitution_source,omitempty"`
	// ContextBudget is the context_budget value from the agent spec.
	ContextBudget int `json:"context_budget,omitempty"`
	// PromptSizeEstimate is the estimated character count of the assembled
	// system prompt before it was sent to the model.
	PromptSizeEstimate int `json:"prompt_size_estimate,omitempty"`
	// ContextBudgetExceeded is true when the assembled prompt exceeded the
	// agent's declared context_budget (using ~4 chars/token heuristic).
	ContextBudgetExceeded bool   `json:"context_budget_exceeded,omitempty"`
	PolicyDecision        string `json:"policy_decision,omitempty"`
	PolicyReason          string `json:"policy_reason,omitempty"`
	BundleID              string `json:"bundle_id,omitempty"`
	BundleVersion         string `json:"bundle_version,omitempty"`

	// RawOutput stores the unmodified model response for debugging.
	RawOutput string `json:"raw_output,omitempty"`
	// ValidationError is set when the model output did not conform to the
	// expected schema; the raw output is still preserved.
	ValidationError string `json:"validation_error,omitempty"`
}

// Error constructs an error RunResult.
func Error(agentID, model, message string, dur time.Duration) RunResult {
	return RunResult{
		AgentID:   agentID,
		Model:     model,
		Status:    StatusError,
		Summary:   message,
		Findings:  []Finding{},
		Artifacts: []Artifact{},
		Usage:     Usage{DurationMS: dur.Milliseconds()},
	}
}

// ToJSON serializes r to indented JSON.
func (r RunResult) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// ToMarkdown renders r as a human-readable Markdown report.
func (r RunResult) ToMarkdown() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Agent: %s\n\n", r.AgentID))
	b.WriteString(fmt.Sprintf("**Model:** %s  \n", r.Model))
	b.WriteString(fmt.Sprintf("**Status:** %s  \n", r.Status))
	if r.Confidence != "" {
		b.WriteString(fmt.Sprintf("**Confidence:** %s  \n", r.Confidence))
	}
	if len(r.SkillsUsed) > 0 {
		b.WriteString(fmt.Sprintf("**Skills:** %s  \n", strings.Join(r.SkillsUsed, ", ")))
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

	b.WriteString(fmt.Sprintf(
		"---\n_Duration: %dms | Prompt tokens (est.): %d | Completion tokens (est.): %d_\n",
		r.Usage.DurationMS,
		r.Usage.PromptTokensEstimate,
		r.Usage.CompletionTokensEstimate,
	))

	if r.ContextBudgetExceeded {
		b.WriteString("\n> **Warning:** assembled prompt exceeded the agent context budget.\n")
	}

	return b.String()
}
