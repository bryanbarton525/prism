package mcpbridge

import (
	"context"
	"encoding/json"
	"fmt"
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
	ListTools(context.Context, string, downstreammcp.ListToolsOptions) (downstreammcp.ListToolsResult, error)
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
	inventory := inventoryDoc{
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
		res, err := p.client.ListTools(ctx, server.Name, downstreammcp.ListToolsOptions{MaxTools: 20})
		if err != nil {
			item.Error = err.Error()
		} else {
			item.Tools = res.Tools
			item.ToolsTotal = res.Total
			item.ToolsTruncated = res.Truncated
		}
		inventory.Servers = append(inventory.Servers, item)
	}
	content, err := marshalBounded(inventory, outputLimit)
	if err != nil {
		return plugins.ToolResult{}, err
	}
	return plugins.ToolResult{Label: "runtime-plugin:mcp", Content: content, EvidencePack: evidencePack(content, len(servers))}, nil
}

type serverInventory struct {
	Name           string                      `json:"name"`
	Transport      string                      `json:"transport"`
	URL            string                      `json:"url,omitempty"`
	Command        string                      `json:"command,omitempty"`
	Description    string                      `json:"description,omitempty"`
	Tools          []downstreammcp.ToolSummary `json:"tools,omitempty"`
	ToolsTotal     int                         `json:"tools_total,omitempty"`
	ToolsTruncated bool                        `json:"tools_truncated,omitempty"`
	Error          string                      `json:"error,omitempty"`
}

type inventoryDoc struct {
	Configured bool              `json:"configured"`
	Servers    []serverInventory `json:"servers"`
	Notes      []string          `json:"notes"`
}

// marshalBounded keeps the inventory under limit by shrinking it
// structurally — first dropping tool descriptions, then capping tools per
// server — and re-marshalling. Cutting marshalled JSON at a byte offset would
// hand the model invalid JSON presented as structured inventory.
func marshalBounded(inventory inventoryDoc, limit int) (string, error) {
	marshal := func() (string, error) {
		data, err := json.MarshalIndent(inventory, "", "  ")
		return string(data), err
	}
	content, err := marshal()
	if err != nil || len(content) <= limit {
		return content, err
	}

	for i := range inventory.Servers {
		for j := range inventory.Servers[i].Tools {
			inventory.Servers[i].Tools[j].Description = ""
		}
	}
	inventory.Notes = append(inventory.Notes, "Inventory reduced to fit output limit: tool descriptions omitted.")
	content, err = marshal()
	if err != nil || len(content) <= limit {
		return content, err
	}

	for _, keep := range []int{10, 5, 2, 0} {
		reduced := false
		for i := range inventory.Servers {
			s := &inventory.Servers[i]
			if len(s.Tools) > keep {
				s.Tools = s.Tools[:keep]
				s.ToolsTruncated = true
				reduced = true
			}
		}
		if reduced {
			inventory.Notes = append(inventory.Notes, fmt.Sprintf("Inventory reduced to fit output limit: at most %d tool(s) listed per server; use list_mcp_server_tools for the full set.", keep))
		}
		content, err = marshal()
		if err != nil || len(content) <= limit {
			return content, err
		}
	}

	// Even with zero tools listed, enough servers with long metadata can
	// exceed the limit. Fall back to a minimal valid document rather than
	// returning JSON that overflows the declared bound.
	minimal := inventoryDoc{
		Configured: inventory.Configured,
		Notes: []string{
			fmt.Sprintf("Inventory of %d server(s) exceeded the output limit; use list_mcp_servers and list_mcp_server_tools for details.", len(inventory.Servers)),
		},
	}
	data, err := json.MarshalIndent(minimal, "", "  ")
	return string(data), err
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
