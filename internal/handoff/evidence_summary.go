package handoff

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bryanbarton525/prism/internal/llm/runtime"
	"github.com/bryanbarton525/prism/internal/prompt"
)

type EvidenceSummary struct {
	Findings []Finding `json:"findings"`
	Risks    []Risk    `json:"risks"`
	Sources  []Source  `json:"sources"`
}

type Finding struct {
	Title      string `json:"title"`
	Summary    string `json:"summary"`
	Confidence string `json:"confidence"`
}

type Risk struct {
	Title      string `json:"title"`
	Severity   string `json:"severity"`
	Mitigation string `json:"mitigation"`
}

type Source struct {
	Type string `json:"type"`
	Path string `json:"path"`
	Line string `json:"line,omitempty"`
}

type RepoScanRequest struct {
	Task     string
	Evidence string
	Model    string
}

func GenerateRepoScanSummary(ctx context.Context, llm runtime.ModelRuntime, req RepoScanRequest) (EvidenceSummary, error) {
	if llm == nil {
		return EvidenceSummary{}, fmt.Errorf("model runtime is required")
	}
	var b prompt.Builder
	user := b.Add(prompt.SectionStableSystem, "System", "Summarize repository evidence for a Prism handoff.").
		Add(prompt.SectionStableRole, "Specialist Role", "You are a careful repo scan specialist. Return concise evidence-backed findings.").
		Add(prompt.SectionStableSchema, "Output Schema", "Return JSON matching the evidence_summary schema.").
		Add(prompt.SectionVolatileTask, "Task", req.Task).
		Add(prompt.SectionVolatileEvidence, "Evidence", req.Evidence).
		Build()
	resp, err := llm.GenerateStructured(ctx, runtime.StructuredRequest{
		ChatRequest: runtime.ChatRequest{
			Model:       req.Model,
			Messages:    []runtime.Message{{Role: "user", Content: user}},
			Temperature: float64Ptr(0.1),
			MaxTokens:   1200,
			Metadata: map[string]string{
				"workflow": "repo_scan",
				"agent":    "evidence_summary",
			},
		},
		Name:   "evidence_summary",
		Strict: true,
		Schema: EvidenceSummarySchema(),
	})
	if err != nil {
		return EvidenceSummary{}, fmt.Errorf("generating structured repo scan summary: %w", err)
	}
	data, err := json.Marshal(resp.Parsed)
	if err != nil {
		return EvidenceSummary{}, fmt.Errorf("encoding structured repo scan summary: %w", err)
	}
	var summary EvidenceSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return EvidenceSummary{}, fmt.Errorf("decoding structured repo scan summary: %w", err)
	}
	return summary, nil
}

func EvidenceSummarySchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"findings", "risks", "sources"},
		"properties": map[string]any{
			"findings": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"title", "summary", "confidence"},
					"properties": map[string]any{
						"title":      map[string]any{"type": "string"},
						"summary":    map[string]any{"type": "string"},
						"confidence": map[string]any{"type": "string"},
					},
				},
			},
			"risks": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"title", "severity", "mitigation"},
					"properties": map[string]any{
						"title":      map[string]any{"type": "string"},
						"severity":   map[string]any{"type": "string"},
						"mitigation": map[string]any{"type": "string"},
					},
				},
			},
			"sources": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"type", "path"},
					"properties": map[string]any{
						"type": map[string]any{"type": "string"},
						"path": map[string]any{"type": "string"},
						"line": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
}

func float64Ptr(v float64) *float64 {
	return &v
}
