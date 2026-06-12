package events

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/bryanbarton525/prism/pkg/observe"
)

func TestStoreDoesNotPersistRawPromptFields(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	err = store.Append(context.Background(), observe.RunEvent{
		Timestamp:            time.Now().UTC(),
		RunID:                "run-1",
		AgentID:              "kubectl",
		Model:                "qwen3.5:9b",
		Status:               "ok",
		Skills:               []string{"k8s-rollout-diagnostics"},
		PromptTokensEstimate: 123,
	})
	if err != nil {
		t.Fatal(err)
	}
	items, err := store.List(context.Background(), ListOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("events = %d, want 1", len(items))
	}
	if items[0].Skills[0] != "k8s-rollout-diagnostics" {
		t.Fatalf("skills = %#v", items[0].Skills)
	}
}

func TestSummaryIncludesDashboardAndReportFields(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	err = store.Append(context.Background(), observe.RunEvent{
		Timestamp:             time.Now().UTC(),
		RunID:                 "graph-1",
		EventKind:             "graph",
		GraphID:               "g",
		Status:                "validation_fail",
		Plugins:               []string{"kubernetes"},
		BundleID:              "k8s-core-triage",
		BundleVersion:         "0.1.0",
		ContextBudgetExceeded: true,
		PolicyDecision:        "deny",
	})
	if err != nil {
		t.Fatal(err)
	}
	sum, err := store.Summary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sum.GraphExecutions != 1 || sum.ValidationFailures != 1 || sum.PolicyDenials != 1 {
		t.Fatalf("unexpected summary counts: %#v", sum)
	}
	if sum.TopPlugins["kubernetes"] != 1 {
		t.Fatalf("plugins = %#v", sum.TopPlugins)
	}
	if sum.BundleVersions["k8s-core-triage@0.1.0"] != 1 {
		t.Fatalf("bundles = %#v", sum.BundleVersions)
	}
}

func TestStoreIsAppendOnlyAndFiltersMetadata(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	event := observe.RunEvent{
		Timestamp: time.Now().UTC(),
		RunID:     "run-1",
		Status:    "ok",
		Metadata:  observe.Metadata{ActorID: "alice", WorkspaceID: "platform", Source: "cli"},
		AgentID:   "kubectl",
		Skills:    []string{"k8s-rollout-diagnostics"},
	}
	if err := store.Append(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if err := store.Append(context.Background(), event); err == nil {
		t.Fatal("expected duplicate run_id to fail instead of updating event history")
	}
	items, err := store.List(context.Background(), ListOptions{
		Actor:     "alice",
		Workspace: "platform",
		Skill:     "k8s-rollout-diagnostics",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].RunID != "run-1" {
		t.Fatalf("filtered items = %#v", items)
	}
}

func TestOpenMigratesPriorSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
CREATE TABLE run_events (
  run_id TEXT PRIMARY KEY,
  timestamp TEXT NOT NULL,
  graph_id TEXT,
  graph_node_id TEXT,
  actor_id TEXT,
  workspace_id TEXT,
  source TEXT,
  correlation_id TEXT,
  agent_id TEXT,
  model TEXT,
  status TEXT,
  skills_json TEXT NOT NULL,
  plugins_json TEXT NOT NULL,
  duration_ms INTEGER,
  prompt_tokens_estimate INTEGER,
  completion_tokens_estimate INTEGER,
  context_budget INTEGER,
  prompt_size_estimate INTEGER,
  context_budget_exceeded INTEGER,
  policy_decision TEXT,
  policy_reason TEXT,
  bundle_id TEXT,
  bundle_version TEXT,
  error TEXT,
  validation_error TEXT
)`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Append(context.Background(), observe.RunEvent{RunID: "run-1", EventKind: "run", Status: "ok"}); err != nil {
		t.Fatal(err)
	}
	items, err := store.List(context.Background(), ListOptions{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].EventKind != "run" {
		t.Fatalf("items = %#v", items)
	}
}
