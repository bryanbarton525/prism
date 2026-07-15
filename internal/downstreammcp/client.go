package downstreammcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/bryanbarton525/prism/internal/textutil"

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

// ListToolsResult reports the bounded tool inventory plus the true total so
// callers (and the models reading their output) can tell when the list was
// cut by MaxTools.
type ListToolsResult struct {
	Tools     []ToolSummary `json:"tools"`
	Total     int           `json:"total"`
	Truncated bool          `json:"truncated,omitempty"`
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

func (c *Client) ListTools(ctx context.Context, serverName string, opts ListToolsOptions) (ListToolsResult, error) {
	server, ok := c.state.Get(serverName)
	if !ok {
		return ListToolsResult{}, fmt.Errorf("downstream MCP server %q is not configured", serverName)
	}
	ctx, cancel := operationContext(ctx, server)
	defer cancel()
	session, closeFn, err := c.connect(ctx, server)
	if err != nil {
		return ListToolsResult{}, err
	}
	defer closeFn()
	res, err := session.ListTools(ctx, &mcpsdk.ListToolsParams{})
	if err != nil {
		return ListToolsResult{}, fmt.Errorf("listing tools from %s: %w", serverName, err)
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
	return ListToolsResult{
		Tools:     tools,
		Total:     len(res.Tools),
		Truncated: limit < len(res.Tools),
	}, nil
}

func (c *Client) CallTool(ctx context.Context, serverName, toolName string, args map[string]any) (CallResult, error) {
	server, ok := c.state.Get(serverName)
	if !ok {
		return CallResult{}, fmt.Errorf("downstream MCP server %q is not configured", serverName)
	}
	ctx, cancel := operationContext(ctx, server)
	defer cancel()
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

// operationContext applies the server's timeout_ms to one ListTools/CallTool
// operation (connect + request), so failures surface as a clean deadline on
// the call itself rather than the transport dying mid-request.
func operationContext(ctx context.Context, server Server) (context.Context, context.CancelFunc) {
	timeout := time.Duration(server.TimeoutMS) * time.Millisecond
	return context.WithTimeout(ctx, timeout)
}

// connect expects ctx to carry the operation deadline (see operationContext).
// The command-transport subprocess is bound to ctx, so it is reaped when the
// operation finishes or times out.
func (c *Client) connect(ctx context.Context, server Server) (*mcpsdk.ClientSession, func(), error) {
	server = server.withDefaults()
	if err := server.Validate(); err != nil {
		return nil, nil, err
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "prism-downstream-mcp", Version: "v0.1.0"}, nil)
	var transport mcpsdk.Transport
	switch server.Transport {
	case TransportCommand:
		transport = &mcpsdk.CommandTransport{Command: exec.CommandContext(ctx, server.Command, server.Args...)}
	case TransportSSE:
		// No http.Client Timeout here: a client-wide timeout would kill the
		// long-lived SSE stream. The operation context bounds the call.
		transport = &mcpsdk.SSEClientTransport{Endpoint: server.URL, HTTPClient: &http.Client{}}
	}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to downstream MCP server %s: %w", server.Name, err)
	}
	closeFn := func() {
		_ = session.Close()
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
		if err != nil {
			parts = append(parts, fmt.Sprintf("[unrenderable %T content: %v]", item, err))
			continue
		}
		parts = append(parts, string(data))
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func trim(s string, limit int) string {
	out, _ := trimWithFlag(s, limit)
	return out
}

func trimWithFlag(s string, limit int) (string, bool) {
	out, cut := textutil.CutBytes(s, limit)
	if !cut {
		return out, false
	}
	return out + "...", true
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
