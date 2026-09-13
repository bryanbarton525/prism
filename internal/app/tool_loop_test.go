package app

import (
	"strings"
	"testing"

	llmruntime "github.com/bryanbarton525/prism/internal/llm/runtime"
)

func TestMapArg(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		want    map[string]any
		wantErr bool
	}{
		{name: "missing key", args: map[string]any{}, want: map[string]any{}},
		{name: "nil value", args: map[string]any{"arguments": nil}, want: map[string]any{}},
		{name: "object", args: map[string]any{"arguments": map[string]any{"a": "b"}}, want: map[string]any{"a": "b"}},
		{name: "stringified object", args: map[string]any{"arguments": `{"a":"b"}`}, want: map[string]any{"a": "b"}},
		{name: "empty string", args: map[string]any{"arguments": "  "}, want: map[string]any{}},
		{name: "unparseable string", args: map[string]any{"arguments": "not json"}, wantErr: true},
		{name: "wrong type", args: map[string]any{"arguments": 42}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapArg(tt.args, "arguments")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestParseTextToolCall(t *testing.T) {
	offered := map[string]bool{"call_mcp_tool": true, "list_mcp_servers": true}
	tests := []struct {
		name     string
		content  string
		wantTool string
		wantOK   bool
	}{
		{
			// Exact shape observed from SGLang without a tool-call parser.
			name:     "fenced json tool call",
			content:  "```json\n{\n  \"name\": \"call_mcp_tool\",\n  \"arguments\": {\n    \"server\": \"prism-self\",\n    \"tool\": \"list_agents\",\n    \"arguments\": {}\n  }\n}\n```",
			wantTool: "call_mcp_tool",
			wantOK:   true,
		},
		{
			name:     "plain json tool call",
			content:  `{"name":"list_mcp_servers","arguments":{}}`,
			wantTool: "list_mcp_servers",
			wantOK:   true,
		},
		{
			name:     "parameters instead of arguments",
			content:  `{"name":"call_mcp_tool","parameters":{"server":"s","tool":"t"}}`,
			wantTool: "call_mcp_tool",
			wantOK:   true,
		},
		{
			name:    "result envelope is not a tool call",
			content: `{"summary":"done","findings":[],"confidence":"high"}`,
			wantOK:  false,
		},
		{
			name:    "unknown tool name",
			content: `{"name":"rm_rf_everything","arguments":{}}`,
			wantOK:  false,
		},
		{
			name:    "prose",
			content: "The servers are prism-self and hang-server.",
			wantOK:  false,
		},
		{
			name:    "json with name field but non-object arguments",
			content: `{"name":"call_mcp_tool","arguments":"broken"}`,
			wantOK:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			call, ok := parseTextToolCall(tt.content, offered)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (call=%#v)", ok, tt.wantOK, call)
			}
			if ok && call.Function.Name != tt.wantTool {
				t.Fatalf("tool = %q, want %q", call.Function.Name, tt.wantTool)
			}
		})
	}
}

// TestRunner_Run_RecoversTextFormToolCall verifies the loop executes a tool
// call the runtime returned as fenced text instead of structured tool_calls.
func TestRunner_Run_RecoversTextFormToolCall(t *testing.T) {
	root := makeTestRoot(t,
		map[string]string{"linear.md": linearSpec()},
		map[string]string{"linear-issue-management": linearSkill()},
	)
	downstream := &fakeDownstreamMCP{}
	modelRuntime := &fakeModelRuntime{responses: []llmruntime.ChatResponse{
		{
			Model: "Qwen/Qwen2.5-Coder-14B-Instruct-AWQ",
			Message: llmruntime.Message{
				Role:    "assistant",
				Content: "```json\n{\"name\": \"call_mcp_tool\", \"arguments\": {\"server\": \"linear\", \"tool\": \"create_issue\", \"arguments\": {\"title\": \"x\"}}}\n```",
			},
			Usage: llmruntime.Usage{PromptTokens: 10, CompletionTokens: 4},
		},
		{
			Model: "Qwen/Qwen2.5-Coder-14B-Instruct-AWQ",
			Message: llmruntime.Message{
				Role:    "assistant",
				Content: `{"summary":"issue created via recovered tool call","confidence":"high"}`,
			},
			Usage: llmruntime.Usage{PromptTokens: 12, CompletionTokens: 3},
		},
	}}
	runner, err := New(Config{RootDir: root, ModelRuntime: modelRuntime, DownstreamMCP: downstream})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	res, err := runner.Run(t.Context(), RunRequest{
		AgentID:    "linear",
		Task:       "Create an issue.",
		SkillNames: []string{"linear-issue-management"},
	})
	if err != nil {
		t.Fatalf("Run(): %v", err)
	}
	if len(downstream.calls) != 1 || downstream.calls[0] != "linear.create_issue" {
		t.Fatalf("downstream calls = %#v", downstream.calls)
	}
	if !strings.Contains(res.Summary, "issue created via recovered tool call") {
		t.Fatalf("summary = %q", res.Summary)
	}
	foundRecovery := false
	for _, a := range res.Artifacts {
		if a.Label == "mcp-tool:text-form-recovered" {
			foundRecovery = true
		}
	}
	if !foundRecovery {
		t.Fatal("missing text-form-recovered artifact")
	}
}

func TestNormalizeToolCallIDs(t *testing.T) {
	calls := []llmruntime.ToolCall{
		{Function: llmruntime.ToolCallFunction{Name: "call_mcp_tool"}},
		{Function: llmruntime.ToolCallFunction{Name: "call_mcp_tool"}},
		{ID: "call_9", Function: llmruntime.ToolCallFunction{Name: "list_mcp_servers"}},
	}
	normalizeToolCallIDs(calls, 2)
	if calls[0].ID == calls[1].ID {
		t.Fatalf("duplicate synthesized IDs: %q", calls[0].ID)
	}
	if calls[2].ID != "call_9" {
		t.Fatalf("existing ID overwritten: %q", calls[2].ID)
	}
}

// TestRunner_Run_MCPToolLoopExhaustionSynthesizes verifies that when the model
// keeps requesting tools past the round budget, the runner forces a final
// synthesis call without tools instead of returning an empty tool-call
// message as a successful result.
func TestRunner_Run_MCPToolLoopExhaustionSynthesizes(t *testing.T) {
	root := makeTestRoot(t,
		map[string]string{"linear.md": linearSpec()},
		map[string]string{"linear-issue-management": linearSkill()},
	)
	downstream := &fakeDownstreamMCP{}
	toolCallResp := llmruntime.ChatResponse{
		Model: "openai/gpt-oss-20b",
		Message: llmruntime.Message{
			Role: "assistant",
			ToolCalls: []llmruntime.ToolCall{{
				Type: "function",
				Function: llmruntime.ToolCallFunction{
					Name:      "list_mcp_servers",
					Arguments: map[string]any{},
				},
			}},
		},
		Usage: llmruntime.Usage{PromptTokens: 5, CompletionTokens: 1},
	}
	finalResp := llmruntime.ChatResponse{
		Model: "openai/gpt-oss-20b",
		Message: llmruntime.Message{
			Role:    "assistant",
			Content: `{"summary":"synthesized after exhausting tool rounds","confidence":"medium"}`,
		},
		Usage: llmruntime.Usage{PromptTokens: 6, CompletionTokens: 2},
	}
	// maxMCPToolRounds tool-call responses, then the forced synthesis answer.
	responses := make([]llmruntime.ChatResponse, 0, maxMCPToolRounds+1)
	for i := 0; i < maxMCPToolRounds; i++ {
		responses = append(responses, toolCallResp)
	}
	responses = append(responses, finalResp)
	modelRuntime := &fakeModelRuntime{responses: responses}

	runner, err := New(Config{RootDir: root, ModelRuntime: modelRuntime, DownstreamMCP: downstream})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	res, err := runner.Run(t.Context(), RunRequest{
		AgentID:    "linear",
		Task:       "Loop forever.",
		SkillNames: []string{"linear-issue-management"},
	})
	if err != nil {
		t.Fatalf("Run(): %v", err)
	}
	if modelRuntime.calls != maxMCPToolRounds+1 {
		t.Fatalf("model runtime calls = %d, want %d", modelRuntime.calls, maxMCPToolRounds+1)
	}
	// The synthesis request must not offer tools again.
	lastReq := modelRuntime.requests[len(modelRuntime.requests)-1]
	if len(lastReq.Tools) != 0 {
		t.Fatalf("final synthesis request still offers %d tools", len(lastReq.Tools))
	}
	if !strings.Contains(res.Summary, "synthesized after exhausting tool rounds") {
		t.Fatalf("summary = %q", res.Summary)
	}
	foundMarker := false
	for _, a := range res.Artifacts {
		if a.Label == "mcp-tool:max-rounds" {
			foundMarker = true
		}
	}
	if !foundMarker {
		t.Fatal("missing max-rounds artifact")
	}
}
