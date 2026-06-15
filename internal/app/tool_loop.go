package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/downstreammcp"
	llmruntime "github.com/bryanbarton525/prism/internal/llm/runtime"
	"github.com/bryanbarton525/prism/internal/ollama"
	"github.com/bryanbarton525/prism/internal/result"
)

const maxMCPToolRounds = 4

type chatToolResult struct {
	response         *ollama.ChatResponse
	artifacts        []result.Artifact
	promptTokens     int
	completionTokens int
}

func (r *Runner) chatWithTools(ctx context.Context, req ollama.ChatRequest, spec *agent.Spec) (*chatToolResult, error) {
	if !agentUsesMCP(spec) || r.downmcp == nil {
		if r.llm != nil {
			resp, err := r.chatWithModelRuntime(ctx, req)
			if err != nil {
				return nil, err
			}
			return resp, nil
		}
		resp, err := r.ollama.Chat(ctx, req)
		if err != nil {
			return nil, err
		}
		return &chatToolResult{response: resp, promptTokens: resp.PromptEvalCount, completionTokens: resp.EvalCount}, nil
	}
	if r.llm != nil {
		return r.chatWithModelRuntimeTools(ctx, req)
	}

	req.Tools = prismMCPTools()
	var artifacts []result.Artifact
	var last *ollama.ChatResponse
	var promptTokens int
	var completionTokens int
	for round := 0; round <= maxMCPToolRounds; round++ {
		resp, err := r.ollama.Chat(ctx, req)
		if err != nil {
			return nil, err
		}
		last = resp
		promptTokens += resp.PromptEvalCount
		completionTokens += resp.EvalCount
		if len(resp.Message.ToolCalls) == 0 {
			return &chatToolResult{response: resp, artifacts: artifacts, promptTokens: promptTokens, completionTokens: completionTokens}, nil
		}
		req.Messages = append(req.Messages, resp.Message)
		for _, call := range resp.Message.ToolCalls {
			content, artifact := r.executeMCPToolCall(ctx, call.Function.Name, call.Function.Arguments)
			artifacts = append(artifacts, artifact)
			req.Messages = append(req.Messages, ollama.Message{Role: "tool", Content: content, ToolName: call.Function.Name})
		}
	}
	artifacts = append(artifacts, result.Artifact{
		Type:    "mcp_tool_loop",
		Label:   "mcp-tool:max-rounds",
		Content: fmt.Sprintf("stopped after %d downstream MCP tool round(s)", maxMCPToolRounds),
	})
	return &chatToolResult{response: last, artifacts: artifacts, promptTokens: promptTokens, completionTokens: completionTokens}, nil
}

func (r *Runner) chatWithModelRuntime(ctx context.Context, req ollama.ChatRequest) (*chatToolResult, error) {
	maxTokens := 0
	if req.Options != nil {
		maxTokens = req.Options.NumPredict
	}
	chatResp, err := r.llm.Chat(ctx, llmruntime.ChatRequest{
		Model:       req.Model,
		Messages:    ollamaMessagesToRuntime(req.Messages),
		Temperature: temperaturePtr(req.Options),
		MaxTokens:   maxTokens,
	})
	if err != nil {
		return nil, err
	}
	return &chatToolResult{
		response: &ollama.ChatResponse{
			Model: chatResp.Model,
			Message: ollama.Message{
				Role:    chatResp.Message.Role,
				Content: chatResp.Message.Content,
			},
			PromptEvalCount: chatResp.Usage.PromptTokens,
			EvalCount:       chatResp.Usage.CompletionTokens,
		},
		promptTokens:     chatResp.Usage.PromptTokens,
		completionTokens: chatResp.Usage.CompletionTokens,
	}, nil
}

