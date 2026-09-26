//go:build integration

package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bryanbarton525/prism/internal/downstreammcp"
	llmruntime "github.com/bryanbarton525/prism/internal/llm/runtime"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestLiveSGLangAgentFindsIssueThroughLocalMCP(t *testing.T) {
	base := os.Getenv("PRISM_LIVE_SGLANG_URL")
	modelName := os.Getenv("PRISM_LIVE_SGLANG_MODEL")
	state := os.Getenv("PRISM_TEST_MODEL_STATE")
	if base == "" || modelName == "" || state == "" {
		t.Skip("set PRISM_LIVE_SGLANG_URL, PRISM_LIVE_SGLANG_MODEL, and PRISM_TEST_MODEL_STATE")
	}
	model, err := llmruntime.NewOpenAICompatibleRuntime(llmruntime.Config{Engine: llmruntime.EngineSGLang, BaseURL: base, Model: modelName})
	if err != nil {
		t.Fatal(err)
	}
	root := makeTestRoot(t, map[string]string{"linear.md": linearSpec()}, map[string]string{"linear-issue-management": linearSkill()})
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "live-issue-fixture", Version: "1"}, nil)
	mcpsdk.AddTool(server, &mcpsdk.Tool{Name: "find_issue", Description: "Find an issue by issue ID or keyword"}, func(_ context.Context, _ *mcpsdk.CallToolRequest, _ struct {
		Query string `json:"query"`
	}) (*mcpsdk.CallToolResult, map[string]string, error) {
		return nil, map[string]string{"id": "ENG-731", "title": "Blue Canary"}, nil
	})
	httpServer := httptest.NewServer(mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, nil))
	defer httpServer.Close()
	runner, err := New(Config{RootDir: root, ModelRuntime: model, DownstreamMCP: downstreammcp.New(downstreammcp.State{Servers: []downstreammcp.Server{{Name: "issues", Transport: downstreammcp.TransportStreamableHTTP, URL: httpServer.URL}}}), ToolModelStateDir: state, ToolRecommendationAgents: []string{"linear"}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	result, err := runner.Run(ctx, RunRequest{AgentID: "linear", Task: "Use the issues MCP server to find issue ENG-731. Report its exact title and ID; do not guess.", SkillNames: []string{"linear-issue-management"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" || !strings.Contains(result.Summary, "Blue Canary") || !strings.Contains(result.Summary, "ENG-731") {
		t.Fatalf("result status=%s summary=%q artifacts=%+v", result.Status, result.Summary, result.Artifacts)
	}
}

// BenchmarkLiveSGLangIssueLookup measures completed Prism runs, not just
// embedding time. Run with -benchtime=6x or more for each baseline/backend.
func BenchmarkLiveSGLangIssueLookup(b *testing.B) {
	base := os.Getenv("PRISM_LIVE_SGLANG_URL")
	modelName := os.Getenv("PRISM_LIVE_SGLANG_MODEL")
	state := os.Getenv("PRISM_TEST_MODEL_STATE")
	if base == "" || modelName == "" || state == "" {
		b.Skip("set PRISM_LIVE_SGLANG_URL, PRISM_LIVE_SGLANG_MODEL, and PRISM_TEST_MODEL_STATE")
	}
	model, err := llmruntime.NewOpenAICompatibleRuntime(llmruntime.Config{Engine: llmruntime.EngineSGLang, BaseURL: base, Model: modelName})
	if err != nil {
		b.Fatal(err)
	}
	root := makeTestRoot(b, map[string]string{"linear.md": linearSpec()}, map[string]string{"linear-issue-management": linearSkill()})
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "live-issue-benchmark", Version: "1"}, nil)
	mcpsdk.AddTool(server, &mcpsdk.Tool{Name: "find_issue", Description: "Find an issue by issue ID or keyword"}, func(_ context.Context, _ *mcpsdk.CallToolRequest, _ struct {
		Query string `json:"query"`
	}) (*mcpsdk.CallToolResult, map[string]string, error) {
		return nil, map[string]string{"id": "ENG-731", "title": "Blue Canary"}, nil
	})
	httpServer := httptest.NewServer(mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, nil))
	defer httpServer.Close()
	downstream := downstreammcp.New(downstreammcp.State{Servers: []downstreammcp.Server{{Name: "issues", Transport: downstreammcp.TransportStreamableHTTP, URL: httpServer.URL}}})
	for _, mode := range []struct {
		name, backend string
		parallel      bool
	}{
		{name: "baseline", backend: ""},
		{name: "potion", backend: "potion"},
		{name: "onnx", backend: "onnx"},
		{name: "baseline_parallel", backend: "", parallel: true},
		{name: "potion_parallel", backend: "potion", parallel: true},
		{name: "onnx_parallel", backend: "onnx", parallel: true},
	} {
		b.Run(mode.name, func(b *testing.B) {
			b.StopTimer()
			cfg := Config{RootDir: root, ModelRuntime: model, DownstreamMCP: downstream}
			if mode.backend != "" {
				cfg.ToolModelStateDir = state
				cfg.ToolRecommendationModel = mode.backend
				cfg.ToolRecommendationAgents = []string{"linear"}
			}
			runner, err := New(cfg)
			if err != nil {
				b.Fatal(err)
			}
			var successes atomic.Int64
			run := func() {
				ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
				result, runErr := runner.Run(ctx, RunRequest{AgentID: "linear", Task: "Use the issues MCP server to find issue ENG-731. Report its exact title and ID; do not guess.", SkillNames: []string{"linear-issue-management"}})
				cancel()
				if runErr == nil && result.Status == "ok" && strings.Contains(result.Summary, "Blue Canary") && strings.Contains(result.Summary, "ENG-731") {
					successes.Add(1)
				}
			}
			b.StartTimer()
			if mode.parallel {
				b.SetParallelism(1)
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						run()
					}
				})
			} else {
				for i := 0; i < b.N; i++ {
					run()
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(successes.Load())/float64(b.N), "success/op")
		})
	}
}

