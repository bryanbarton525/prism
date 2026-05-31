package benchmark

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/app"
	"github.com/bryanbarton525/prism/internal/skill"
)

// Mode identifies orchestrator-only vs Prism-delegated benchmark paths.
type Mode string

const (
	ModeOrchestratorOnly Mode = "orchestrator_only"
	ModePrismDelegated   Mode = "prism_delegated"
)

// RunOptions configures a benchmark execution. Mock defaults to false (real Ollama).
type RunOptions struct {
	Mock          bool
	MockPerCallMS int
	OllamaHost    string
	Model         string
}

// ModeResult holds metrics for one benchmark mode.
type ModeResult struct {
	Mode                     Mode    `json:"mode"`
	ScenarioID               string  `json:"scenario_id"`
	OrchestratorInputTokens  int     `json:"orchestrator_input_tokens"`
	OrchestratorOutputTokens int     `json:"orchestrator_output_tokens"`
	OrchestratorTokens       int     `json:"orchestrator_tokens"`
	LocalTokens              int     `json:"local_tokens"`
	WallClockMS              int64   `json:"wall_clock_ms"`
	OrchestratorCostUSD      float64 `json:"orchestrator_cost_usd"`
	LocalCostUSD             float64 `json:"local_cost_usd"`
	Pass                     bool    `json:"pass"`
	ReportText               string  `json:"report_text"`
	DelegationCount          int     `json:"delegation_count"`
	LatencyCompliant         bool    `json:"latency_compliant"`
}

func (opts RunOptions) model() string {
	if opts.Model != "" {
		return opts.Model
	}
	return defaultBenchmarkModel
}

func (opts RunOptions) live() bool { return !opts.Mock }

// RunOrchestratorOnly benchmarks Cursor without MCP: one large orchestrator context.
func RunOrchestratorOnly(ctx context.Context, root string, scenario *Scenario, rates Rates, opts RunOptions) (ModeResult, error) {
	brief, err := scenario.ReadBrief()
	if err != nil {
		return ModeResult{}, err
	}
	evidence, err := scenario.LoadAllEvidence()
	if err != nil {
		return ModeResult{}, err
	}
	fullContext := strings.Builder{}
	fullContext.WriteString("# Evidence bundle\n\n")
	fullContext.WriteString(evidence)
	if scenario.LoadFullLibrary() {
		fullSkills, err := loadAllSkillBodies(root)
		if err != nil {
			return ModeResult{}, err
		}
		constitutions, err := loadAllConstitutions(root)
		if err != nil {
			return ModeResult{}, err
		}
		fullContext.WriteString("\n\n# All agent constitutions\n\n")
		fullContext.WriteString(constitutions)
		fullContext.WriteString("\n\n# All skill bodies (full library loaded)\n\n")
		fullContext.WriteString(fullSkills)
	}
	if extra, err := scenario.LoadOrchestratorContext(); err != nil {
		return ModeResult{}, err
	} else if extra != "" {
		fullContext.WriteString("\n\n# Orchestrator session context (rules, history, runbooks)\n\n")
		fullContext.WriteString(extra)
	}

	synthTmpl, err := scenario.ReadTask(scenario.Synthesis.OrchestratorPromptFile)
	if err != nil {
		return ModeResult{}, err
	}
	userPrompt := strings.ReplaceAll(synthTmpl, "{{BRIEF}}", brief)
	userPrompt = strings.ReplaceAll(userPrompt, "{{FULL_CONTEXT}}", fullContext.String())

	var report string
	var inTok, outTok int
	var elapsed time.Duration

	if opts.live() {
		if err := ensureModel(ctx, opts.OllamaHost, opts.model()); err != nil {
			return ModeResult{}, err
		}
		inTok = EstimateTokens(userPrompt)
		res, err := ollamaChat(ctx, opts.OllamaHost, opts.model(),
			"You are the primary orchestrator. Write a complete incident report in Markdown. Include every required term from the user message verbatim.",
			userPrompt)
		if err != nil {
			return ModeResult{}, fmt.Errorf("orchestrator_only ollama: %w", err)
		}
		report = res.text
		outTok = res.outputTok
		if res.promptTok > 0 {
			inTok = res.promptTok
		}
		elapsed = time.Duration(res.durationMS) * time.Millisecond
	} else {
		report, err = scenario.SynthesisResponse()
		if err != nil {
			return ModeResult{}, err
		}
		inTok = EstimateTokens(userPrompt)
		outTok = EstimateTokens(report)
		elapsed = time.Duration(int64(8000+len(userPrompt)/80)) * time.Millisecond
	}

	return ModeResult{
		Mode:                     ModeOrchestratorOnly,
		ScenarioID:               scenario.ID,
		OrchestratorInputTokens:  inTok,
		OrchestratorOutputTokens: outTok,
		OrchestratorTokens:       inTok + outTok,
		LocalTokens:              0,
		WallClockMS:              elapsed.Milliseconds(),
		OrchestratorCostUSD:      CostUSD(inTok, outTok, rates.Orchestrator),
		LocalCostUSD:             0,
		Pass:                     PassesAssertions(report, scenario.Assertions.RequiredPhrases),
		ReportText:               report,
		DelegationCount:          0,
		LatencyCompliant:         true,
	}, nil
}

