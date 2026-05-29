// Package app wires together the agent registry, Ollama client, and prompt
// assembly to expose a single AgentRunner used by both the CLI and MCP adapters.
package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/ollama"
	"github.com/bryanbarton525/prism/internal/result"
)

// RunRequest holds the parameters for a single agent invocation.
type RunRequest struct {
	AgentID    string
	Task       string
	SkillNames []string
	Format     string // "json" or "markdown"
}

// AgentRunner is the shared core interface used by both CLI and MCP adapters.
type AgentRunner interface {
	ListAgents(ctx context.Context) ([]result.AgentSummary, error)
	Run(ctx context.Context, req RunRequest) (*result.RunResult, error)
	GetConstitution(ctx context.Context, agentID string) (*result.Constitution, error)
	Doctor(ctx context.Context) (*result.DoctorResult, error)
}

// Config holds configuration resolved from flags, env vars, and defaults.
type Config struct {
	OllamaHost string
	AgentDir   string
	SkillsDir  string
	RepoRoot   string
}

// Runner implements AgentRunner using the agent registry and Ollama client.
type Runner struct {
	cfg      Config
	registry *agent.Registry
	ollama   *ollama.Client
}

// New creates a Runner. The registry is loaded eagerly from cfg.AgentDir and
// cfg.SkillsDir.
func New(cfg Config) (*Runner, error) {
	reg, err := agent.LoadRegistry(cfg.AgentDir, cfg.SkillsDir)
	if err != nil {
		return nil, fmt.Errorf("loading agent registry: %w", err)
	}
	oc := ollama.New(cfg.OllamaHost)
	return &Runner{cfg: cfg, registry: reg, ollama: oc}, nil
}

// ListAgents returns a summary of all registered agents.
func (r *Runner) ListAgents(_ context.Context) ([]result.AgentSummary, error) {
	summaries := make([]result.AgentSummary, 0, len(r.registry.Agents))
	for _, spec := range r.registry.Agents {
		summaries = append(summaries, result.AgentSummary{
			ID:              spec.ID,
			Name:            spec.Name,
			Description:     spec.Description,
			Model:           spec.Model,
			AllowedSkills:   spec.AllowedSkills,
			LatencyBudgetMs: spec.LatencyBudgetMs,
		})
	}
	// Sort for deterministic output.
	sortSummaries(summaries)
	return summaries, nil
}

func sortSummaries(s []result.AgentSummary) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j].ID < s[j-1].ID; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// GetConstitution returns the constitution text for the given agent.
func (r *Runner) GetConstitution(_ context.Context, agentID string) (*result.Constitution, error) {
	spec, err := r.registry.Get(agentID)
	if err != nil {
		return nil, err
	}
	text, err := r.registry.ConstitutionText(spec, r.cfg.RepoRoot)
	if err != nil {
		return nil, err
	}
	return &result.Constitution{AgentID: agentID, Text: text}, nil
}

// Run invokes an agent with the given request and returns a normalized result.
func (r *Runner) Run(ctx context.Context, req RunRequest) (*result.RunResult, error) {
	spec, err := r.registry.Get(req.AgentID)
	if err != nil {
		return nil, err
	}

	skills, err := r.registry.GetSkills(spec, req.SkillNames)
	if err != nil {
		return nil, err
	}

	constitution, err := r.registry.ConstitutionText(spec, r.cfg.RepoRoot)
	if err != nil {
		return nil, err
	}

	prompt := buildPrompt(constitution, skills, req.Task, req.Format)

	messages := []ollama.ChatMessage{
		{Role: "system", Content: prompt},
		{Role: "user", Content: req.Task},
	}

	budget := spec.LatencyBudgetMs
	if budget <= 0 {
		budget = 60000
	}
	deadline := time.Now().Add(time.Duration(budget) * time.Millisecond)
	runCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	startTime := time.Now()
	resp, err := r.ollama.Chat(runCtx, spec.Model, messages, spec.Temperature, spec.ContextBudget)
	durationMs := int(time.Since(startTime).Milliseconds())

	skillNames := make([]string, 0, len(skills))
	for _, sk := range skills {
		skillNames = append(skillNames, sk.Name)
	}

	if err != nil {
		return &result.RunResult{
			AgentID:    req.AgentID,
			Model:      spec.Model,
			Status:     "error",
			Error:      err.Error(),
			SkillsUsed: skillNames,
			Usage:      result.Usage{DurationMs: durationMs},
		}, nil
	}

	raw := resp.Message.Content
	return &result.RunResult{
		AgentID:    req.AgentID,
		Model:      resp.Model,
		Status:     "ok",
		Summary:    extractSummary(raw),
		RawOutput:  raw,
		Confidence: "medium",
		Findings:   []string{},
		Artifacts:  []string{},
		SkillsUsed: skillNames,
		Usage: result.Usage{
			PromptTokensEstimate:     resp.PromptEvalCount,
			CompletionTokensEstimate: resp.EvalCount,
			DurationMs:               durationMs,
		},
	}, nil
}

