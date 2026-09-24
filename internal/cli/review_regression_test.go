package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/bryanbarton525/prism/internal/agent/importconfig"
	"github.com/bryanbarton525/prism/internal/agent/importer"
	"github.com/bryanbarton525/prism/internal/extensions"
)

func TestReviewYesRequiresExplicitInstallSelection(t *testing.T) {
	cmd := newInstallCmd()
	cmd.SetArgs([]string{"--yes"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes requires explicit") {
		t.Fatalf("expected explicit selection error, got %v", err)
	}
}

func TestImportDecisionAppliesExplicitBudgets(t *testing.T) {
	report := importer.Report{Adapter: "claude-markdown"}
	cfg, err := importerConfigFromDecision("local", report, &importconfig.AgentDecision{ContextBudget: 4096, LatencyMS: 2500})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ContextBudget != 4096 || cfg.LatencyBudgetMS != 2500 {
		t.Fatalf("budgets were not applied: %#v", cfg)
	}
}

func TestInstallGraphifyFlagsRequireSpecificOptIn(t *testing.T) {
	err := validateInstallGraphify(installFlags{graphifyIndex: "graph.json", graphifyFingerprint: "sha"})
	if err == nil || !strings.Contains(err.Error(), "--graphify-managed") {
		t.Fatalf("expected explicit opt-in error, got %v", err)
	}
}

func TestNativeAgentImportPreservesConstitutionSupportFile(t *testing.T) {
	source := []byte("---\nid: helper\nname: Helper\ndescription: helper\nmodel: source-cloud\ncontext_budget: 100\nallowed_skills: []\nlatency_budget_ms: 1000\nconstitution_path: constitutions/helper.md\n---\n")
	files := fstest.MapFS{
		"agents/helper.md":        {Data: source},
		"constitutions/helper.md": {Data: []byte("independent constitution")},
	}
	candidate, err := identifyAgentCandidate("agents/helper.md", source, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if err := attachAgentSupportFiles(files, &candidate); err != nil {
		t.Fatal(err)
	}
	normalized, report, err := importer.Translate("agents/helper.md", source, importer.Config{DefaultModel: "local-model"})
	if err != nil {
		t.Fatal(err)
	}
	stateDir := t.TempDir()
	entries, err := extensions.NewLocalAgentService(stateDir).InstallTranslatedAgents(context.Background(), []extensions.TranslatedAgentInstall{{
		Name: "helper.md", Normalized: normalized, Source: source, SourceName: "agents/helper.md", SupportFiles: candidate.SupportFiles,
		Request: extensions.InstallLocalAgentRequest{SourceURI: "local", Provenance: extensions.Provenance{Adapter: report.Adapter}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	entry := entries[0]
	data, err := os.ReadFile(filepath.Join(entry.ObjectPath, "constitutions", "helper.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "independent constitution" {
		t.Fatalf("constitution = %q", data)
	}
}
