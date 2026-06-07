package linear

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/bryanbarton525/prism/internal/plugins"
	"github.com/bryanbarton525/prism/pkg/evidence"
)

const (
	ToolMCPContext = "linear.mcp_context"
	defaultMCPURL  = "https://mcp.linear.app/mcp"
	outputLimit    = 8000
)

type Plugin struct{}

func New() *Plugin {
	return &Plugin{}
}

func (p *Plugin) Name() string {
	return "linear"
}

func (p *Plugin) Tools() []plugins.ToolSpec {
	return []plugins.ToolSpec{{
		Name:        ToolMCPContext,
		Description: "Collect bounded Linear MCP setup context and issue-operation hints.",
		ReadOnly:    true,
		Mode:        "read_only",
		MaxBytes:    outputLimit,
	}}
}

func (p *Plugin) Call(ctx context.Context, call plugins.ToolCall) (plugins.ToolResult, error) {
	if err := ctx.Err(); err != nil {
		return plugins.ToolResult{}, err
	}
	if call.Tool != ToolMCPContext {
		return plugins.ToolResult{}, fmt.Errorf("unsupported Linear tool %q", call.Tool)
	}

	task := call.Args["task"]
	mcpURL := configuredMCPURL()
	keys := extractIssueKeys(task)
	action := inferAction(task)
	authConfigured := os.Getenv("LINEAR_API_KEY") != "" || os.Getenv("PRISM_LINEAR_API_KEY") != ""

	payload := map[string]any{
		"mcp_server_url":      mcpURL,
		"authenticated_hint":  authConfigured,
		"write_execution":     false,
		"operation_hint":      action,
		"issue_keys":          keys,
		"approval_required":   action != "search",
		"recommended_channel": "authenticated Linear MCP server",
		"notes": []string{
			"Prism does not execute Linear writes from this plugin.",
			"Use the authenticated Linear MCP server to create, update, comment on, or archive issues.",
			"Do not claim a Linear mutation completed unless the Linear MCP tool response is attached as evidence.",
		},
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return plugins.ToolResult{}, err
	}
	content := trim(string(data), outputLimit)

	pack := evidence.Pack{
		Kind:           "linear.mcp_context",
		Plugin:         "linear",
		CollectionTime: time.Now().UTC(),
		Limits:         evidence.Limits{MaxBytes: outputLimit, MaxArtifacts: 1},
		Summary: map[string]any{
			"issue_keys":         keys,
			"operation_hint":     action,
			"write_execution":    false,
			"auth_hint_present":  authConfigured,
			"configured_mcp_url": mcpURL,
		},
		Artifacts: []evidence.Artifact{{
			Type:    "linear_mcp_context",
			Name:    "linear",
			Content: content,
		}},
	}

	return plugins.ToolResult{Label: "runtime-plugin:linear", Content: content, EvidencePack: &pack}, nil
}

func configuredMCPURL() string {
	raw := strings.TrimSpace(os.Getenv("PRISM_LINEAR_MCP_URL"))
	if raw == "" {
		return defaultMCPURL
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return defaultMCPURL
	}
	return raw
}

var issueKeyPattern = regexp.MustCompile(`\b[A-Z][A-Z0-9]+-\d+\b`)

func extractIssueKeys(task string) []string {
	matches := issueKeyPattern.FindAllString(task, -1)
	if len(matches) == 0 {
		return []string{}
	}
	seen := map[string]bool{}
	keys := make([]string, 0, len(matches))
	for _, match := range matches {
		if seen[match] {
			continue
		}
		seen[match] = true
		keys = append(keys, match)
	}
	return keys
}

func inferAction(task string) string {
	lower := strings.ToLower(task)
	switch {
	case containsAny(lower, "create", "draft", "propose", "proposed issue", "open a linear", "file a", "new issue", "new ticket"):
		return "create_issue"
	case containsAny(lower, "edit", "update", "move", "assign", "set status", "priority", "label"):
		return "update_issue"
	case containsAny(lower, "comment", "reply", "add note"):
		return "comment"
	case containsAny(lower, "archive", "close", "cancel"):
		return "archive_or_close"
	case containsAny(lower, "triage", "dedupe", "duplicate", "search", "find"):
		return "search"
	default:
		return "triage"
	}
}

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

func trim(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
