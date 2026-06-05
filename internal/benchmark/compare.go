package benchmark

import (
	"context"
	"fmt"
	"strings"
)

// ComparisonReport is the full benchmark output comparing both modes.
type ComparisonReport struct {
	ScenarioID              string             `json:"scenario_id"`
	ScenarioTitle           string             `json:"scenario_title"`
	Baseline                string             `json:"baseline"`
	Delegated               string             `json:"delegated"`
	OrchestratorOnly        ModeResult         `json:"orchestrator_only"`
	PrismDelegated          ModeResult         `json:"prism_delegated"`
	TokenReductionPercent   float64            `json:"token_reduction_percent"`
	NetCostSavingsUSD       float64            `json:"net_cost_savings_usd"`
	WallClockDeltaMS        int64              `json:"wall_clock_delta_ms"`
	PassRate                map[string]float64 `json:"pass_rate"`
	LatencyBudgetCompliance float64            `json:"latency_budget_compliance"`
	Markdown                string             `json:"-"`
}

// Compare runs orchestrator-only and prism-delegated modes for one scenario.
func Compare(ctx context.Context, root, scenarioID string, opts RunOptions) (ComparisonReport, error) {
	scenario, err := LoadScenario(root, scenarioID)
	if err != nil {
		return ComparisonReport{}, err
	}
	rates, err := LoadRates(root)
	if err != nil {
		return ComparisonReport{}, err
	}

	orch, err := RunOrchestratorOnly(ctx, root, scenario, rates, opts)
	if err != nil {
		return ComparisonReport{}, fmt.Errorf("orchestrator_only: %w", err)
	}
	del, err := RunPrismDelegated(ctx, root, scenario, rates, opts)
	if err != nil {
		return ComparisonReport{}, fmt.Errorf("prism_delegated: %w", err)
	}

	cr := ComparisonReport{
		ScenarioID:       scenario.ID,
		ScenarioTitle:    scenario.Title,
		Baseline:         string(ModeOrchestratorOnly),
		Delegated:        string(ModePrismDelegated),
		OrchestratorOnly: orch,
		PrismDelegated:   del,
	}

	if orch.OrchestratorInputTokens > 0 {
		saved := orch.OrchestratorInputTokens - del.OrchestratorInputTokens
		cr.TokenReductionPercent = float64(saved) / float64(orch.OrchestratorInputTokens) * 100
	} else if orch.OrchestratorTokens > 0 {
		saved := orch.OrchestratorTokens - del.OrchestratorTokens
		cr.TokenReductionPercent = float64(saved) / float64(orch.OrchestratorTokens) * 100
	}

	orchTotal := orch.OrchestratorCostUSD
	delTotal := del.OrchestratorCostUSD + del.LocalCostUSD
	cr.NetCostSavingsUSD = orchTotal - delTotal
	cr.WallClockDeltaMS = del.WallClockMS - orch.WallClockMS

	cr.PassRate = map[string]float64{
		string(ModeOrchestratorOnly): boolRate(orch.Pass),
		string(ModePrismDelegated):   boolRate(del.Pass),
	}
	if del.LatencyCompliant {
		cr.LatencyBudgetCompliance = 1.0
	} else {
		cr.LatencyBudgetCompliance = 0.0
	}

	cr.Markdown = formatMarkdown(cr, scenario)
	return cr, nil
}

func boolRate(pass bool) float64 {
	if pass {
		return 1.0
	}
	return 0.0
}

func formatMarkdown(cr ComparisonReport, s *Scenario) string {
	var b strings.Builder
	b.WriteString("# Prism benchmark: ")
	b.WriteString(s.Title)
	b.WriteString("\n\n")
	b.WriteString("**Scenario ID:** `")
	b.WriteString(s.ID)
	b.WriteString("`\n\n")
	b.WriteString(s.Description)
	b.WriteString("\n\n")

	b.WriteString("## Summary\n\n")
	b.WriteString("| Metric | Orchestrator only (no MCP) | Prism delegated (MCP) |\n")
	b.WriteString("|--------|---------------------------|------------------------|\n")
	b.WriteString(fmt.Sprintf("| Orchestrator input tokens | %d | %d |\n", cr.OrchestratorOnly.OrchestratorInputTokens, cr.PrismDelegated.OrchestratorInputTokens))
	b.WriteString(fmt.Sprintf("| Orchestrator output tokens | %d | %d |\n", cr.OrchestratorOnly.OrchestratorOutputTokens, cr.PrismDelegated.OrchestratorOutputTokens))
	b.WriteString(fmt.Sprintf("| Orchestrator tokens (total) | %d | %d |\n", cr.OrchestratorOnly.OrchestratorTokens, cr.PrismDelegated.OrchestratorTokens))
	b.WriteString(fmt.Sprintf("| Local (Ollama) tokens | %d | %d |\n", cr.OrchestratorOnly.LocalTokens, cr.PrismDelegated.LocalTokens))
	b.WriteString(fmt.Sprintf("| Est. orchestrator cost (USD) | %.4f | %.4f |\n", cr.OrchestratorOnly.OrchestratorCostUSD, cr.PrismDelegated.OrchestratorCostUSD))
	b.WriteString(fmt.Sprintf("| Est. local cost (USD) | %.4f | %.4f |\n", cr.OrchestratorOnly.LocalCostUSD, cr.PrismDelegated.LocalCostUSD))
	b.WriteString(fmt.Sprintf("| Wall clock (ms) | %d | %d |\n", cr.OrchestratorOnly.WallClockMS, cr.PrismDelegated.WallClockMS))
	b.WriteString(fmt.Sprintf("| Pass | %v | %v |\n", cr.OrchestratorOnly.Pass, cr.PrismDelegated.Pass))
	b.WriteString(fmt.Sprintf("| Delegations | 0 | %d |\n\n", cr.PrismDelegated.DelegationCount))

	b.WriteString(fmt.Sprintf("**Orchestrator input token reduction:** %.1f%%\n\n", cr.TokenReductionPercent))
	b.WriteString(fmt.Sprintf("**Net cost delta (baseline − delegated total):** $%.4f\n\n", cr.NetCostSavingsUSD))
	b.WriteString(fmt.Sprintf("**Wall clock delta (delegated − baseline):** %d ms ", cr.WallClockDeltaMS))
	if cr.WallClockDeltaMS > 0 {
		b.WriteString("(delegated slower — expected when adding eight local calls)\n\n")
	} else {
		b.WriteString("(delegated faster)\n\n")
	}

	b.WriteString("## Interpretation\n\n")
	b.WriteString("- **Without MCP:** the orchestrator ingests all evidence, all eight skill bodies, and all constitutions in one context, then writes the report.\n")
	b.WriteString("- **With MCP:** the orchestrator reads a short brief, receives eight narrow specialist summaries, then synthesizes — local models absorb skill/constitution scope.\n\n")

	b.WriteString("## Orchestrator-only report (excerpt)\n\n")
	b.WriteString(truncate(cr.OrchestratorOnly.ReportText, 1200))
	b.WriteString("\n\n## Prism-delegated report (excerpt)\n\n")
	b.WriteString(truncate(cr.PrismDelegated.ReportText, 1200))
	b.WriteString("\n")

	return b.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n\n…"
}
