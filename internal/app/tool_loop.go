package app

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/downstreammcp"
	llmruntime "github.com/bryanbarton525/prism/internal/llm/runtime"
	"github.com/bryanbarton525/prism/internal/result"
)

const maxMCPToolRounds = 4

type chatToolResult struct {
	response         *llmruntime.ChatResponse
	artifacts        []result.Artifact
	promptTokens     int
	completionTokens int
}

func (r *Runner) chatWithTools(ctx context.Context, req llmruntime.ChatRequest, spec *agent.Spec) (*chatToolResult, error) {
	if !agentUsesMCP(spec) || r.downmcp == nil {
		resp, err := r.llm.Chat(ctx, req)
		if err != nil {
			return nil, err
		}
		return &chatToolResult{response: resp, promptTokens: resp.Usage.PromptTokens, completionTokens: resp.Usage.CompletionTokens}, nil
	}
	return r.chatWithMCPToolLoop(ctx, req)
}

func (r *Runner) chatWithMCPToolLoop(ctx context.Context, req llmruntime.ChatRequest) (*chatToolResult, error) {
	req.Tools = prismMCPTools()
	offered := make(map[string]bool, len(req.Tools))
	for _, tool := range req.Tools {
		offered[tool.Function.Name] = true
	}
	var artifacts []result.Artifact
	var promptTokens int
	var completionTokens int
	for round := 0; round < maxMCPToolRounds; round++ {
		resp, err := r.llm.Chat(ctx, req)
		if err != nil {
			return nil, err
		}
		promptTokens += resp.Usage.PromptTokens
		completionTokens += resp.Usage.CompletionTokens
		if len(resp.Message.ToolCalls) == 0 {
			// Some OpenAI-compatible servers (e.g. SGLang without a
			// tool-call parser for the model) return the model's tool call as
			// plain or fenced JSON text. Recover it so the call executes
			// instead of the raw JSON being returned as the final answer.
			call, ok := parseTextToolCall(resp.Message.Content, offered)
			if !ok {
				return &chatToolResult{response: resp, artifacts: artifacts, promptTokens: promptTokens, completionTokens: completionTokens}, nil
			}
			resp.Message.ToolCalls = []llmruntime.ToolCall{call}
			artifacts = append(artifacts, result.Artifact{
				Type:    "mcp_tool_loop",
				Label:   "mcp-tool:text-form-recovered",
				Content: fmt.Sprintf("recovered text-form tool call to %s from assistant content (runtime returned no structured tool_calls)", call.Function.Name),
			})
		}
		// IDs must be assigned before the assistant message is appended so the
		// tool results below correlate with the calls the model sees in history.
		normalizeToolCallIDs(resp.Message.ToolCalls, round)
		req.Messages = append(req.Messages, resp.Message)
		for _, call := range resp.Message.ToolCalls {
			content, artifact := r.executeMCPToolCall(ctx, call.Function.Name, call.Function.Arguments)
			artifacts = append(artifacts, artifact)
			req.Messages = append(req.Messages, llmruntime.Message{
				Role:       "tool",
				Content:    content,
				ToolCallID: call.ID,
				ToolName:   call.Function.Name,
			})
		}
	}
	// Round budget exhausted while the model still wants tools. Withdraw the
	// tools and force one final synthesis pass so the model answers from the
	// tool results already in the conversation; returning the tool-call
	// message as the final response would hand the orchestrator an empty
	// answer marked ok.
	req.Tools = nil
	resp, err := r.llm.Chat(ctx, req)
	if err != nil {
		return nil, err
	}
	promptTokens += resp.Usage.PromptTokens
	completionTokens += resp.Usage.CompletionTokens
	artifacts = append(artifacts, result.Artifact{
		Type:    "mcp_tool_loop",
		Label:   "mcp-tool:max-rounds",
		Content: fmt.Sprintf("stopped after %d downstream MCP tool round(s); final answer synthesized without tools", maxMCPToolRounds),
	})
	return &chatToolResult{response: resp, artifacts: artifacts, promptTokens: promptTokens, completionTokens: completionTokens}, nil
}

func agentUsesMCP(spec *agent.Spec) bool {
	for _, tool := range spec.Tools {
		if tool == "mcp" {
			return true
		}
	}
	return false
}

func mcpToolLoopInstructions() string {
	return `

# Prism MCP Bridge Tools

You may call Prism bridge tools during this run:

- list_mcp_servers: discover configured downstream MCP servers.
- list_mcp_server_tools: inspect compact downstream tool names and schemas.
- call_mcp_tool: execute one bounded downstream MCP tool call.

Use these tools when the task requires live downstream MCP evidence or action.
After tool results are returned, produce the final Prism result envelope for the parent.
Do not claim a downstream mutation succeeded unless a call_mcp_tool result proves it.
`
}

func prismMCPTools() []llmruntime.Tool {
	return []llmruntime.Tool{
		functionTool("list_mcp_servers", "List downstream MCP servers configured for Prism.", map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}),
		functionTool("list_mcp_server_tools", "List compact tool inventory for one downstream MCP server.", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"server":         map[string]any{"type": "string", "description": "Configured downstream MCP server name."},
				"include_schema": map[string]any{"type": "boolean", "description": "Whether to include input schemas."},
				"max_tools":      map[string]any{"type": "integer", "description": "Maximum tools to return."},
			},
			"required": []string{"server"},
		}),
		functionTool("call_mcp_tool", "Call one tool on a configured downstream MCP server and return a bounded result.", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"server":    map[string]any{"type": "string", "description": "Configured downstream MCP server name."},
				"tool":      map[string]any{"type": "string", "description": "Downstream MCP tool name."},
				"arguments": map[string]any{"type": "object", "description": "Arguments for the downstream MCP tool."},
			},
			"required": []string{"server", "tool"},
		}),
	}
}

