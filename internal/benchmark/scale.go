package benchmark

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ScenarioResults holds committed per-run metrics from results.yaml.
type ScenarioResults struct {
	MeasuredAt                  string  `yaml:"measured_at"`
	ModelLocal                  string  `yaml:"model_local"`
	ModelOrchestrator           string  `yaml:"model_orchestrator"`
	OrchestratorOnly            RunSnapshot `yaml:"orchestrator_only"`
	PrismDelegated              RunSnapshot `yaml:"prism_delegated"`
	InputTokenReductionPercent  float64 `yaml:"input_token_reduction_percent"`
	NetSavingsUSD               float64 `yaml:"net_savings_usd"`
}

// RunSnapshot is one mode's committed metrics.
type RunSnapshot struct {
	InputTokens  int     `yaml:"input_tokens"`
	OutputTokens int     `yaml:"output_tokens"`
	TotalTokens  int     `yaml:"total_tokens"`
	LocalTokens  int     `yaml:"local_tokens"`
	CostUSD      float64 `yaml:"cost_usd"`
	WallClockMS  int64   `yaml:"wall_clock_ms"`
	Pass         bool    `yaml:"pass"`
	Delegations  int     `yaml:"delegations"`
}

// ScaleProfiles configures monthly projection inputs.
type ScaleProfiles struct {
	Profiles map[string]UsageProfile `yaml:"profiles"`
	ReferenceScenarios struct {
		Incident        string `yaml:"incident"`
		IncidentAtScale string `yaml:"incident_at_scale"`
		Codegen         string `yaml:"codegen"`
	} `yaml:"reference_scenarios"`
	AtScaleThresholdMultiplier float64 `yaml:"at_scale_threshold_multiplier"`
}

// UsageProfile describes monthly task volume for one team shape.
type UsageProfile struct {
	Description             string  `yaml:"description"`
	IncidentsPerMonth       int     `yaml:"incidents_per_month"`
	CodegenTasksPerMonth    int     `yaml:"codegen_tasks_per_month"`
	ContextSizeMultiplier   float64 `yaml:"context_size_multiplier"`
}

// ProfileProjection is monthly cost/savings for one usage profile.
type ProfileProjection struct {
	ProfileID              string  `json:"profile_id"`
	Description            string  `json:"description"`
	IncidentsPerMonth      int     `json:"incidents_per_month"`
	CodegenTasksPerMonth   int     `json:"codegen_tasks_per_month"`
	ContextSizeMultiplier  float64 `json:"context_size_multiplier"`
	IncidentScenario       string  `json:"incident_scenario"`
	SavingsPerIncidentUSD  float64 `json:"savings_per_incident_usd"`
	SavingsPerCodegenUSD   float64 `json:"savings_per_codegen_usd"`
	MonthlyBaselineUSD     float64 `json:"monthly_baseline_usd"`
	MonthlyDelegatedUSD    float64 `json:"monthly_delegated_usd"`
	MonthlyNetSavingsUSD   float64 `json:"monthly_net_savings_usd"`
	AnnualNetSavingsUSD    float64 `json:"annual_net_savings_usd"`
	OrchestratorInputSaved int     `json:"orchestrator_input_tokens_saved_per_month"`
}

// MonthlyProjectionReport aggregates all profiles.
type MonthlyProjectionReport struct {
	RatesModel       string              `json:"rates_model"`
	Profiles         []ProfileProjection `json:"profiles"`
	Markdown         string              `json:"-"`
}

