package downstreammcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
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
	// FullContent is available only to the in-process run evidence store.
	FullContent string `json:"-"`
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
	var inventory []*mcpsdk.Tool
	cursor := ""
	seen := map[string]bool{}
	incomplete := false
	metadataBytes := 0
	for page := 0; page < 100; page++ {
		res, listErr := session.ListTools(ctx, &mcpsdk.ListToolsParams{Cursor: cursor})
		if listErr != nil {
			return ListToolsResult{}, fmt.Errorf("listing tools from %s: %w", serverName, listErr)
		}
		for _, tool := range res.Tools {
			if len(inventory) >= 2000 {
				incomplete = true
				break
			}
			encoded, _ := json.Marshal(tool)
			metadataBytes += len(encoded)
			if metadataBytes > 8<<20 {
				incomplete = true
				break
			}
			inventory = append(inventory, tool)
		}
		if incomplete || res.NextCursor == "" {
			break
		}
		if seen[res.NextCursor] {
			incomplete = true
			break
		}
		seen[res.NextCursor] = true
		cursor = res.NextCursor
		if page == 99 {
			incomplete = true
		}
	}
	limit := opts.MaxTools
	if limit <= 0 || limit > len(inventory) {
		limit = len(inventory)
	}
	tools := make([]ToolSummary, 0, limit)
	for _, tool := range inventory[:limit] {
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
		Total:     len(inventory),
		Truncated: limit < len(inventory) || incomplete,
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
	fullContent := contentText(res.Content)
	content, truncated := trimWithFlag(fullContent, server.MaxBytes)
	structured, structuredTruncated := boundedStructuredContent(res.StructuredContent, server.MaxBytes)
	if res.StructuredContent != nil {
		if raw, marshalErr := json.Marshal(res.StructuredContent); marshalErr == nil {
			fullContent += "\n" + string(raw)
		}
	}
	return CallResult{
		Server:            serverName,
		Tool:              toolName,
		IsError:           res.IsError,
		Content:           content,
		StructuredContent: structured,
		Truncated:         truncated || structuredTruncated,
		FullContent:       fullContent,
	}, nil
}

func boundedStructuredContent(value any, maxBytes int) (any, bool) {
	if value == nil {
		return nil, false
	}
	data, err := json.Marshal(value)
	if err != nil {
		return map[string]any{"truncated": true, "error": "structured content could not be serialized"}, true
	}
	if maxBytes <= 0 || len(data) <= maxBytes {
		return value, false
	}
	// Keep the value valid JSON while bounding the bytes that can reach the
	// bridge/model. Returning only the truncation marker avoids creating an
	// invalid partial JSON document.
	return map[string]any{"truncated": true}, true
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
		cmd := exec.CommandContext(ctx, server.Command, server.Args...)
		env, err := resolveReferencedValues(server.EnvRefs, "command env")
		if err != nil {
			return nil, nil, err
		}
		if len(env) > 0 {
			cmd.Env = append(os.Environ(), flattenEnvironment(env)...)
		}
		transport = &mcpsdk.CommandTransport{Command: cmd}
	case TransportSSE:
		// No http.Client Timeout here: a client-wide timeout would kill the
		// long-lived SSE stream. The operation context bounds the call.
		httpClient, err := downstreamHTTPClient(server.URL, server.HeaderRefs)
		if err != nil {
			return nil, nil, err
		}
		transport = &mcpsdk.SSEClientTransport{Endpoint: server.URL, HTTPClient: httpClient}
	case TransportStreamableHTTP:
		httpClient, err := downstreamHTTPClient(server.URL, server.HeaderRefs)
		if err != nil {
			return nil, nil, err
		}
		httpClient.Transport = limitedResponseTransport{base: httpClient.Transport, limit: 16 << 20}
		transport = &mcpsdk.StreamableClientTransport{Endpoint: server.URL, HTTPClient: httpClient}
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

type limitedResponseTransport struct {
	base  http.RoundTripper
	limit int64
}

func (t limitedResponseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp.ContentLength > t.limit {
		resp.Body.Close()
		return nil, fmt.Errorf("downstream MCP response exceeds %d bytes", t.limit)
	}
	resp.Body = &limitedReadCloser{ReadCloser: resp.Body, remaining: t.limit}
	return resp, nil
}

type limitedReadCloser struct {
	io.ReadCloser
	remaining int64
}

func (r *limitedReadCloser) Read(p []byte) (int, error) {
	if r.remaining < 0 {
		return 0, fmt.Errorf("downstream MCP response size limit exceeded")
	}
	if int64(len(p)) > r.remaining+1 {
		p = p[:r.remaining+1]
	}
	n, err := r.ReadCloser.Read(p)
	r.remaining -= int64(n)
	if r.remaining < 0 {
		return n, fmt.Errorf("downstream MCP response size limit exceeded")
	}
	return n, err
}

func resolveReferencedValues(refs map[string]string, label string) (map[string]string, error) {
	values := map[string]string{}
	for key, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return nil, fmt.Errorf("%s reference for %q is empty", label, key)
		}
		value, ok := os.LookupEnv(ref)
		if !ok {
			return nil, fmt.Errorf("%s reference %q for %q is not set", label, ref, key)
		}
		values[key] = value
	}
	return values, nil
}

func downstreamHTTPClient(endpoint string, headerRefs map[string]string) (*http.Client, error) {
	origin, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid downstream endpoint %q: %w", endpoint, err)
	}
	headers, err := resolveReferencedValues(headerRefs, "header")
	if err != nil {
		return nil, err
	}
	base := http.DefaultTransport
	if len(headers) == 0 {
		return &http.Client{
			Transport: base,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) == 0 {
					return nil
				}
				if !sameOrigin(origin, req.URL) {
					return fmt.Errorf("cross-origin redirect blocked from %s to %s", origin.Host, req.URL.Host)
				}
				return nil
			},
		}, nil
	}
	return &http.Client{
		Transport: headerTransport{base: base, headers: headers, origin: origin},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) == 0 {
				return nil
			}
			if !sameOrigin(origin, req.URL) {
				return fmt.Errorf("cross-origin redirect blocked from %s to %s", origin.Host, req.URL.Host)
			}
			return nil
		},
	}, nil
}

type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
	origin  *url.URL
}

func (h headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.Header = req.Header.Clone()
	if sameOrigin(h.origin, req.URL) {
		for key, value := range h.headers {
			cloned.Header.Set(key, value)
		}
	}
	return h.base.RoundTrip(cloned)
}

func sameOrigin(expected, got *url.URL) bool {
	if expected == nil || got == nil {
		return false
	}
	return strings.EqualFold(expected.Scheme, got.Scheme) && strings.EqualFold(expected.Host, got.Host)
}

func flattenEnvironment(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for key, value := range values {
		out = append(out, key+"="+value)
	}
	return out
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
