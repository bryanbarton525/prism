// Package app provides the shared AgentRunner used by both the CLI and MCP adapters.
package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/ollama"
	"github.com/bryanbarton525/prism/internal/result"
	"github.com/bryanbarton525/prism/internal/skill"
)

// Config holds the runtime configuration for the Runner.
type Config struct {
	AgentDir   string
	SkillsDir  string
	OllamaHost string
}

// RunRequest is the input to Runner.Run.
type RunRequest struct {
	AgentID    string
	Task       string   // task text from --input or --stdin
	SkillNames []string // required; validated against agent.allowed_skills
	Format     string   // "json" or "markdown"
}

// Runner implements the shared agent execution logic.
type Runner struct {
	cfg      Config
	registry *agent.Registry
	ollama   *ollama.Client
}

// New creates a Runner from cfg and loads all agent specs.
func New(cfg Config) (*Runner, error) {
	reg := agent.NewRegistry(cfg.AgentDir)
	if err := reg.Load(); err != nil {
		return nil, fmt.Errorf("loading agents: %w", err)
	}
	oc := ollama.NewClient(cfg.OllamaHost)
	return &Runner{cfg: cfg, registry: reg, ollama: oc}, nil
}

// ListAgents returns summaries for all registered agents.
func (r *Runner) ListAgents(_ context.Context) ([]agent.Summary, error) {
	return r.registry.List(), nil
}

// GetSpec returns the full Spec for agentID.
func (r *Runner) GetSpec(_ context.Context, agentID string) (*agent.Spec, error) {
	return r.registry.Get(agentID)
}

// Run executes one agent invocation end-to-end.
func (r *Runner) Run(ctx context.Context, req RunRequest) (result.RunResult, error) {
	start := time.Now()

	// 1. Resolve the agent spec.
	spec, err := r.registry.Get(req.AgentID)
	if err != nil {
		return result.Error(req.AgentID, "", err.Error(), time.Since(start)), nil
	}

	// 2. Require at least one skill.
	if len(req.SkillNames) == 0 {
		msg := "at least one --skills value is required"
		return result.RunResult{
			AgentID:  req.AgentID,
			Model:    spec.Model,
			Status:   result.StatusValidationFail,
			Summary:  msg,
			Findings: []result.Finding{},
			Artifacts: []result.Artifact{},
			Usage:    result.Usage{DurationMS: time.Since(start).Milliseconds()},
		}, nil
	}

	// 3. Validate each skill against the agent's allowed_skills.
	for _, name := range req.SkillNames {
		if !spec.AllowsSkill(name) {
			msg := fmt.Sprintf("skill %q is not in agent %q allowed_skills: [%s]",
				name, req.AgentID, strings.Join(spec.AllowedSkills, ", "))
			return result.RunResult{
				AgentID:  req.AgentID,
				Model:    spec.Model,
				Status:   result.StatusValidationFail,
				Summary:  msg,
				Findings: []result.Finding{},
				Artifacts: []result.Artifact{},
				Usage:    result.Usage{DurationMS: time.Since(start).Milliseconds()},
			}, nil
		}
	}

	// 4. Load skill content.
	skills, err := skill.ValidateSkillsDir(r.cfg.SkillsDir, req.SkillNames)
	if err != nil {
		return result.Error(req.AgentID, spec.Model, fmt.Sprintf("loading skills: %s", err), time.Since(start)), nil
	}

	// 5. Assemble the Ollama prompt.
	systemPrompt, userPrompt := assemblePrompt(spec, skills, req.Task)

	// 6. Apply a context deadline from the agent's latency budget if the
	//    caller did not already set a deadline.
	if spec.LatencyBudgetMS > 0 {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, time.Duration(spec.LatencyBudgetMS)*time.Millisecond)
			defer cancel()
		}
	}

	// 7. Call Ollama.
	chatReq := ollama.ChatRequest{
		Model: spec.Model,
		Messages: []ollama.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Options: &ollama.Options{
			Temperature: spec.Temperature,
			NumCtx:      spec.ContextBudget,
		},
	}

	chatResp, err := r.ollama.Chat(ctx, chatReq)
	elapsed := time.Since(start)
	if err != nil {
		status := result.StatusError
		if ctx.Err() == context.DeadlineExceeded {
			status = result.StatusTimeout
		}
		return result.RunResult{
			AgentID:  req.AgentID,
			Model:    spec.Model,
			Status:   status,
			Summary:  fmt.Sprintf("model invocation failed: %s", err),
			Findings: []result.Finding{},
			Artifacts: []result.Artifact{},
			Usage:    result.Usage{DurationMS: elapsed.Milliseconds()},
		}, nil
	}

	// 8. Build the result envelope.
	rawText := chatResp.Message.Content
	res := result.RunResult{
		AgentID:   req.AgentID,
		Model:     chatResp.Model,
		Status:    result.StatusOK,
		Summary:   rawText,
		RawOutput: rawText,
		Findings:  []result.Finding{},
		Artifacts: []result.Artifact{},
		Usage: result.Usage{
			PromptTokensEstimate:     chatResp.PromptEvalCount,
			CompletionTokensEstimate: chatResp.EvalCount,
			DurationMS:               elapsed.Milliseconds(),
		},
	}

	return res, nil
}

// assemblePrompt builds the system and user messages from the agent spec,
// attached skills, and task text.  The order follows the implementation plan:
// constitution → skills → task.
func assemblePrompt(spec *agent.Spec, skills map[string]*skill.Skill, task string) (system, user string) {
	var sb strings.Builder

	// Constitution / agent body.
	sb.WriteString(spec.Body)
	sb.WriteString("\n\n")

	// Attached skills — only the requested subset.
	if len(skills) > 0 {
		sb.WriteString("# Attached Agent Skills\n\n")
		for _, sk := range skills {
			sb.WriteString(sk.FullText())
			sb.WriteString("\n\n")
		}
	}

	system = strings.TrimSpace(sb.String())
	user = strings.TrimSpace(task)
	return system, user
}
