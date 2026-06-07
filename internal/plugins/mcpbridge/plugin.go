package mcpbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bryanbarton525/prism/internal/downstreammcp"
	"github.com/bryanbarton525/prism/internal/plugins"
	"github.com/bryanbarton525/prism/pkg/evidence"
)

const (
	ToolInventory = "mcp.inventory"
	outputLimit   = 12000
)

type Plugin struct {
	client Client
}

type Client interface {
	Servers() []downstreammcp.Server
	ListTools(context.Context, string, downstreammcp.ListToolsOptions) ([]downstreammcp.ToolSummary, error)
}

func New(client Client) *Plugin {
	return &Plugin{client: client}
}

func (p *Plugin) Name() string {
	return "mcp"
}

func (p *Plugin) Tools() []plugins.ToolSpec {
	return []plugins.ToolSpec{{
		Name:        ToolInventory,
		Description: "Collect compact downstream MCP server and tool inventory.",
		ReadOnly:    true,
		Mode:        "read_only",
		MaxBytes:    outputLimit,
	}}
}

func (p *Plugin) Call(ctx context.Context, call plugins.ToolCall) (plugins.ToolResult, error) {
	if call.Tool != ToolInventory {
		return plugins.ToolResult{}, fmt.Errorf("unsupported MCP bridge tool %q", call.Tool)
	}
	if p.client == nil {
		content := `{"configured":false,"servers":[]}`
		return plugins.ToolResult{Label: "runtime-plugin:mcp", Content: content, EvidencePack: evidencePack(content, 0)}, nil
	}
	servers := p.client.Servers()
	type serverInventory struct {
		Name        string                      `json:"name"`
		Transport   string                      `json:"transport"`
		URL         string                      `json:"url,omitempty"`
		Command     string                      `json:"command,omitempty"`
		Description string                      `json:"description,omitempty"`
		Tools       []downstreammcp.ToolSummary `json:"tools,omitempty"`
		Error       string                      `json:"error,omitempty"`
	}
	inventory := struct {
		Configured bool              `json:"configured"`
		Servers    []serverInventory `json:"servers"`
		Notes      []string          `json:"notes"`
	}{
		Configured: len(servers) > 0,
		Notes: []string{
			"Downstream MCP tools are available through Prism MCP bridge calls.",
			"Large downstream schemas stay out of the parent orchestrator unless explicitly requested.",
		},
	}
	for _, server := range servers {
		item := serverInventory{
			Name:        server.Name,
			Transport:   server.Transport,
			URL:         server.URL,
			Command:     server.Command,
			Description: server.Description,
		}
		tools, err := p.client.ListTools(ctx, server.Name, downstreammcp.ListToolsOptions{MaxTools: 20})
		if err != nil {
			item.Error = err.Error()
		} else {
			item.Tools = tools
		}
		inventory.Servers = append(inventory.Servers, item)
	}
	data, err := json.MarshalIndent(inventory, "", "  ")
	if err != nil {
		return plugins.ToolResult{}, err
	}
	content := trim(string(data), outputLimit)
	return plugins.ToolResult{Label: "runtime-plugin:mcp", Content: content, EvidencePack: evidencePack(content, len(servers))}, nil
}

func evidencePack(content string, servers int) *evidence.Pack {
	return &evidence.Pack{
		Kind:           "mcp.inventory",
		Plugin:         "mcp",
		CollectionTime: time.Now().UTC(),
		Limits:         evidence.Limits{MaxBytes: outputLimit, MaxArtifacts: 1},
		Summary:        map[string]any{"servers": servers, "bounded": true},
		Artifacts:      []evidence.Artifact{{Type: "mcp_inventory", Name: "downstream-mcp", Content: content}},
	}
}

func trim(s string, limit int) string {
	s = strings.TrimSpace(s)
	if limit <= 0 || len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
