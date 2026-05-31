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
	wantDel := CostUSD(ref.PrismDelegated.InputTokens, ref.PrismDelegated.OutputTokens, rates.Orchestrator)
	if del != wantDel {
		t.Errorf("delegated cost should stay flat at %f: got %f", wantDel, del)
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
	if len(report.ModelShowcase) < 5 {
		t.Fatalf("expected >=5 model showcase rows, got %d", len(report.ModelShowcase))
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

func TestLoadOrchestratorModelProfiles(t *testing.T) {
	root := repoRoot(t)
	models, err := LoadOrchestratorModelProfiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) < 5 {
		t.Fatalf("expected >=5 model profiles, got %d", len(models))
	}
	want := map[string]bool{
		"gpt-5.4":           false,
		"gpt-5.5":           false,
		"claude-opus-4.7":   false,
		"claude-opus-4.6":   false,
		"claude-sonnet-4.6": false,
	}
	for _, m := range models {
		if _, ok := want[m.ID]; ok {
			want[m.ID] = true
		}
	}
	for id, seen := range want {
		if !seen {
			t.Errorf("missing model profile %q", id)
		}
	}
}

func TestModelShowcaseDifferentiatedRates(t *testing.T) {
	root := repoRoot(t)
	report, err := ProjectMonthly(root)
	if err != nil {
		t.Fatal(err)
	}
	byModel := make(map[string]ModelShowcaseRow, len(report.ModelShowcase))
	for _, row := range report.ModelShowcase {
		byModel[row.Model] = row
	}
	gpt54 := byModel["gpt-5.4"]
	gpt55 := byModel["gpt-5.5"]
	sonnet := byModel["claude-sonnet-4.6"]
	if gpt55.MonthlySavingsUSD <= gpt54.MonthlySavingsUSD {
		t.Errorf("gpt-5.5 monthly %.2f should exceed gpt-5.4 %.2f", gpt55.MonthlySavingsUSD, gpt54.MonthlySavingsUSD)
	}
	if sonnet.MonthlySavingsUSD >= gpt55.MonthlySavingsUSD {
		t.Errorf("sonnet monthly %.2f should be below gpt-5.5 %.2f", sonnet.MonthlySavingsUSD, gpt55.MonthlySavingsUSD)
	}
	if gpt54.IncidentSavingsUSD == gpt55.IncidentSavingsUSD {
		t.Errorf("per-run incident savings should differ between gpt-5.4 and gpt-5.5")
	}
}
