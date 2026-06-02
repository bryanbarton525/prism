// Package app provides the shared AgentRunner used by both the CLI and MCP
// adapters. Neither adapter should duplicate the agent-resolution, prompt
// assembly, or result-normalization logic; all of that lives here.
package app

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/ollama"
	"github.com/bryanbarton525/prism/internal/plugins"
	kubeplugin "github.com/bryanbarton525/prism/internal/plugins/kubernetes"
	"github.com/bryanbarton525/prism/internal/result"
	"github.com/bryanbarton525/prism/internal/skill"
)

// AgentRunner is the service interface consumed by both the CLI and MCP adapters.
// Any adapter that satisfies this interface can be tested in isolation without
// a live Ollama instance.
type AgentRunner interface {
	// ListAgents returns lightweight summaries for every registered agent.
	ListAgents(ctx context.Context) ([]agent.Summary, error)
	// Run executes one agent invocation end-to-end and returns a normalized
	// result envelope with enough metadata for the orchestrator to judge
	// usefulness.
	Run(ctx context.Context, req RunRequest) (result.RunResult, error)
	// GetConstitution returns the resolved constitution text and its source for
	// a given agent ID. Useful for auditing and MCP get_constitution calls.
	GetConstitution(ctx context.Context, agentID string) (Constitution, error)
	// Doctor reports Ollama connectivity, registry state, and skill layout.
	Doctor(ctx context.Context) (result.DoctorResult, error)
}

// Constitution holds the resolved constitution text and provenance.
type Constitution struct {
	AgentID string `json:"agent_id"`
	// Source is one of "path", "body", or "legacy".
	Source string `json:"source"`
	// Path is set when Source is "path" or "legacy".
	Path string `json:"path,omitempty"`
	Text string `json:"text"`
}

// Config holds runtime configuration for a Runner.
type Config struct {
	// RootFS is the resolved fs.FS for the project root. When set it takes
	// priority over RootDir. Set by the CLI via rootresolver after resolving
	// the --root flag (local path or remote GitHub URL).
	RootFS fs.FS
	// RootLabel is the human-readable root descriptor for display (e.g.
	// "https://github.com/owner/repo" or "/local/path"). Used in doctor output.
	RootLabel string
	// RootDir is the local project root. Used when RootFS is nil. Kept for
	// backward compatibility with tests that create on-disk fixtures.
	RootDir string
	// AgentDir is a local path override for the agents directory.
	// When set it overrides the agents/ sub-FS derived from RootFS/RootDir.
	AgentDir string
	// SkillsDir is a local path override for the skills directory.
	// When set it overrides the skills/ sub-FS derived from RootFS/RootDir.
	SkillsDir string
	// OllamaHost is the base URL of the local Ollama server.
	// Defaults to http://127.0.0.1:11434 if empty.
	OllamaHost string
	// RuntimePlugins is the optional registry of native plugins exposed to agents.
	// Defaults to the built-in registry.
	RuntimePlugins *plugins.Registry
}

// rootFS returns the effective root FS: RootFS if set, else os.DirFS(RootDir).
func (c *Config) rootFS() fs.FS {
	if c.RootFS != nil {
		return c.RootFS
	}
	return os.DirFS(c.RootDir)
}

// agentFS returns the FS to use for reading agent specs.
func (c *Config) agentFS() fs.FS {
	if c.AgentDir != "" {
		return os.DirFS(c.AgentDir)
	}
	sub, err := fs.Sub(c.rootFS(), "agents")
	if err != nil {
		// Should never happen for valid paths; fall back to root.
		return c.rootFS()
	}
	return sub
}

// skillsFS returns the FS to use for reading skills.
func (c *Config) skillsFS() fs.FS {
	if c.SkillsDir != "" {
		return os.DirFS(c.SkillsDir)
	}
	sub, err := fs.Sub(c.rootFS(), "skills")
	if err != nil {
		return c.rootFS()
	}
	return sub
}

// agentDirLabel returns a display string for the agents directory.
func (c *Config) agentDirLabel() string {
	if c.AgentDir != "" {
		return c.AgentDir
	}
	label := c.RootLabel
	if label == "" {
		label = c.RootDir
	}
	return label + "/agents"
}

// skillsDirLabel returns a display string for the skills directory.
func (c *Config) skillsDirLabel() string {
	if c.SkillsDir != "" {
		return c.SkillsDir
	}
	label := c.RootLabel
	if label == "" {
		label = c.RootDir
	}
	return label + "/skills"
}