func TestRecommendToolsWithLiveKevServer(t *testing.T) {
	kevURL := os.Getenv("PRISM_LIVE_KEV_URL")
	state := os.Getenv("PRISM_TEST_MODEL_STATE")
	if kevURL == "" || state == "" {
		t.Skip("set PRISM_LIVE_KEV_URL and PRISM_TEST_MODEL_STATE")
	}
	root := makeTestRoot(t, map[string]string{"linear.md": linearSpec()}, nil)
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "kev-issue-fixture", Version: "1"}, nil)
	mcpsdk.AddTool(server, &mcpsdk.Tool{Name: "find_issue", Description: "Find an issue by ID"}, func(_ context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, struct{}, error) {
		return nil, struct{}{}, nil
	})
	mcpsdk.AddTool(server, &mcpsdk.Tool{Name: "create_issue", Description: "Create an issue"}, func(_ context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, struct{}, error) {
		return nil, struct{}{}, nil
	})
	httpServer := httptest.NewServer(mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, nil))
	defer httpServer.Close()
	runner, err := New(Config{RootDir: root, DownstreamMCP: downstreammcp.New(downstreammcp.State{Servers: []downstreammcp.Server{{Name: "issues", Transport: downstreammcp.TransportStreamableHTTP, URL: httpServer.URL}}}), ToolModelStateDir: state, KevURL: kevURL})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	got, err := runner.RecommendTools(ctx, "linear", "Find issue ENG-731", 2)
	if err != nil {
		t.Fatal(err)
	}
	if got.ScoreKind != "kev_noul" || len(got.Tools) != 2 || got.ModelIdentity != "kev-latest" {
		t.Fatalf("live Kev recommendation: %+v", got)
	}
}

func TestRecommendToolsWithLiveKevTwentyCandidates(t *testing.T) {
	kevURL := os.Getenv("PRISM_LIVE_KEV_URL")
	state := os.Getenv("PRISM_TEST_MODEL_STATE")
	if kevURL == "" || state == "" {
		t.Skip("set PRISM_LIVE_KEV_URL and PRISM_TEST_MODEL_STATE")
	}
	root := makeTestRoot(t, map[string]string{"linear.md": linearSpec()}, nil)
	runner, err := New(Config{RootDir: root, DownstreamMCP: loadRecommendationFixture{}, ToolModelStateDir: state, KevURL: kevURL})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	got, err := runner.RecommendTools(ctx, "linear", "Find issue ENG-731", 5)
	if err != nil {
		t.Fatal(err)
	}
	if got.ScoreKind != "kev_noul" || len(got.Tools) != 5 {
		t.Fatalf("live 20-candidate Kev recommendation: %+v", got)
	}
}

func BenchmarkLiveKevRecommendation(b *testing.B) {
	kevURL := os.Getenv("PRISM_LIVE_KEV_URL")
	state := os.Getenv("PRISM_TEST_MODEL_STATE")
	if kevURL == "" || state == "" {
		b.Skip("set PRISM_LIVE_KEV_URL and PRISM_TEST_MODEL_STATE")
	}
	root := makeTestRoot(b, map[string]string{"linear.md": linearSpec()}, nil)
	for _, parallel := range []bool{false, true} {
		name := "sequential"
		if parallel {
			name = "parallel"
		}
		b.Run(name, func(b *testing.B) {
			runner, err := New(Config{RootDir: root, DownstreamMCP: loadRecommendationFixture{}, ToolModelStateDir: state, KevURL: kevURL})
			if err != nil {
				b.Fatal(err)
			}
			var successes atomic.Int64
			run := func() {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				got, err := runner.RecommendTools(ctx, "linear", "Find issue ENG-731", 5)
				cancel()
				if err == nil && got.ScoreKind == "kev_noul" && len(got.Tools) == 5 {
					successes.Add(1)
				}
			}
			b.ResetTimer()
			if parallel {
				b.SetParallelism(1)
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						run()
					}
				})
			} else {
				for i := 0; i < b.N; i++ {
					run()
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(successes.Load())/float64(b.N), "kev_success/op")
		})
	}
}
