//go:build mock

package benchmark

import (
	"context"
	"testing"
)

func TestHomelabReleaseIncident_Mock(t *testing.T) {
	root := repoRoot(t)
	ctx := context.Background()

	report, err := Compare(ctx, root, "homelab-release-incident", RunOptions{
		Mock:          true,
		MockPerCallMS: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OrchestratorOnly.Pass || !report.PrismDelegated.Pass {
		t.Fatal("mock scenario failed assertions")
	}
	t.Logf("token_reduction_percent=%.1f%%", report.TokenReductionPercent)
	t.Logf("net_cost_savings_usd=$%.4f", report.NetCostSavingsUSD)
	t.Logf("orchestrator_only_input=%d output=%d", report.OrchestratorOnly.OrchestratorInputTokens, report.OrchestratorOnly.OrchestratorOutputTokens)
	t.Logf("prism_delegated_input=%d output=%d local_tokens=%d", report.PrismDelegated.OrchestratorInputTokens, report.PrismDelegated.OrchestratorOutputTokens, report.PrismDelegated.LocalTokens)
	if report.TokenReductionPercent < 35 {
		t.Errorf("token_reduction_percent=%.1f below 35", report.TokenReductionPercent)
	}
}
