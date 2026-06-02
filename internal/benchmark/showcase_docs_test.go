//go:build docsgen

package benchmark

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