// RunRequest is the input to Runner.Run.
type RunRequest struct {
	// AgentID identifies the specialist to invoke.
	AgentID string
	// Task is the task text supplied by the orchestrator (from --input, --stdin,
	// or the MCP tool call payload).
	Task string
	// SkillNames is the required list of skill IDs to attach.
	// Each must be present in the agent's allowed_skills list.
	SkillNames []string
	// Format controls the rendered output: "json" (default) or "markdown".
	Format string
}

// Runner implements AgentRunner using a local Ollama server.
type Runner struct {
	cfg      Config
	rootFS   fs.FS // resolved root FS (cached from cfg)
	skillsFS fs.FS // resolved skills FS (cached from cfg)
	registry *agent.Registry
	plugins  *plugins.Registry
	ollama   *ollama.Client
}

// Ensure Runner satisfies the AgentRunner interface at compile time.
var _ AgentRunner = (*Runner)(nil)

// New creates a Runner from cfg, loads all agent specs, and returns.
// It fails fast if the agent directory cannot be read or any spec is invalid.
func New(cfg Config) (*Runner, error) {
	agentFS := cfg.agentFS()
	reg := agent.NewRegistry(agentFS)
	if err := reg.Load(); err != nil {
		return nil, fmt.Errorf("loading agents: %w", err)
	}
	pluginRegistry := cfg.RuntimePlugins
	if pluginRegistry == nil {
		pluginRegistry = defaultRuntimePlugins()
	}
	oc := ollama.NewClient(cfg.OllamaHost)
	return &Runner{
		cfg:      cfg,
		rootFS:   cfg.rootFS(),
		skillsFS: cfg.skillsFS(),
		registry: reg,
		plugins:  pluginRegistry,
		ollama:   oc,
	}, nil
}

func defaultRuntimePlugins() *plugins.Registry {
	reg := plugins.NewRegistry(kubeplugin.New())
	reg.Alias("kubectl", "kubernetes")
	return reg
}

// ListAgents implements AgentRunner.
func (r *Runner) ListAgents(_ context.Context) ([]agent.Summary, error) {
	return r.registry.List(), nil
}

// GetSpec returns the full parsed Spec for agentID.
// This is a convenience method for CLI commands that need more detail than
// the Summary returned by ListAgents.
func (r *Runner) GetSpec(_ context.Context, agentID string) (*agent.Spec, error) {
	return r.registry.Get(agentID)
}

// GetConstitution implements AgentRunner. It resolves the constitution for
// agentID and returns it with provenance metadata.
func (r *Runner) GetConstitution(_ context.Context, agentID string) (Constitution, error) {
	spec, err := r.registry.Get(agentID)
	if err != nil {
		return Constitution{}, err
	}

	text, src, err := spec.ResolveConstitution(r.rootFS)
	if err != nil {
		return Constitution{}, fmt.Errorf("resolving constitution for %s: %w", agentID, err)
	}

	c := Constitution{
		AgentID: agentID,
		Source:  src,
		Text:    text,
	}
	if src == "path" {
		c.Path = spec.ConstitutionPath
	} else if src == "legacy" {
		c.Path = "constitutions/" + agentID + ".md"
	}
	return c, nil
}