// LoadResults reads testdata/benchmarks/results.yaml.
func LoadResults(root string) (map[string]ScenarioResults, error) {
	path := filepath.Join(root, "testdata", "benchmarks", "results.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw map[string]ScenarioResults
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// LoadScaleProfiles reads testdata/benchmarks/scale-profiles.yaml.
func LoadScaleProfiles(root string) (ScaleProfiles, error) {
	path := filepath.Join(root, "testdata", "benchmarks", "scale-profiles.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return ScaleProfiles{}, err
	}
	var sp ScaleProfiles
	if err := yaml.Unmarshal(data, &sp); err != nil {
		return ScaleProfiles{}, err
	}
	return sp, nil
}

// ScaledSavingsPerRun adjusts committed savings for a context multiplier.
// Baseline orchestrator input scales linearly; delegated orchestrator input stays near flat.
func ScaledSavingsPerRun(ref ScenarioResults, rates Rates, multiplier float64) (baselineUSD, delegatedUSD, savingsUSD float64, inputSaved int) {
	if multiplier <= 0 {
		multiplier = 1
	}
	scaledIn := float64(ref.OrchestratorOnly.InputTokens) * multiplier
	baselineUSD = CostUSD(int(scaledIn), ref.OrchestratorOnly.OutputTokens, rates.Orchestrator)
	delegatedUSD = ref.PrismDelegated.CostUSD
	if delegatedUSD == 0 {
		delegatedUSD = CostUSD(ref.PrismDelegated.InputTokens, ref.PrismDelegated.OutputTokens, rates.Orchestrator)
	}
	savingsUSD = roundUSD(baselineUSD - delegatedUSD)
	inputSaved = int(scaledIn) - ref.PrismDelegated.InputTokens
	if inputSaved < 0 {
		inputSaved = 0
	}
	return baselineUSD, delegatedUSD, savingsUSD, inputSaved
}

// ProjectMonthly builds a savings report from committed results and usage profiles.
func ProjectMonthly(root string) (MonthlyProjectionReport, error) {
	results, err := LoadResults(root)
	if err != nil {
		return MonthlyProjectionReport{}, err
	}
	profiles, err := LoadScaleProfiles(root)
	if err != nil {
		return MonthlyProjectionReport{}, err
	}
	rates, err := LoadRates(root)
	if err != nil {
		return MonthlyProjectionReport{}, err
	}

	report := MonthlyProjectionReport{RatesModel: rates.Orchestrator.Model}
	threshold := profiles.AtScaleThresholdMultiplier
	if threshold <= 0 {
		threshold = 2.5
	}

	for id, prof := range profiles.Profiles {
		incidentKey := profiles.ReferenceScenarios.Incident
		if prof.ContextSizeMultiplier >= threshold {
			if atScale, ok := results[profiles.ReferenceScenarios.IncidentAtScale]; ok {
				incidentKey = profiles.ReferenceScenarios.IncidentAtScale
				_ = atScale // use below via results map
			}
		}
		incidentRef, ok := results[incidentKey]
		if !ok {
			return MonthlyProjectionReport{}, fmt.Errorf("missing results for scenario %q", incidentKey)
		}
		codegenRef, ok := results[profiles.ReferenceScenarios.Codegen]
		if !ok {
			return MonthlyProjectionReport{}, fmt.Errorf("missing results for scenario %q", profiles.ReferenceScenarios.Codegen)
		}

		incBase, incDel, incSave, incInputSaved := ScaledSavingsPerRun(incidentRef, rates, prof.ContextSizeMultiplier)
		_, _, codeSave, _ := ScaledSavingsPerRun(codegenRef, rates, prof.ContextSizeMultiplier)

		incidents := prof.IncidentsPerMonth
		codegen := prof.CodegenTasksPerMonth
		monthlyBaseline := incBase*float64(incidents) + ScaledBaselineOnly(codegenRef, rates, prof.ContextSizeMultiplier)*float64(codegen)
		monthlyDelegated := incDel*float64(incidents) + ScaledDelegatedOnly(codegenRef, rates)*float64(codegen)
		monthlySavings := incSave*float64(incidents) + codeSave*float64(codegen)
		monthlyNet := roundUSD(monthlySavings)

		report.Profiles = append(report.Profiles, ProfileProjection{
			ProfileID:              id,
			Description:            prof.Description,
			IncidentsPerMonth:      incidents,
			CodegenTasksPerMonth:   codegen,
			ContextSizeMultiplier:  prof.ContextSizeMultiplier,
			IncidentScenario:       incidentKey,
			SavingsPerIncidentUSD:  incSave,
			SavingsPerCodegenUSD:   codeSave,
			MonthlyBaselineUSD:     roundUSD(monthlyBaseline),
			MonthlyDelegatedUSD:    roundUSD(monthlyDelegated),
			MonthlyNetSavingsUSD:   monthlyNet,
			AnnualNetSavingsUSD:    roundUSD(monthlyNet * 12),
			OrchestratorInputSaved: incInputSaved*incidents + int(float64(codegenRef.OrchestratorOnly.InputTokens-codegenRef.PrismDelegated.InputTokens)*prof.ContextSizeMultiplier)*codegen,
		})
	}

	report.Markdown = formatProjectionMarkdown(report, rates, root)
	return report, nil
}

func ScaledBaselineOnly(ref ScenarioResults, rates Rates, multiplier float64) float64 {
	if multiplier <= 0 {
		multiplier = 1
	}
	scaledIn := int(float64(ref.OrchestratorOnly.InputTokens) * multiplier)
	return CostUSD(scaledIn, ref.OrchestratorOnly.OutputTokens, rates.Orchestrator)
}

func ScaledDelegatedOnly(ref ScenarioResults, rates Rates) float64 {
	if ref.PrismDelegated.CostUSD > 0 {
		return ref.PrismDelegated.CostUSD
	}
	return CostUSD(ref.PrismDelegated.InputTokens, ref.PrismDelegated.OutputTokens, rates.Orchestrator)
}

func roundUSD(v float64) float64 {
	return math.Round(v*100) / 100
}

func formatProjectionMarkdown(r MonthlyProjectionReport, rates Rates, root string) string {
	var b strings.Builder
	b.WriteString("# Prism monthly savings projection\n\n")
	b.WriteString(fmt.Sprintf("Orchestrator pricing: **%s** ($%.2f/M input, $%.2f/M output). Local Ollama: **$0**.\n\n",
		rates.Orchestrator.Model, rates.Orchestrator.InputPerMillion, rates.Orchestrator.OutputPerMillion))
	b.WriteString("Based on committed benchmark results in `testdata/benchmarks/results.yaml`.\n\n")

	b.WriteString("## Per-run reference (committed)\n\n")
	b.WriteString("| Scenario | Baseline input | Delegated input | Input reduction | Savings/run |\n")
	b.WriteString("|----------|----------------|-----------------|-----------------|-------------|\n")
	if committed, err := LoadResults(root); err == nil {
		for _, id := range []string{
			"homelab-release-incident",
			"homelab-release-incident-at-scale",
			"codegen-helper-task",
		} {
			if s, ok := committed[id]; ok {
				b.WriteString(fmt.Sprintf("| %s | %s | %s | %.1f%% | $%.4f |\n",
					id, formatInt(s.OrchestratorOnly.InputTokens), formatInt(s.PrismDelegated.InputTokens),
					s.InputTokenReductionPercent, s.NetSavingsUSD))
			}
		}
	}
	b.WriteString("\n")

	b.WriteString("## Monthly profiles\n\n")
	b.WriteString("| Profile | Incidents/mo | Codegen/mo | Context scale | Monthly savings | Annual savings |\n")
	b.WriteString("|---------|--------------|------------|---------------|-----------------|----------------|\n")
	for _, p := range r.Profiles {
		b.WriteString(fmt.Sprintf("| **%s** | %d | %d | %.1fx | **$%.2f** | $%.2f |\n",
			p.ProfileID, p.IncidentsPerMonth, p.CodegenTasksPerMonth, p.ContextSizeMultiplier,
			p.MonthlyNetSavingsUSD, p.AnnualNetSavingsUSD))
	}
	b.WriteString("\n")

	for _, p := range r.Profiles {
		b.WriteString(fmt.Sprintf("### %s\n\n", p.ProfileID))
		b.WriteString(p.Description)
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("- Incident scenario: `%s` ($%.4f saved/incident)\n", p.IncidentScenario, p.SavingsPerIncidentUSD))
		b.WriteString(fmt.Sprintf("- Codegen scenario: `codegen-helper-task` ($%.4f saved/task)\n", p.SavingsPerCodegenUSD))
		b.WriteString(fmt.Sprintf("- Orchestrator input tokens avoided/month: ~%s\n\n", formatInt(p.OrchestratorInputSaved)))
	}

	b.WriteString("## Assumptions\n\n")
	b.WriteString("- **Without MCP:** orchestrator loads all evidence, all skills, all constitutions, plus session padding at scale.\n")
	b.WriteString("- **With MCP:** orchestrator reads a brief and compact agent summaries; Ollama runs absorb skill/constitution scope.\n")
	b.WriteString("- Re-measure with `prism benchmark run <scenario>` and update `results.yaml` for your models and rates.\n")

	return b.String()
}

func formatInt(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var out []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
}
