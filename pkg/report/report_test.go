package report

import (
	"strings"
	"testing"
)

func TestMonthlyProjectionExports(t *testing.T) {
	root := "../.."
	structured, err := MonthlyProjectionReport(root)
	if err != nil {
		t.Fatalf("MonthlyProjectionReport(): %v", err)
	}
	if len(structured.Profiles) == 0 {
		t.Fatal("expected profiles")
	}

	jsonData, err := MonthlyProjectionJSON(root)
	if err != nil {
		t.Fatalf("MonthlyProjectionJSON(): %v", err)
	}
	if !strings.Contains(string(jsonData), `"profiles"`) {
		t.Fatalf("JSON export missing profiles: %s", jsonData)
	}

	markdown, err := MonthlyProjectionMarkdown(root)
	if err != nil {
		t.Fatalf("MonthlyProjectionMarkdown(): %v", err)
	}
	if !strings.Contains(markdown, "Monthly") {
		t.Fatalf("Markdown export looks wrong: %s", markdown)
	}
}
