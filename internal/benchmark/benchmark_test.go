//go:build live

package benchmark

import (
	"context"
	"testing"
	"time"
)

func TestHomelabReleaseIncident(t *testing.T) {
	root := repoRoot(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	host := "http://127.0.0.1:11434"
	if err := OllamaReachable(ctx, host); err != nil {
		t.Skipf("skipping live benchmark: Ollama not reachable: %v", err)
	}
	if err := ensureModel(ctx, host, defaultBenchmarkModel); err != nil {
		t.Skipf("skipping live benchmark: %v", err)
	}

	report, err := Compare(ctx, root, "homelab-release-incident", RunOptions{
		Mock:       false,
		OllamaHost: host,
	})
	if err != nil {
		t.Fatal(err)
	}

	scenario, err := LoadScenario(root, "homelab-release-incident")
	if err != nil {
		t.Fatal(err)
	}
	phrases := scenario.Assertions.RequiredPhrases

	if missing := MissingAssertions(report.OrchestratorOnly.ReportText, phrases); len(missing) > 0 {
		t.Errorf("orchestrator_only missing phrases: %v\n%s", missing, truncate(report.OrchestratorOnly.ReportText, 800))
	}
	if missing := MissingAssertions(report.PrismDelegated.ReportText, phrases); len(missing) > 0 {
		t.Errorf("prism_delegated missing phrases: %v\n%s", missing, truncate(report.PrismDelegated.ReportText, 800))
	}
	if report.PrismDelegated.DelegationCount != 8 {
		t.Fatalf("delegations: want 8, got %d", report.PrismDelegated.DelegationCount)
	}
	if report.OrchestratorOnly.WallClockMS <= 0 {
		t.Error("orchestrator_only: expected measured wall clock")
	}
	if report.PrismDelegated.WallClockMS <= 0 {
		t.Error("prism_delegated: expected measured wall clock")
	}

	t.Logf("orchestrator_only: input_tokens=%d output_tokens=%d wall_ms=%d pass=%v",
		report.OrchestratorOnly.OrchestratorInputTokens, report.OrchestratorOnly.OrchestratorOutputTokens,
		report.OrchestratorOnly.WallClockMS, report.OrchestratorOnly.Pass)
	t.Logf("prism_delegated: orch_input=%d orch_output=%d local_tokens=%d wall_ms=%d pass=%v latency_ok=%v",
		report.PrismDelegated.OrchestratorInputTokens, report.PrismDelegated.OrchestratorOutputTokens,
		report.PrismDelegated.LocalTokens,
		report.PrismDelegated.WallClockMS, report.PrismDelegated.Pass, report.PrismDelegated.LatencyCompliant)
	t.Logf("input_token_reduction=%.1f%% cost_savings=$%.4f wall_clock_delta_ms=%d",
		report.TokenReductionPercent, report.NetCostSavingsUSD, report.WallClockDeltaMS)
}
