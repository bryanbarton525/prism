package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/bryanbarton525/prism/pkg/observe"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("event store path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Store{db: db}
	if err := s.Migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS run_events (
  run_id TEXT PRIMARY KEY,
  timestamp TEXT NOT NULL,
  graph_id TEXT,
  graph_node_id TEXT,
  event_kind TEXT,
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
);
CREATE INDEX IF NOT EXISTS run_events_timestamp_idx ON run_events(timestamp);
CREATE INDEX IF NOT EXISTS run_events_agent_idx ON run_events(agent_id);
CREATE INDEX IF NOT EXISTS run_events_status_idx ON run_events(status);
`); err != nil {
		return err
	}
	if err := s.addColumnIfMissing(ctx, "run_events", "event_kind", "TEXT"); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES(1, ?)`, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) addColumnIfMissing(ctx context.Context, table, column, definition string) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
	return err
}

func (s *Store) ObserveRun(ctx context.Context, event observe.RunEvent) error {
	return s.Append(ctx, event)
}

func (s *Store) Append(ctx context.Context, event observe.RunEvent) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	skills, _ := json.Marshal(event.Skills)
	plugins, _ := json.Marshal(event.Plugins)
	_, err := s.db.ExecContext(ctx, `
INSERT INTO run_events (
  run_id, timestamp, graph_id, graph_node_id, event_kind, actor_id, workspace_id, source, correlation_id,
  agent_id, model, status, skills_json, plugins_json, duration_ms, prompt_tokens_estimate,
  completion_tokens_estimate, context_budget, prompt_size_estimate, context_budget_exceeded,
  policy_decision, policy_reason, bundle_id, bundle_version, error, validation_error
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(run_id) DO UPDATE SET
  timestamp=excluded.timestamp,
  status=excluded.status,
  error=excluded.error,
  validation_error=excluded.validation_error
`, event.RunID, event.Timestamp.Format(time.RFC3339Nano), event.GraphID, event.GraphNodeID, event.EventKind,
		event.ActorID, event.WorkspaceID, event.Source, event.CorrelationID,
		event.AgentID, event.Model, event.Status, string(skills), string(plugins),
		event.DurationMS, event.PromptTokensEstimate, event.CompletionTokensEstimate,
		event.ContextBudget, event.PromptSizeEstimate, boolInt(event.ContextBudgetExceeded),
		event.PolicyDecision, event.PolicyReason, event.BundleID, event.BundleVersion,
		event.Error, event.ValidationError)
	return err
}

type ListOptions struct {
	Limit  int
	Status string
	Agent  string
	Source string
}

func (s *Store) List(ctx context.Context, opts ListOptions) ([]observe.RunEvent, error) {
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	query := `SELECT run_id, timestamp, graph_id, graph_node_id, event_kind, actor_id, workspace_id, source, correlation_id,
agent_id, model, status, skills_json, plugins_json, duration_ms, prompt_tokens_estimate,
completion_tokens_estimate, context_budget, prompt_size_estimate, context_budget_exceeded,
policy_decision, policy_reason, bundle_id, bundle_version, error, validation_error FROM run_events`
	var where []string
	var args []any
	if opts.Status != "" {
		where = append(where, "status = ?")
		args = append(args, opts.Status)
	}
	if opts.Agent != "" {
		where = append(where, "agent_id = ?")
		args = append(args, opts.Agent)
	}
	if opts.Source != "" {
		where = append(where, "source = ?")
		args = append(args, opts.Source)
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY timestamp DESC LIMIT ?"
	args = append(args, opts.Limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []observe.RunEvent
	for rows.Next() {
		var event observe.RunEvent
		var ts, skillsJSON, pluginsJSON string
		var exceeded int
		if err := rows.Scan(&event.RunID, &ts, &event.GraphID, &event.GraphNodeID, &event.EventKind,
			&event.ActorID, &event.WorkspaceID, &event.Source, &event.CorrelationID,
			&event.AgentID, &event.Model, &event.Status, &skillsJSON, &pluginsJSON,
			&event.DurationMS, &event.PromptTokensEstimate, &event.CompletionTokensEstimate,
			&event.ContextBudget, &event.PromptSizeEstimate, &exceeded,
			&event.PolicyDecision, &event.PolicyReason, &event.BundleID, &event.BundleVersion,
			&event.Error, &event.ValidationError); err != nil {
			return nil, err
		}
		event.Timestamp, _ = time.Parse(time.RFC3339Nano, ts)
		_ = json.Unmarshal([]byte(skillsJSON), &event.Skills)
		_ = json.Unmarshal([]byte(pluginsJSON), &event.Plugins)
		event.ContextBudgetExceeded = exceeded != 0
		out = append(out, event)
	}
	return out, rows.Err()
}

type Summary struct {
	Total                    int            `json:"total"`
	StatusCounts             map[string]int `json:"status_counts"`
	TopAgents                map[string]int `json:"top_agents"`
	TopSkills                map[string]int `json:"top_skills"`
	TopPlugins               map[string]int `json:"top_plugins"`
	BundleVersions           map[string]int `json:"bundle_versions"`
	GraphExecutions          int            `json:"graph_executions"`
	ValidationFailures       int            `json:"validation_failures"`
	Timeouts                 int            `json:"timeouts"`
	ContextBudgetWarnings    int            `json:"context_budget_warnings"`
	PromptTokensEstimate     int            `json:"prompt_tokens_estimate"`
	CompletionTokensEstimate int            `json:"completion_tokens_estimate"`
	PolicyDenials            int            `json:"policy_denials"`
}

func (s *Store) Summary(ctx context.Context) (Summary, error) {
	events, err := s.List(ctx, ListOptions{Limit: 10000})
	if err != nil {
		return Summary{}, err
	}
	sum := Summary{
		StatusCounts:   make(map[string]int),
		TopAgents:      make(map[string]int),
		TopSkills:      make(map[string]int),
		TopPlugins:     make(map[string]int),
		BundleVersions: make(map[string]int),
	}
	for _, event := range events {
		sum.Total++
		sum.StatusCounts[event.Status]++
		if event.EventKind == "graph" {
			sum.GraphExecutions++
		}
		if event.Status == "validation_fail" {
			sum.ValidationFailures++
		}
		if event.Status == "timeout" {
			sum.Timeouts++
		}
		if event.AgentID != "" {
			sum.TopAgents[event.AgentID]++
		}
		for _, skill := range event.Skills {
			sum.TopSkills[skill]++
		}
		for _, plugin := range event.Plugins {
			sum.TopPlugins[plugin]++
		}
		if event.BundleID != "" || event.BundleVersion != "" {
			sum.BundleVersions[event.BundleID+"@"+event.BundleVersion]++
		}
		if event.ContextBudgetExceeded {
			sum.ContextBudgetWarnings++
		}
		sum.PromptTokensEstimate += event.PromptTokensEstimate
		sum.CompletionTokensEstimate += event.CompletionTokensEstimate
		if event.PolicyDecision == "deny" {
			sum.PolicyDenials++
		}
	}
	return sum, nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
