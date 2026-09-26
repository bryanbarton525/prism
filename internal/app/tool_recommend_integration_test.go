package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/bryanbarton525/prism/internal/downstreammcp"
	llmruntime "github.com/bryanbarton525/prism/internal/llm/runtime"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type evidenceReadingRuntime struct {
	fakeModelRuntime
	step    int
	sawTail bool
}

func TestOptedInAgentReceivesRealPotionShortlist(t *testing.T) {
	state := os.Getenv("PRISM_TEST_MODEL_STATE")
	if state == "" {
		t.Skip("set PRISM_TEST_MODEL_STATE after models setup potion")
	}
	root := makeTestRoot(t, map[string]string{"linear.md": linearSpec()}, map[string]string{"linear-issue-management": linearSkill()})
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "issue-fixture", Version: "1"}, nil)
	mcpsdk.AddTool(server, &mcpsdk.Tool{Name: "find_issue", Description: "Search open issues by keyword"}, func(_ context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, map[string]string, error) {
		return nil, map[string]string{"result": "found"}, nil
	})
	httpServer := httptest.NewServer(mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, nil))
	defer httpServer.Close()
	model := &fakeModelRuntime{}
	runner, err := New(Config{RootDir: root, ModelRuntime: model, DownstreamMCP: downstreammcp.New(downstreammcp.State{Servers: []downstreammcp.Server{{Name: "issues", Transport: downstreammcp.TransportStreamableHTTP, URL: httpServer.URL}}}), ToolModelStateDir: state, ToolRecommendationAgents: []string{"linear"}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.Run(context.Background(), RunRequest{AgentID: "linear", Task: "Find an open issue", SkillNames: []string{"linear-issue-management"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" || len(model.requests) != 1 || !strings.Contains(model.requests[0].Messages[1].Content, "find_issue") {
		t.Fatalf("status=%s model requests=%+v", result.Status, model.requests)
	}
}

func (m *evidenceReadingRuntime) Chat(_ context.Context, req llmruntime.ChatRequest) (*llmruntime.ChatResponse, error) {
	m.step++
	switch m.step {
	case 1:
		return &llmruntime.ChatResponse{Model: req.Model, Message: llmruntime.Message{Role: "assistant", ToolCalls: []llmruntime.ToolCall{{Function: llmruntime.ToolCallFunction{Name: "call_mcp_tool", Arguments: map[string]any{"server": "issues", "tool": "find_issue", "arguments": map[string]any{}}}}}}}, nil
	case 2:
		for _, message := range req.Messages {
			if message.Role != "tool" || message.ToolName != "call_mcp_tool" {
				continue
			}
			var preview struct {
				ResultID string `json:"result_id"`
				Preview  string `json:"preview"`
			}
			if err := json.Unmarshal([]byte(message.Content), &preview); err != nil {
				return nil, err
			}
			if preview.ResultID == "" || strings.Contains(preview.Preview, "TAIL_EVIDENCE") {
				return nil, context.Canceled
			}
			return &llmruntime.ChatResponse{Model: req.Model, Message: llmruntime.Message{Role: "assistant", ToolCalls: []llmruntime.ToolCall{{Function: llmruntime.ToolCallFunction{Name: "read_tool_result", Arguments: map[string]any{"result_id": preview.ResultID, "offset": 7900, "limit": 300}}}}}}, nil
		}
		return nil, context.Canceled
	default:
		for _, message := range req.Messages {
			if message.Role == "tool" && message.ToolName == "read_tool_result" && strings.Contains(message.Content, "TAIL_EVIDENCE") {
				m.sawTail = true
			}
		}
		return &llmruntime.ChatResponse{Model: req.Model, Message: llmruntime.Message{Role: "assistant", Content: `{"summary":"found evidence","findings":["TAIL_EVIDENCE"],"confidence":"high"}`}}, nil
	}
}

func TestAgentReadsLargeResultFromRealLocalMCPServer(t *testing.T) {
	root := makeTestRoot(t, map[string]string{"linear.md": linearSpec()}, map[string]string{"linear-issue-management": linearSkill()})
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "issue-fixture", Version: "1"}, nil)
	mcpsdk.AddTool(server, &mcpsdk.Tool{Name: "find_issue", Description: "Find an issue"}, func(_ context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, map[string]string, error) {
		return nil, map[string]string{"evidence": strings.Repeat("x", 8000) + "TAIL_EVIDENCE"}, nil
	})
	httpServer := httptest.NewServer(mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, nil))
	defer httpServer.Close()
	downstream := downstreammcp.New(downstreammcp.State{Servers: []downstreammcp.Server{{Name: "issues", Transport: downstreammcp.TransportStreamableHTTP, URL: httpServer.URL, MaxBytes: 500}}})
	model := &evidenceReadingRuntime{}
	runner, err := New(Config{RootDir: root, ModelRuntime: model, DownstreamMCP: downstream})
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.Run(context.Background(), RunRequest{AgentID: "linear", Task: "Find an issue with tail evidence", SkillNames: []string{"linear-issue-management"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" || !model.sawTail || model.step != 3 {
		t.Fatalf("status=%s read=%v steps=%d summary=%q", result.Status, model.sawTail, model.step, result.Summary)
	}
}
