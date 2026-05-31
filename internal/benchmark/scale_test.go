package benchmark

import (
	"os"
	"path/filepath"
	"strings"
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
	if report.Showcase.ScenarioID != "todo-spa-build" {
		t.Fatalf("showcase scenario = %q, want todo-spa-build", report.Showcase.ScenarioID)
	}
	if report.Showcase.WithoutInputTokens != 6191 || report.Showcase.WithInputTokens != 363 {
		t.Fatalf("showcase tokens without=%d with=%d, want 6191/363",
			report.Showcase.WithoutInputTokens, report.Showcase.WithInputTokens)
	}
	if report.Showcase.TasksPerDay != 20 || report.Showcase.TasksPerMonth != 400 || report.Showcase.TasksPerYear != 4800 {
		t.Fatalf("showcase workload = %d/%d/%d, want 20/400/4800",
			report.Showcase.TasksPerDay, report.Showcase.TasksPerMonth, report.Showcase.TasksPerYear)
	}
	byModel := make(map[string]ModelShowcaseRow, len(report.ModelShowcase))
	for _, row := range report.ModelShowcase {
		byModel[row.Model] = row
	}
	gpt54 := byModel["gpt-5.4"]
	gpt55 := byModel["gpt-5.5"]
	sonnet := byModel["claude-sonnet-4.6"]
	if gpt54.WithoutPrismUSD != 0.0276 || gpt54.WithPrismUSD != 0.0170 || gpt54.SavedPerTaskUSD != 0.0107 {
		t.Errorf("gpt-5.4 costs = %.4f/%.4f saved %.4f, want 0.0276/0.0170 saved 0.0107",
			gpt54.WithoutPrismUSD, gpt54.WithPrismUSD, gpt54.SavedPerTaskUSD)
	}
	if gpt55.SavedPerTaskUSD != 0.0213 || gpt55.WithoutPerMonthUSD != 22.12 || gpt55.WithPerMonthUSD != 13.6 {
		t.Errorf("gpt-5.5 task=%.4f mo without/with=%.2f/%.2f, want 0.0213 saved, 22.12/13.60 mo",
			gpt55.SavedPerTaskUSD, gpt55.WithoutPerMonthUSD, gpt55.WithPerMonthUSD)
	}
	if gpt54.WithoutPerYearUSD != 132.48 || gpt54.WithPerYearUSD != 81.6 {
		t.Errorf("gpt-5.4 yearly without/with = %.2f/%.2f, want 132.48/81.60",
			gpt54.WithoutPerYearUSD, gpt54.WithPerYearUSD)
	}
	if gpt55.SavedPerTaskUSD <= gpt54.SavedPerTaskUSD {
		t.Errorf("gpt-5.5 saved/task %.4f should exceed gpt-5.4 %.4f", gpt55.SavedPerTaskUSD, gpt54.SavedPerTaskUSD)
	}
	if sonnet.SavedPerTaskUSD >= gpt55.SavedPerTaskUSD {
		t.Errorf("sonnet saved/task %.4f should be below gpt-5.5 %.4f", sonnet.SavedPerTaskUSD, gpt55.SavedPerTaskUSD)
	}
	if gpt54.SavedPerTaskUSD == gpt55.SavedPerTaskUSD {
		t.Errorf("per-task savings should differ between gpt-5.4 and gpt-5.5")
	}
}

func TestWriteShowcaseDocs(t *testing.T) {
	root := repoRoot(t)
	report, err := WriteShowcaseDocs(root)
	if err != nil {
		t.Fatal(err)
	}
	showcase := FormatShowcaseMarkdown(report)
	if !strings.Contains(showcase, "6,191 in / 811 out") || !strings.Contains(showcase, "363 in / 1,072 out") {
		t.Fatalf("showcase missing live token counts:\n%s", showcase)
	}

	livePath := filepath.Join(root, "testdata", "benchmarks", "scenarios", "todo-spa-build", "live-results.yaml")
	data, err := os.ReadFile(livePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "input_tokens: 6191") {
		t.Fatalf("live-results.yaml missing committed token counts")
	}

	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), showcaseMarkerStart) {
		t.Fatal("README missing showcase markers")
	}
}