var toolCallFenceRE = regexp.MustCompile("(?s)```(?:json)?\\s*([\\s\\S]*?)```")

// parseTextToolCall recognizes a tool call emitted as assistant text —
// `{"name": <tool>, "arguments": {...}}`, optionally inside a markdown fence —
// and returns it as a structured call when the named tool is one this loop
// offered. Anything else (result envelopes, prose, unknown tools) is not a
// tool call.
func parseTextToolCall(content string, offered map[string]bool) (llmruntime.ToolCall, bool) {
	content = strings.TrimSpace(content)
	if content == "" {
		return llmruntime.ToolCall{}, false
	}
	candidates := []string{content}
	if m := toolCallFenceRE.FindStringSubmatch(content); len(m) == 2 {
		candidates = append([]string{strings.TrimSpace(m[1])}, candidates...)
	}
	for _, c := range candidates {
		if !strings.HasPrefix(c, "{") {
			continue
		}
		var raw struct {
			Name       string          `json:"name"`
			Arguments  json.RawMessage `json:"arguments"`
			Parameters json.RawMessage `json:"parameters"`
		}
		if err := json.Unmarshal([]byte(c), &raw); err != nil {
			continue
		}
		if raw.Name == "" || !offered[raw.Name] {
			continue
		}
		argsRaw := raw.Arguments
		if len(argsRaw) == 0 {
			argsRaw = raw.Parameters
		}
		args := map[string]any{}
		if len(argsRaw) > 0 && string(argsRaw) != "null" {
			if err := json.Unmarshal(argsRaw, &args); err != nil {
				continue
			}
		}
		return llmruntime.ToolCall{
			Type:     "function",
			Function: llmruntime.ToolCallFunction{Name: raw.Name, Arguments: args},
		}, true
	}
	return llmruntime.ToolCall{}, false
}

// normalizeToolCallIDs fills in unique IDs for runtimes (e.g. Ollama) that do
// not assign them. Two calls to the same tool in one round must not share an
// ID or the model cannot correlate results with calls.
func normalizeToolCallIDs(calls []llmruntime.ToolCall, round int) {
	for i := range calls {
		if calls[i].ID == "" {
			calls[i].ID = fmt.Sprintf("%s-r%d-%d", calls[i].Function.Name, round, i)
		}
	}
}

func functionTool(name, description string, parameters map[string]any) llmruntime.Tool {
	return llmruntime.Tool{
		Type: "function",
		Function: llmruntime.ToolFunction{
			Name:        name,
			Description: description,
			Parameters:  parameters,
		},
	}
}

func (r *Runner) executeMCPToolCall(ctx context.Context, name string, args map[string]any) (string, result.Artifact) {
	if args == nil {
		args = map[string]any{}
	}
	content, label, err := r.dispatchMCPToolCall(ctx, name, args)
	if err != nil {
		content = marshalToolResult(map[string]any{"error": err.Error()})
		label = "mcp-tool:" + name
	}
	return content, result.Artifact{
		Type:    "mcp_tool_call",
		Label:   label,
		Content: content,
	}
}

func (r *Runner) dispatchMCPToolCall(ctx context.Context, name string, args map[string]any) (string, string, error) {
	switch name {
	case "list_mcp_servers":
		return marshalToolResult(map[string]any{"servers": r.downmcp.Servers()}), "mcp-tool:list_mcp_servers", nil
	case "list_mcp_server_tools":
		server, err := stringArg(args, "server")
		if err != nil {
			return "", "", err
		}
		includeSchema, _ := boolArg(args, "include_schema")
		maxTools, _ := intArg(args, "max_tools")
		res, err := r.downmcp.ListTools(ctx, server, downstreammcp.ListToolsOptions{IncludeSchema: includeSchema, MaxTools: maxTools})
		if err != nil {
			return "", "", err
		}
		return marshalToolResult(map[string]any{"server": server, "tools": res.Tools, "total": res.Total, "truncated": res.Truncated}), "mcp-tool:" + server + ".tools", nil
	case "call_mcp_tool":
		server, err := stringArg(args, "server")
		if err != nil {
			return "", "", err
		}
		tool, err := stringArg(args, "tool")
		if err != nil {
			return "", "", err
		}
		toolArgs, err := mapArg(args, "arguments")
		if err != nil {
			return "", "", err
		}
		res, err := r.downmcp.CallTool(ctx, server, tool, toolArgs)
		if err != nil {
			return "", "", err
		}
		return marshalToolResult(res), "mcp-tool:" + server + "." + tool, nil
	default:
		return "", "", fmt.Errorf("unsupported Prism MCP bridge tool %q", name)
	}
}

func marshalToolResult(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(data)
}

func stringArg(args map[string]any, key string) (string, error) {
	v, ok := args[key]
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("%s must be a non-empty string", key)
	}
	return s, nil
}

func boolArg(args map[string]any, key string) (bool, bool) {
	v, ok := args[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

func intArg(args map[string]any, key string) (int, bool) {
	v, ok := args[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

// mapArg extracts a JSON-object argument. Small local models often emit the
// object as a JSON-encoded string, so that form is decoded rather than
// rejected. Any other shape is an error — silently substituting an empty map
// would fire the downstream tool with the wrong arguments.
func mapArg(args map[string]any, key string) (map[string]any, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return map[string]any{}, nil
	}
	switch m := v.(type) {
	case map[string]any:
		return m, nil
	case string:
		if strings.TrimSpace(m) == "" {
			return map[string]any{}, nil
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(m), &out); err != nil {
			return nil, fmt.Errorf("%s must be a JSON object, got an unparseable string: %v", key, err)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%s must be a JSON object, got %T", key, v)
	}
}
