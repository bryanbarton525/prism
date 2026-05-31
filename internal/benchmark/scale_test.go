package benchmark

import (
	"testing"
)

func TestScaledSavingsPerRun(t *testing.T) {
	ref := ScenarioResults{
		OrchestratorOnly: RunSnapshot{InputTokens: 4000, OutputTokens: 500, CostUSD: 0.012},
		PrismDelegated:   RunSnapshot{InputTokens: 800, OutputTokens: 500, CostUSD: 0.006},
	}
	rates := Rates{
		Orchestrator: RateModel{InputPerMillion: 2, OutputPerMillion: 8},
	}
	base, del, save, saved := ScaledSavingsPerRun(ref, rates, 2.0)
	if base <= ref.OrchestratorOnly.CostUSD {
		t.Errorf("scaled baseline should exceed base: %f", base)
	}
	if del != ref.PrismDelegated.CostUSD {
		t.Errorf("delegated cost should stay flat: got %f", del)
	}
	if save <= 0 {
		t.Errorf("expected positive savings, got %f", save)
	}
	if saved < 6000 {
		t.Errorf("expected ~6000 input tokens saved, got %d", saved)
	}
}

func TestProjectMonthly(t *testing.T) {
	root := repoRoot(t)
	report, err := ProjectMonthly(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Profiles) < 3 {
		t.Fatalf("expected 3 profiles, got %d", len(report.Profiles))
	}
	for _, p := range report.Profiles {
		if p.MonthlyNetSavingsUSD <= 0 {
			t.Errorf("profile %q: expected positive monthly savings", p.ProfileID)
		}
		if p.AnnualNetSavingsUSD < p.MonthlyNetSavingsUSD*12-0.02 {
			t.Errorf("profile %q: annual %.2f should be ~12x monthly %.2f", p.ProfileID, p.AnnualNetSavingsUSD, p.MonthlyNetSavingsUSD)
		}
	}
	// enterprise should save more than solo
	var solo, enterprise ProfileProjection
	for _, p := range report.Profiles {
		switch p.ProfileID {
		case "solo_developer":
			solo = p
		case "enterprise_sre":
			enterprise = p
		}
	}
	if enterprise.MonthlyNetSavingsUSD <= solo.MonthlyNetSavingsUSD {
		t.Errorf("enterprise savings %f should exceed solo %f", enterprise.MonthlyNetSavingsUSD, solo.MonthlyNetSavingsUSD)
	}
}
