package linear

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bryanbarton525/prism/internal/plugins"
)

func TestMCPContextExtractsIssueKeysAndAction(t *testing.T) {
	t.Setenv("PRISM_LINEAR_MCP_URL", "https://mcp.linear.app/mcp")
	plugin := New()
	res, err := plugin.Call(context.Background(), plugins.ToolCall{
		Tool: ToolMCPContext,
		Args: map[string]string{"task": "Update ENG-123 and ENG-123 priority, then comment on OPS-9"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Label != "runtime-plugin:linear" {
		t.Fatalf("label = %q", res.Label)
	}
	if res.EvidencePack == nil || res.EvidencePack.Kind != "linear.mcp_context" {
		t.Fatalf("evidence pack = %#v", res.EvidencePack)
	}

	var payload struct {
		OperationHint string   `json:"operation_hint"`
		IssueKeys     []string `json:"issue_keys"`
		WriteExec     bool     `json:"write_execution"`
	}
	if err := json.Unmarshal([]byte(res.Content), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.OperationHint != "update_issue" {
		t.Fatalf("operation_hint = %q", payload.OperationHint)
	}
	if payload.WriteExec {
		t.Fatal("linear context plugin must not execute writes")
	}
	if len(payload.IssueKeys) != 2 || payload.IssueKeys[0] != "ENG-123" || payload.IssueKeys[1] != "OPS-9" {
		t.Fatalf("issue_keys = %#v", payload.IssueKeys)
	}
}

func TestMCPContextFallsBackToDefaultURL(t *testing.T) {
	t.Setenv("PRISM_LINEAR_MCP_URL", "://bad")
	plugin := New()
	res, err := plugin.Call(context.Background(), plugins.ToolCall{
		Tool: ToolMCPContext,
		Args: map[string]string{"task": "Create a Linear issue for checkout follow-up"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(res.Content), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["mcp_server_url"] != defaultMCPURL {
		t.Fatalf("mcp_server_url = %#v", payload["mcp_server_url"])
	}
	if payload["approval_required"] != true {
		t.Fatalf("approval_required = %#v", payload["approval_required"])
	}
}

func TestMCPContextInfersDraftIssueAsCreate(t *testing.T) {
	plugin := New()
	res, err := plugin.Call(context.Background(), plugins.ToolCall{
		Tool: ToolMCPContext,
		Args: map[string]string{"task": "Draft a proposed Linear issue for checkout-api rollout follow-up"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(res.Content), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["operation_hint"] != "create_issue" {
		t.Fatalf("operation_hint = %#v", payload["operation_hint"])
	}
}