// Doctor checks Ollama connectivity, model availability, and registry state.
func (r *Runner) Doctor(ctx context.Context) (*result.DoctorResult, error) {
	dr := &result.DoctorResult{
		OllamaHost: r.ollama.Host(),
		AgentDir:   r.cfg.AgentDir,
		AgentCount: len(r.registry.Agents),
		SkillCount: len(r.registry.Skills),
	}

	// Check Ollama connectivity.
	pingErr := r.ollama.Ping(ctx)
	if pingErr != nil {
		dr.Checks = append(dr.Checks, result.DoctorCheck{
			Name:    "ollama_connectivity",
			Status:  "fail",
			Message: pingErr.Error(),
		})
		dr.Status = "degraded"
		return dr, nil
	}
	dr.Checks = append(dr.Checks, result.DoctorCheck{
		Name:    "ollama_connectivity",
		Status:  "ok",
		Message: fmt.Sprintf("reachable at %s", r.ollama.Host()),
	})

	// List available models.
	models, modelsErr := r.ollama.ListModels(ctx)
	if modelsErr != nil {
		dr.Checks = append(dr.Checks, result.DoctorCheck{
			Name:    "ollama_models",
			Status:  "fail",
			Message: modelsErr.Error(),
		})
	} else {
		modelNames := make([]string, 0, len(models))
		for _, m := range models {
			modelNames = append(modelNames, m.Name)
		}
		dr.Checks = append(dr.Checks, result.DoctorCheck{
			Name:    "ollama_models",
			Status:  "ok",
			Message: fmt.Sprintf("%d model(s) available: %s", len(models), strings.Join(modelNames, ", ")),
		})
	}

	// Agent registry check.
	if len(r.registry.Agents) == 0 {
		dr.Checks = append(dr.Checks, result.DoctorCheck{
			Name:    "agent_registry",
			Status:  "warn",
			Message: fmt.Sprintf("no agents found in %s", r.cfg.AgentDir),
		})
	} else {
		agentIDs := make([]string, 0)
		for id := range r.registry.Agents {
			agentIDs = append(agentIDs, id)
		}
		sortStrings(agentIDs)
		dr.Checks = append(dr.Checks, result.DoctorCheck{
			Name:    "agent_registry",
			Status:  "ok",
			Message: fmt.Sprintf("%d agent(s): %s", len(agentIDs), strings.Join(agentIDs, ", ")),
		})
	}

	// Skills directory check.
	if len(r.registry.Skills) == 0 {
		dr.Checks = append(dr.Checks, result.DoctorCheck{
			Name:    "skill_registry",
			Status:  "warn",
			Message: fmt.Sprintf("no skills found in %s", r.cfg.SkillsDir),
		})
	} else {
		dr.Checks = append(dr.Checks, result.DoctorCheck{
			Name:    "skill_registry",
			Status:  "ok",
			Message: fmt.Sprintf("%d skill(s) loaded", len(r.registry.Skills)),
		})
	}

	dr.Status = "ok"
	return dr, nil
}

// buildPrompt assembles the system prompt from the constitution, attached skills,
// and output format hint.
func buildPrompt(constitution string, skills []*agent.SkillSpec, task, format string) string {
	var sb strings.Builder
	sb.WriteString("# Agent Constitution\n\n")
	sb.WriteString(strings.TrimSpace(constitution))
	sb.WriteString("\n\n")

	if len(skills) > 0 {
		sb.WriteString("# Attached Skills\n\n")
		for _, sk := range skills {
			sb.WriteString(fmt.Sprintf("## %s\n\n", sk.Name))
			sb.WriteString(strings.TrimSpace(sk.Body))
			sb.WriteString("\n\n")
		}
	}

	_ = task
	if format == "json" {
		sb.WriteString("# Output\n\nRespond with a JSON object containing: summary, findings (array), confidence (low/medium/high).\n")
	} else {
		sb.WriteString("# Output\n\nRespond with a clear Markdown summary of your findings.\n")
	}

	return sb.String()
}

// extractSummary attempts to pull the first paragraph from the model output as
// a short summary. Falls back to the first 200 characters.
func extractSummary(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	idx := strings.Index(raw, "\n\n")
	if idx > 0 && idx < 500 {
		return strings.TrimSpace(raw[:idx])
	}
	if len(raw) > 200 {
		return raw[:200]
	}
	return raw
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
