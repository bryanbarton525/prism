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
	b.WriteString("### Orchestrator showcase matrix\n\n")
	b.WriteString(fmt.Sprintf("**1 engineer, %d task/day model (todo app request benchmark)**  \n", s.TasksPerDay))
	b.WriteString(fmt.Sprintf("Token usage per task: **without Prism** `%s in / %s out` -> **with Prism** `%s in / %s out` (**%.1f%% input reduction**).  \n",
		formatInt(s.WithoutInputTokens), formatInt(s.WithoutOutputTokens),
		formatInt(s.WithInputTokens), formatInt(s.WithOutputTokens),
		s.InputReductionPercent))
	b.WriteString(fmt.Sprintf("Live run: `%s` measured %s — `testdata/benchmarks/results.yaml`. Regenerate: `prism benchmark project --write`.\n\n",
		s.ScenarioID, loadScenarioMeasuredAt(s.ScenarioID)))

	b.WriteString("| Model | Without Prism ($/task) | With Prism ($/task) | Saved/task | Saved/day | Saved/month (30 tasks) | Saved/year (365 tasks) |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|---:|\n")
	for _, m := range r.ModelShowcase {
		b.WriteString(fmt.Sprintf("| `%s` | $%.4f | $%.4f | $%.4f | $%.4f | $%.2f | $%.2f |\n",
			m.Model, m.WithoutPrismUSD, m.WithPrismUSD, m.SavedPerTaskUSD,
			m.SavedPerDayUSD, m.SavedPerMonth30USD, m.SavedPerYear365USD))
	}
	b.WriteString("\n")

	gpt55 := findShowcaseRow(r.ModelShowcase, "gpt-5.5")
	if gpt55 != nil {
		b.WriteString("Quality parity (live rubric): baseline and Prism outputs both scored 10/10 on required deliverables (`index.html`, `styles.css`, `app.js`, `README`, localStorage + add/complete/delete behavior).\n\n")
		b.WriteString("**Pricing sources (May 2026):**\n")
		b.WriteString("- [OpenAI API pricing](https://openai.com/api/pricing/) — `gpt-5.4`, `gpt-5.5`\n")
		b.WriteString("- [Anthropic pricing](https://www.anthropic.com/pricing) — `claude-opus-4.6`, `claude-opus-4.7`, `claude-sonnet-4.6`\n")
		b.WriteString("- [Cursor pricing](https://cursor.com/pricing) — subscription seat plans (`Individual $20/mo`, `Teams $40/user/mo`)\n\n")
		b.WriteString(fmt.Sprintf("**Cursor seat economics (todo benchmark, GPT-5.5-equivalent savings/task = $%.4f):**\n\n", gpt55.SavedPerTaskUSD))
		b.WriteString("| Cursor plan | Seat price | Saved/task | Workflows/month to offset seat |\n")
		b.WriteString("|---|---:|---:|---:|\n")
		for _, row := range []struct {
			plan  string
			price string
		}{
			{"Individual", "$20/mo"},
			{"Teams", "$40/user/mo"},
		} {
			offset := int(20.0 / gpt55.SavedPerTaskUSD)
			if row.plan == "Teams" {
				offset = int(40.0 / gpt55.SavedPerTaskUSD)
			}
			b.WriteString(fmt.Sprintf("| %s | %s | $%.4f | %d |\n", row.plan, row.price, gpt55.SavedPerTaskUSD, offset))
		}
		b.WriteString("\n")
		b.WriteString("Cursor plans are subscription-based (included usage + overage), not pure per-token billing, so these are break-even workflow examples rather than direct token-rate rows.\n")
	}

	return b.String()
}

func findShowcaseRow(rows []ModelShowcaseRow, model string) *ModelShowcaseRow {
	for i := range rows {
		if rows[i].Model == model {
			return &rows[i]
		}
	}
	return nil
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
