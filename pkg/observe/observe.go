// Package observe defines stable run-event contracts for Prism integrations.
package observe

import (
	"context"
	"time"
)

// Metadata carries caller-supplied context for one Prism run.
type Metadata struct {
	ActorID       string `json:"actor_id,omitempty"`
	WorkspaceID   string `json:"workspace_id,omitempty"`
	Source        string `json:"source,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

// RunEvent is the stable event emitted after each Prism agent invocation.
type RunEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	RunID       string    `json:"run_id"`
	GraphID     string    `json:"graph_id,omitempty"`
	GraphNodeID string    `json:"graph_node_id,omitempty"`
	EventKind   string    `json:"event_kind,omitempty"`
	Metadata

	AgentID string   `json:"agent_id"`
	Model   string   `json:"model"`
	Status  string   `json:"status"`
	Skills  []string `json:"skills"`
	Plugins []string `json:"plugins,omitempty"`

	DurationMS               int64  `json:"duration_ms"`
	PromptTokensEstimate     int    `json:"prompt_tokens_estimate"`
	CompletionTokensEstimate int    `json:"completion_tokens_estimate"`
	ContextBudget            int    `json:"context_budget,omitempty"`
	PromptSizeEstimate       int    `json:"prompt_size_estimate,omitempty"`
	ContextBudgetExceeded    bool   `json:"context_budget_exceeded,omitempty"`
	PolicyDecision           string `json:"policy_decision,omitempty"`
	PolicyReason             string `json:"policy_reason,omitempty"`
	BundleID                 string `json:"bundle_id,omitempty"`
	BundleVersion            string `json:"bundle_version,omitempty"`

	Error           string `json:"error,omitempty"`
	ValidationError string `json:"validation_error,omitempty"`
}

// Sink receives run events. Implementations should be fast and non-blocking.
type Sink interface {
	ObserveRun(context.Context, RunEvent) error
}

// NoopSink drops all events.
type NoopSink struct{}

// ObserveRun implements Sink.
func (NoopSink) ObserveRun(context.Context, RunEvent) error {
	return nil
}
