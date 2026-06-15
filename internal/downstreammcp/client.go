package downstreammcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type Client struct {
	state State
}

type ToolSummary struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema,omitempty"`
}

type ListToolsOptions struct {
	IncludeSchema bool
	MaxTools      int
}

type CallResult struct {
	Server            string `json:"server"`
	Tool              string `json:"tool"`
	IsError           bool   `json:"is_error,omitempty"`
	Content           string `json:"content,omitempty"`
	StructuredContent any    `json:"structured_content,omitempty"`
	Truncated         bool   `json:"truncated,omitempty"`
}

func New(state State) *Client {
	return &Client{state: state}
}

func (c *Client) Servers() []Server {
	return c.state.PublicServers()
}

func (c *Client) ListTools(ctx context.Context, serverName string, opts ListToolsOptions) ([]ToolSummary, error) {
	server, ok := c.state.Get(serverName)
	if !ok {
		return nil, fmt.Errorf("downstream MCP server %q is not configured", serverName)
	}
	session, closeFn, err := c.connect(ctx, server)
	if err != nil {
		return nil, err
	}
	defer closeFn()
	res, err := session.ListTools(ctx, &mcpsdk.ListToolsParams{})
	if err != nil {
		return nil, fmt.Errorf("listing tools from %s: %w", serverName, err)
	}
	limit := opts.MaxTools
	if limit <= 0 || limit > len(res.Tools) {
		limit = len(res.Tools)
	}
	tools := make([]ToolSummary, 0, limit)
	for _, tool := range res.Tools[:limit] {
		summary := ToolSummary{
			Name:        tool.Name,
			Title:       tool.Title,
			Description: trim(tool.Description, 500),
		}
		if opts.IncludeSchema {
			summary.InputSchema = tool.InputSchema
		}
		tools = append(tools, summary)
	}
	return tools, nil
}

func (c *Client) CallTool(ctx context.Context, serverName, toolName string, args map[string]any) (CallResult, error) {
	server, ok := c.state.Get(serverName)
	if !ok {
		return CallResult{}, fmt.Errorf("downstream MCP server %q is not configured", serverName)
	}
	session, closeFn, err := c.connect(ctx, server)
	if err != nil {
		return CallResult{}, err
	}
	defer closeFn()
	res, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: toolName, Arguments: args})
	if err != nil {
		return CallResult{}, fmt.Errorf("calling %s.%s: %w", serverName, toolName, err)
	}
	content := contentText(res.Content)
	content, truncated := trimWithFlag(content, server.MaxBytes)
	return CallResult{
		Server:            serverName,
		Tool:              toolName,
		IsError:           res.IsError,
		Content:           content,
		StructuredContent: res.StructuredContent,
		Truncated:         truncated,
	}, nil
}

func (c *Client) connect(ctx context.Context, server Server) (*mcpsdk.ClientSession, func(), error) {
	server = server.withDefaults()
	if err := server.Validate(); err != nil {
		return nil, nil, err
	}
	timeout := time.Duration(server.TimeoutMS) * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, timeout)
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "prism-downstream-mcp", Version: "v0.1.0"}, nil)
	var transport mcpsdk.Transport
	switch server.Transport {
	case TransportCommand:
		transport = &mcpsdk.CommandTransport{Command: exec.CommandContext(ctx, server.Command, server.Args...)}
	case TransportSSE:
		transport = &mcpsdk.SSEClientTransport{Endpoint: server.URL, HTTPClient: &http.Client{Timeout: timeout}}
	}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("connecting to downstream MCP server %s: %w", server.Name, err)
	}
	closeFn := func() {
		_ = session.Close()
		cancel()
	}
	return session, closeFn, nil
}

func contentText(content []mcpsdk.Content) string {
	if len(content) == 0 {
		return ""
	}
	parts := make([]string, 0, len(content))
	for _, item := range content {
		if text, ok := item.(*mcpsdk.TextContent); ok {
			parts = append(parts, text.Text)
			continue
		}
		data, err := item.MarshalJSON()
		if err == nil {
			parts = append(parts, string(data))
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func trim(s string, limit int) string {
	out, _ := trimWithFlag(s, limit)
	return out
}

func trimWithFlag(s string, limit int) (string, bool) {
	if limit <= 0 || len(s) <= limit {
		return s, false
	}
	return s[:limit] + "...", true
}

func ParseArguments(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return nil, err
	}
	return args, nil
}