func (r *Runner) chatWithModelRuntimeTools(ctx context.Context, req ollama.ChatRequest) (*chatToolResult, error) {
	messages := ollamaMessagesToRuntime(req.Messages)
	tools := ollamaToolsToRuntime(prismMCPTools())
	maxTokens := 0
	if req.Options != nil {
		maxTokens = req.Options.NumPredict
	}
	var artifacts []result.Artifact
	var last *llmruntime.ChatResponse
	var promptTokens int
	var completionTokens int
	for round := 0; round <= maxMCPToolRounds; round++ {
		resp, err := r.llm.Chat(ctx, llmruntime.ChatRequest{
			Model:       req.Model,
			Messages:    messages,
			Tools:       tools,
			Temperature: temperaturePtr(req.Options),
			MaxTokens:   maxTokens,
		})
		if err != nil {
			return nil, err
		}
		last = resp
		promptTokens += resp.Usage.PromptTokens
		completionTokens += resp.Usage.CompletionTokens
		if len(resp.Message.ToolCalls) == 0 {
			return &chatToolResult{
				response:         runtimeResponseToOllama(resp),
				artifacts:        artifacts,
				promptTokens:     promptTokens,
				completionTokens: completionTokens,
			}, nil
		}
		messages = append(messages, resp.Message)
		for _, call := range resp.Message.ToolCalls {
			content, artifact := r.executeMCPToolCall(ctx, call.Function.Name, call.Function.Arguments)
			artifacts = append(artifacts, artifact)
			messages = append(messages, llmruntime.Message{Role: "tool", Content: content, ToolCallID: runtimeToolCallID(call)})
		}
	}
	artifacts = append(artifacts, result.Artifact{
		Type:    "mcp_tool_loop",
		Label:   "mcp-tool:max-rounds",
		Content: fmt.Sprintf("stopped after %d downstream MCP tool round(s)", maxMCPToolRounds),
	})
	return &chatToolResult{
		response:         runtimeResponseToOllama(last),
		artifacts:        artifacts,
		promptTokens:     promptTokens,
		completionTokens: completionTokens,
	}, nil
}

func temperaturePtr(opts *ollama.Options) *float64 {
	if opts == nil {
		return nil
	}
	return &opts.Temperature
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

func prismMCPTools() []ollama.Tool {
	return []ollama.Tool{
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

func ollamaMessagesToRuntime(messages []ollama.Message) []llmruntime.Message {
	out := make([]llmruntime.Message, 0, len(messages))
	for _, msg := range messages {
		out = append(out, llmruntime.Message{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolName,
			ToolCalls:  ollamaToolCallsToRuntime(msg.ToolCalls),
		})
	}
	return out
}

func ollamaToolsToRuntime(tools []ollama.Tool) []llmruntime.Tool {
	out := make([]llmruntime.Tool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, llmruntime.Tool{
			Type: tool.Type,
			Function: llmruntime.ToolFunction{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  tool.Function.Parameters,
			},
		})
	}
	return out
}

func ollamaToolCallsToRuntime(calls []ollama.ToolCall) []llmruntime.ToolCall {
	out := make([]llmruntime.ToolCall, 0, len(calls))
	for _, call := range calls {
		out = append(out, llmruntime.ToolCall{
			Type: "function",
			Function: llmruntime.ToolCallFunction{
				Name:      call.Function.Name,
				Arguments: call.Function.Arguments,
			},
		})
	}
	return out
}

func runtimeResponseToOllama(resp *llmruntime.ChatResponse) *ollama.ChatResponse {
	if resp == nil {
		return &ollama.ChatResponse{}
	}
	return &ollama.ChatResponse{
		Model: resp.Model,
		Message: ollama.Message{
			Role:    resp.Message.Role,
			Content: resp.Message.Content,
		},
		PromptEvalCount: resp.Usage.PromptTokens,
		EvalCount:       resp.Usage.CompletionTokens,
	}
}

func runtimeToolCallID(call llmruntime.ToolCall) string {
	if call.ID != "" {
		return call.ID
	}
	return call.Function.Name
}

func functionTool(name, description string, parameters map[string]any) ollama.Tool {
	return ollama.Tool{
		Type: "function",
		Function: ollama.ToolFunction{
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
		tools, err := r.downmcp.ListTools(ctx, server, downstreammcp.ListToolsOptions{IncludeSchema: includeSchema, MaxTools: maxTools})
		if err != nil {
			return "", "", err
		}
		return marshalToolResult(map[string]any{"server": server, "tools": tools}), "mcp-tool:" + server + ".tools", nil
	case "call_mcp_tool":
		server, err := stringArg(args, "server")
		if err != nil {
			return "", "", err
		}
		tool, err := stringArg(args, "tool")
		if err != nil {
			return "", "", err
		}
		toolArgs, _ := mapArg(args, "arguments")
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

func mapArg(args map[string]any, key string) (map[string]any, bool) {
	v, ok := args[key]
	if !ok || v == nil {
		return map[string]any{}, false
	}
	m, ok := v.(map[string]any)
	if !ok {
		return map[string]any{}, false
	}
	return m, true
}