// Run implements AgentRunner end-to-end:
//
//  1. Resolve the agent spec by ID.
//  2. Validate that at least one skill is requested.
//  3. Validate each requested skill against the agent's allowed_skills.
//  4. Load the constitution (via constitution_path, inline body, or legacy path).
//  5. Load each skill's full SKILL.md content.
//  6. Assemble the Ollama prompt with progressive disclosure.
//  7. Enforce the per-agent context budget (warn if exceeded).
//  8. Apply the agent's latency_budget_ms as a context deadline if none is set.
//  9. Call Ollama.
//  10. Return a normalized RunResult with usage and provenance metadata.
func (r *Runner) Run(ctx context.Context, req RunRequest) (result.RunResult, error) {
	start := time.Now()

	// ── 1. Resolve agent spec ──────────────────────────────────────────────
	spec, err := r.registry.Get(req.AgentID)
	if err != nil {
		return result.Error(req.AgentID, "", err.Error(), time.Since(start)), nil
	}

	// ── 2. Require at least one skill ─────────────────────────────────────
	if len(req.SkillNames) == 0 {
		return r.validationFail(req.AgentID, spec.Model, start,
			"at least one skill is required; pass one or more values from allowed_skills"), nil
	}

	// ── 3. Validate skills against the agent allowlist ────────────────────
	for _, name := range req.SkillNames {
		if !spec.AllowsSkill(name) {
			return r.validationFail(req.AgentID, spec.Model, start,
				fmt.Sprintf("skill %q is not in agent %q allowed_skills: [%s]",
					name, req.AgentID, strings.Join(spec.AllowedSkills, ", "))), nil
		}
	}

	// ── 4. Load constitution ──────────────────────────────────────────────
	constitutionText, constitutionSrc, err := spec.ResolveConstitution(r.rootFS)
	if err != nil {
		return result.Error(req.AgentID, spec.Model,
			fmt.Sprintf("resolving constitution: %s", err), time.Since(start)), nil
	}

	// ── 5. Load skill content ─────────────────────────────────────────────
	skills, err := skill.LoadMany(r.skillsFS, req.SkillNames)
	if err != nil {
		return result.Error(req.AgentID, spec.Model,
			fmt.Sprintf("loading skills: %s", err), time.Since(start)), nil
	}

	// ── 6. Apply latency budget as context deadline ───────────────────────
	if spec.LatencyBudgetMS > 0 {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(
				ctx, time.Duration(spec.LatencyBudgetMS)*time.Millisecond)
			defer cancel()
		}
	}

	// ── 7. Collect bounded runtime evidence for declared tools ────────────
	evidence := collectRuntimeEvidence(ctx, r.plugins, spec, req.Task)
	task := req.Task
	if evidence.promptBlock != "" {
		task += evidence.promptBlock
	}

	// ── 8. Assemble prompt with progressive disclosure ────────────────────
	systemPrompt, userPrompt := assemblePrompt(constitutionText, skills, req.SkillNames, task)

	// ── 9. Context budget enforcement ─────────────────────────────────────
	promptSize := len(systemPrompt) + len(userPrompt)
	budgetExceeded := false
	if spec.ContextBudget > 0 {
		// Heuristic: ~4 characters per token.
		estimatedTokens := promptSize / 4
		if estimatedTokens > spec.ContextBudget {
			budgetExceeded = true
			// Truncate the system prompt to fit within budget, preserving the
			// constitution header and at least the task.
			systemPrompt = truncateToTokenBudget(systemPrompt, spec.ContextBudget-len(userPrompt)/4)
		}
	}

	// ── 10. Call Ollama ───────────────────────────────────────────────────
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
			AgentID:               req.AgentID,
			Model:                 spec.Model,
			Status:                status,
			Summary:               fmt.Sprintf("model invocation failed: %s", err),
			Findings:              []result.Finding{},
			Artifacts:             evidence.artifacts,
			SkillsUsed:            req.SkillNames,
			ConstitutionSource:    constitutionSrc,
			ContextBudget:         spec.ContextBudget,
			PromptSizeEstimate:    promptSize,
			ContextBudgetExceeded: budgetExceeded,
			Usage:                 result.Usage{DurationMS: elapsed.Milliseconds()},
		}, nil
	}

	// ── 11. Build normalized result ───────────────────────────────────────
	rawText := chatResp.Message.Content
	parsed := result.ParseAgentOutput(rawText, result.DefaultCompactMaxChars)
	artifacts := append([]result.Artifact{}, evidence.artifacts...)
	artifacts = append(artifacts, parsed.Artifacts...)
	return result.RunResult{
		AgentID:    req.AgentID,
		Model:      chatResp.Model,
		Status:     result.StatusOK,
		Summary:    parsed.Summary,
		RawOutput:  parsed.RawOutput,
		Findings:   parsed.Findings,
		Artifacts:  artifacts,
		Confidence: parsed.Confidence,
		Usage: result.Usage{
			PromptTokensEstimate:     chatResp.PromptEvalCount,
			CompletionTokensEstimate: chatResp.EvalCount,
			DurationMS:               elapsed.Milliseconds(),
		},
		SkillsUsed:            req.SkillNames,
		ConstitutionSource:    constitutionSrc,
		ContextBudget:         spec.ContextBudget,
		PromptSizeEstimate:    promptSize,
		ContextBudgetExceeded: budgetExceeded,
	}, nil
}

// validationFail constructs a validation_fail result.
func (r *Runner) validationFail(agentID, model string, start time.Time, msg string) result.RunResult {
	return result.RunResult{
		AgentID:   agentID,
		Model:     model,
		Status:    result.StatusValidationFail,
		Summary:   msg,
		Findings:  []result.Finding{},
		Artifacts: []result.Artifact{},
		Usage:     result.Usage{DurationMS: time.Since(start).Milliseconds()},
	}
}
