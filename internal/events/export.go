package events

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/bryanbarton525/prism/pkg/observe"
)

func WriteJSON(w io.Writer, events []observe.RunEvent) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(events)
}

func WriteCSV(w io.Writer, events []observe.RunEvent) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"timestamp", "run_id", "event_kind", "graph_id", "graph_node_id", "source", "agent_id", "model", "status", "skills", "plugins", "duration_ms", "prompt_tokens_estimate", "completion_tokens_estimate", "context_budget_exceeded", "policy_decision", "policy_reason", "bundle_id", "bundle_version", "error", "validation_error"}); err != nil {
		return err
	}
	for _, event := range events {
		if err := cw.Write([]string{
			event.Timestamp.Format("2006-01-02T15:04:05.999999999Z07:00"),
			event.RunID,
			event.EventKind,
			event.GraphID,
			event.GraphNodeID,
			event.Source,
			event.AgentID,
			event.Model,
			event.Status,
			strings.Join(event.Skills, "|"),
			strings.Join(event.Plugins, "|"),
			strconv.FormatInt(event.DurationMS, 10),
			strconv.Itoa(event.PromptTokensEstimate),
			strconv.Itoa(event.CompletionTokensEstimate),
			fmt.Sprint(event.ContextBudgetExceeded),
			event.PolicyDecision,
			event.PolicyReason,
			event.BundleID,
			event.BundleVersion,
			event.Error,
			event.ValidationError,
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
