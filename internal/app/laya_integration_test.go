//go:build integration

package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bryanbarton525/prism/internal/downstreammcp"
	"github.com/bryanbarton525/prism/internal/extensions"
	llmruntime "github.com/bryanbarton525/prism/internal/llm/runtime"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRecommendToolsWithLiveLaya(t *testing.T) {
	endpoint := os.Getenv("PRISM_LIVE_LAYA_URL")
	state := os.Getenv("PRISM_TEST_MODEL_STATE")
	if endpoint == "" || state == "" {
		t.Skip("set PRISM_LIVE_LAYA_URL and PRISM_TEST_MODEL_STATE")
	}
	root := makeTestRoot(t, map[string]string{"linear.md": linearSpec()}, nil)
	for _, catalog := range []string{"two", "fifty"} {
		t.Run(catalog, func(t *testing.T) {
			cfg := Config{RootDir: root, ToolModelStateDir: state, ToolRecommendationModel: "onnx", DecisionURL: endpoint, DecisionModel: "laya-english"}
			if catalog == "two" {
				cfg.DownstreamMCP = &recommendationFixture{}
				cfg.MCPAccessConfigured = true
				cfg.MCPAccess = extensions.MCPAccessState{Agents: map[string]extensions.MCPAccessRule{"linear": {Mode: extensions.MCPAccessModeCustom, Servers: []string{"issues"}}}}
			} else {
				cfg.DownstreamMCP = loadRecommendationFixture{}
			}
			runner, err := New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			defer cancel()
			start := time.Now()
			got, err := runner.RecommendTools(ctx, "linear", "Find issue ENG-731 without modifying it", 5)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("catalog=%s elapsed=%s first=%+v", catalog, time.Since(start), got.Tools)
			if got.ScoreKind != "laya_choice" || got.ModelIdentity != "laya-english" || len(got.Warnings) != 0 || len(got.Tools) == 0 || got.Tools[0].Name != "find_issue" {
				t.Fatalf("live Laya recommendation: %+v", got)
			}
		})
	}
}

// This fixture permits real model tool calls without writing to external systems.
func TestLiveLayaSGLangIssueLookup(t *testing.T) {
	endpoint := os.Getenv("PRISM_LIVE_LAYA_URL")
	base := os.Getenv("PRISM_LIVE_SGLANG_URL")
	modelName := os.Getenv("PRISM_LIVE_SGLANG_MODEL")
	state := os.Getenv("PRISM_TEST_MODEL_STATE")
	if endpoint == "" || base == "" || modelName == "" || state == "" {
		t.Skip("set live Laya, SGLang, and model state variables")
	}
	model, err := llmruntime.NewOpenAICompatibleRuntime(llmruntime.Config{Engine: llmruntime.EngineSGLang, BaseURL: base, Model: modelName})
	if err != nil {
		t.Fatal(err)
	}
	root := makeTestRoot(t, map[string]string{"linear.md": linearSpec()}, map[string]string{"linear-issue-management": linearSkill()})
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "laya-live-issue-fixture", Version: "1"}, nil)
	mcpsdk.AddTool(server, &mcpsdk.Tool{Name: "find_issue", Description: "Find an issue by ID or keyword"}, func(_ context.Context, _ *mcpsdk.CallToolRequest, _ struct {
		Query string `json:"query"`
	}) (*mcpsdk.CallToolResult, map[string]string, error) { return nil, map[string]string{"id": "ENG-731", "title": "Blue Canary"}, nil })
	mcpsdk.AddTool(server, &mcpsdk.Tool{Name: "create_issue", Description: "Create a new issue"}, func(_ context.Context, _ *mcpsdk.CallToolRequest, _ struct {
		Title string `json:"title"`
	}) (*mcpsdk.CallToolResult, struct{}, error) { t.Error("model tried to create an issue for a read-only task"); return &mcpsdk.CallToolResult{IsError: true}, struct{}{}, nil })
	httpServer := httptest.NewServer(mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, nil))
	defer httpServer.Close()
	runner, err := New(Config{RootDir: root, ModelRuntime: model, DownstreamMCP: downstreammcp.New(downstreammcp.State{Servers: []downstreammcp.Server{{Name: "issues", Transport: downstreammcp.TransportStreamableHTTP, URL: httpServer.URL}}}), ToolModelStateDir: state, ToolRecommendationModel: "onnx", DecisionURL: endpoint, DecisionModel: "laya-english"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	task := "Use the issues MCP server to find issue ENG-731. Report its exact title and ID; do not guess or modify issues."
	start := time.Now()
	recommended, err := runner.RecommendTools(ctx, "linear", task, 2)
	if err != nil {
		t.Fatal(err)
	}
	if recommended.ScoreKind != "laya_choice" || len(recommended.Warnings) > 0 || len(recommended.Tools) != 2 || recommended.Tools[0].Name != "find_issue" {
		t.Fatalf("recommendations=%+v", recommended)
	}
	result, err := runner.Run(ctx, RunRequest{AgentID: "linear", Task: task + recommendationPrompt(recommended), SkillNames: []string{"linear-issue-management"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" || !strings.Contains(result.Summary, "Blue Canary") || !strings.Contains(result.Summary, "ENG-731") {
		t.Fatalf("status=%s summary=%q", result.Status, result.Summary)
	}
	t.Logf("Laya recommendation + SGLang tool task completed in %s: %s", time.Since(start), result.Summary)
}
