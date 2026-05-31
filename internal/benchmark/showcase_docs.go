package benchmark

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	showcaseMarkerStart = "<!-- benchmark-showcase:start -->"
	showcaseMarkerEnd   = "<!-- benchmark-showcase:end -->"
)

// FormatShowcaseMarkdown renders the headline with-vs-without cost matrix from live results.
func FormatShowcaseMarkdown(r MonthlyProjectionReport) string {
	if len(r.ModelShowcase) == 0 || r.Showcase.ScenarioID == "" {
		return ""
	}

	s := r.Showcase
	var b strings.Builder
	b.WriteString(fmt.Sprintf("**1 engineer · 1 task/day · 30-day month · 365-day year** (`%s` live benchmark, %s)\n\n", s.ScenarioID, loadScenarioMeasuredAt(s.ScenarioID)))
	b.WriteString(fmt.Sprintf("Orchestrator tokens per task: **without Prism** `%s in / %s out` → **with Prism** `%s in / %s out` (**%.1f%% input reduction**)\n\n",
		formatInt(s.WithoutInputTokens), formatInt(s.WithoutOutputTokens),
		formatInt(s.WithInputTokens), formatInt(s.WithOutputTokens),
		s.InputReductionPercent))

	b.WriteString("| Model | Without ($/task) | With ($/task) | Daily (without / with) | Monthly (without / with) | Yearly (without / with) |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|\n")
	for _, m := range r.ModelShowcase {
		b.WriteString(fmt.Sprintf("| `%s` | $%.4f | $%.4f | $%.4f / $%.4f | $%.2f / $%.2f | $%.2f / $%.2f |\n",
			m.Model,
			m.WithoutPrismUSD, m.WithPrismUSD,
			m.WithoutPerDayUSD, m.WithPerDayUSD,
			m.WithoutPerMonthUSD, m.WithPerMonthUSD,
			m.WithoutPerYearUSD, m.WithPerYearUSD))
	}
	b.WriteString("\n")
	b.WriteString("Pricing: [OpenAI](https://openai.com/api/pricing/) · [Anthropic](https://www.anthropic.com/pricing) · rates in `testdata/benchmarks/orchestrator-models.yaml`. ")
	b.WriteString("Token counts: `testdata/benchmarks/results.yaml`. Regenerate: `prism benchmark project --write`.\n")

	return b.String()
}

func loadScenarioMeasuredAt(scenarioID string) string {
	root, err := FindRepoRoot()
	if err != nil {
		return "unknown"
	}
	results, err := LoadResults(root)
	if err != nil {
		return "unknown"
	}
	if s, ok := results[scenarioID]; ok && s.MeasuredAt != "" {
		return s.MeasuredAt
	}
	return "unknown"
}

// WriteShowcaseDocs regenerates committed showcase docs from results.yaml.
func WriteShowcaseDocs(root string) (MonthlyProjectionReport, error) {
	report, err := ProjectMonthly(root)
	if err != nil {
		return MonthlyProjectionReport{}, err
	}

	showcase := FormatShowcaseMarkdown(report)
	if showcase == "" {
		return report, fmt.Errorf("no showcase data to write")
	}

	if err := patchFileBetweenMarkers(filepath.Join(root, "README.md"), showcaseMarkerStart, showcaseMarkerEnd, showcase); err != nil {
		return report, err
	}
	if err := patchFileBetweenMarkers(filepath.Join(root, "docs", "benchmark-scale.md"), showcaseMarkerStart, showcaseMarkerEnd, showcase); err != nil {
		return report, err
	}

	standalone := "# Benchmark showcase (generated)\n\nDo not edit by hand. Regenerate with `prism benchmark project --write`.\n\n" + showcase
	if err := os.WriteFile(filepath.Join(root, "docs", "benchmark-showcase.md"), []byte(standalone), 0o644); err != nil {
		return report, err
	}

	if err := syncTodoScenarioLiveResults(root); err != nil {
		return report, err
	}

	return report, nil
}

func patchFileBetweenMarkers(path, start, end, replacement string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(data)
	i := strings.Index(content, start)
	j := strings.Index(content, end)
	if i < 0 || j < 0 || j <= i {
		return fmt.Errorf("%s: missing showcase markers", path)
	}
	i += len(start)
	var b strings.Builder
	b.WriteString(content[:i])
	b.WriteString("\n")
	b.WriteString(strings.TrimSpace(replacement))
	b.WriteString("\n\n")
	b.WriteString(content[j:])
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func syncTodoScenarioLiveResults(root string) error {
	results, err := LoadResults(root)
	if err != nil {
		return err
	}
	ref, ok := results["todo-spa-build"]
	if !ok {
		return fmt.Errorf("missing todo-spa-build in results.yaml")
	}

	out := map[string]ScenarioResults{"todo-spa-build": ref}
	data, err := yaml.Marshal(out)
	if err != nil {
		return err
	}

	header := "# Live benchmark results for this scenario\n\n" +
		"Generated from `testdata/benchmarks/results.yaml` by `prism benchmark project --write`.\n\n" +
		"Re-run live: `prism benchmark run todo-spa-build --json`\n\n"
	path := filepath.Join(root, "testdata", "benchmarks", "scenarios", "todo-spa-build", "live-results.yaml")
	return os.WriteFile(path, append([]byte(header), data...), 0o644)
}
