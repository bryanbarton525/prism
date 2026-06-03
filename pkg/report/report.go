// Package report exposes Prism benchmark reports for external integrations.
package report

import (
	"encoding/json"
	"fmt"

	"github.com/bryanbarton525/prism/internal/benchmark"
)

// MonthlyProjection is the stable benchmark projection shape used by Prism
// dashboards and export commands.
type MonthlyProjection = benchmark.MonthlyProjectionReport

// MonthlyProjectionJSON returns an indented JSON monthly projection report.
func MonthlyProjectionJSON(root string) ([]byte, error) {
	report, err := benchmark.ProjectMonthly(root)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(report, "", "  ")
}

// MonthlyProjectionMarkdown returns the human-readable monthly projection.
func MonthlyProjectionMarkdown(root string) (string, error) {
	report, err := benchmark.ProjectMonthly(root)
	if err != nil {
		return "", err
	}
	if report.Markdown == "" {
		return "", fmt.Errorf("monthly projection did not include markdown output")
	}
	return report.Markdown, nil
}

// MonthlyProjectionReport returns the structured monthly projection.
func MonthlyProjectionReport(root string) (MonthlyProjection, error) {
	return benchmark.ProjectMonthly(root)
}