// RunPrismDelegated benchmarks Prism MCP: eight agent runs plus orchestrator synthesis.
func RunPrismDelegated(ctx context.Context, root string, scenario *Scenario, rates Rates, opts RunOptions) (ModeResult, error) {
	start := time.Now()

	ollamaHost := opts.OllamaHost
	if opts.Mock {
		respMap, err := skillResponseMap(scenario)
		if err != nil {
			return ModeResult{}, err
		}
		server := mockOllamaServer(respMap)
		defer server.Close()
		ollamaHost = server.URL
	} else if err := ensureModel(ctx, ollamaHost, opts.model()); err != nil {
		return ModeResult{}, err
	}

	runner, err := app.New(app.Config{
		RootDir:    root,
		OllamaHost: ollamaHost,
	})
	if err != nil {
		return ModeResult{}, err
	}

	brief, err := scenario.ReadBrief()
	if err != nil {
		return ModeResult{}, err
	}

	var localIn, localOut int
	var summaries strings.Builder
	allCompliant := true

	for _, d := range scenario.Delegations {
		task, err := d.BuildTask(scenario.Dir())
		if err != nil {
			return ModeResult{}, err
		}
		res, err := runner.Run(ctx, app.RunRequest{
			AgentID:    d.AgentID,
			Task:       task,
			SkillNames: d.SkillNames,
			Format:     "json",
		})
		if err != nil {
			return ModeResult{}, err
		}
		if res.Status != "ok" {
			return ModeResult{}, fmt.Errorf("delegation %s: status %s: %s", d.ID, res.Status, res.Summary)
		}
		if opts.Mock && opts.MockPerCallMS > 0 {
			time.Sleep(time.Duration(opts.MockPerCallMS) * time.Millisecond)
		}

		spec, _ := runner.GetSpec(ctx, d.AgentID)
		if spec != nil && spec.LatencyBudgetMS > 0 {
			if res.Usage.DurationMS > int64(spec.LatencyBudgetMS) {
				allCompliant = false
			}
		}

		localIn += res.Usage.PromptTokensEstimate
		localOut += res.Usage.CompletionTokensEstimate
		summaries.WriteString(fmt.Sprintf("### %s (%s)\n\n%s\n\n", d.ID, d.AgentID, res.Summary))
	}

	synthTmpl, err := scenario.ReadTask(scenario.Synthesis.DelegatedPromptFile)
	if err != nil {
		return ModeResult{}, err
	}
	synthUser := strings.ReplaceAll(synthTmpl, "{{BRIEF}}", brief)
	synthUser = strings.ReplaceAll(synthUser, "{{DELEGATION_SUMMARIES}}", summaries.String())

	var report string
	var orchIn, orchOut int

	if opts.live() {
		orchIn = EstimateTokens(synthUser)
		res, err := ollamaChat(ctx, ollamaHost, opts.model(),
			"You are the primary orchestrator. Synthesize the specialist summaries into one incident report in Markdown. Include every required term from the user message verbatim.",
			synthUser)
		if err != nil {
			return ModeResult{}, fmt.Errorf("delegated synthesis ollama: %w", err)
		}
		report = res.text
		orchOut = res.outputTok
		if res.promptTok > 0 {
			orchIn = res.promptTok
		}
	} else {
		report, err = scenario.SynthesisResponse()
		if err != nil {
			return ModeResult{}, err
		}
		orchIn = EstimateTokens(synthUser)
		orchOut = EstimateTokens(report)
	}

	elapsed := time.Since(start)
	if opts.Mock {
		per := int64(opts.MockPerCallMS)
		if per == 0 {
			per = 750
		}
		elapsed = time.Duration(per*int64(len(scenario.Delegations))+1500) * time.Millisecond
	}

	return ModeResult{
		Mode:                     ModePrismDelegated,
		ScenarioID:               scenario.ID,
		OrchestratorInputTokens:  orchIn,
		OrchestratorOutputTokens: orchOut,
		OrchestratorTokens:       orchIn + orchOut,
		LocalTokens:              localIn + localOut,
		WallClockMS:              elapsed.Milliseconds(),
		OrchestratorCostUSD:      CostUSD(orchIn, orchOut, rates.Orchestrator),
		LocalCostUSD:             CostUSD(localIn, localOut, rates.Local),
		Pass:                     PassesAssertions(report, scenario.Assertions.RequiredPhrases),
		ReportText:               report,
		DelegationCount:          len(scenario.Delegations),
		LatencyCompliant:         allCompliant,
	}, nil
}

func loadAllSkillBodies(root string) (string, error) {
	skills, err := skill.DiscoverAll(filepath.Join(root, "skills"))
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, sk := range skills {
		b.WriteString(sk.FullText())
		b.WriteString("\n\n")
	}
	return b.String(), nil
}

func loadAllConstitutions(root string) (string, error) {
	reg := agent.NewRegistry(filepath.Join(root, "agents"))
	if err := reg.Load(); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, sum := range reg.List() {
		spec, err := reg.Get(sum.ID)
		if err != nil {
			return "", err
		}
		text, _, err := spec.ResolveConstitution(root)
		if err != nil {
			return "", err
		}
		b.WriteString("## ")
		b.WriteString(sum.ID)
		b.WriteString("\n\n")
		b.WriteString(text)
		b.WriteString("\n\n")
	}
	return b.String(), nil
}
