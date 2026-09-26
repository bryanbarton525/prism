package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/bryanbarton525/prism/internal/downstreammcp"
	"github.com/bryanbarton525/prism/internal/extensions"
)

type recommendationFixture struct {
	listed []string
}

type loadRecommendationFixture struct{}

func TestRecommendToolsFindsRelevantToolInFiftyToolCatalog(t *testing.T) {
	state := os.Getenv("PRISM_TEST_MODEL_STATE")
	if state == "" {
		t.Skip("set PRISM_TEST_MODEL_STATE after models setup")
	}
	root := makeTestRoot(t, map[string]string{"linear.md": linearSpec()}, nil)
	for _, model := range []string{"potion", "onnx"} {
		t.Run(model, func(t *testing.T) {
			runner, err := New(Config{RootDir: root, DownstreamMCP: loadRecommendationFixture{}, ToolModelStateDir: state, ToolRecommendationModel: model})
			if err != nil {
				t.Fatal(err)
			}
			got, err := runner.RecommendTools(t.Context(), "linear", "Find issue ENG-731", 5)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, tool := range got.Tools {
				found = found || tool.Name == "find_issue"
			}
			if !found {
				t.Fatalf("find_issue missing from top five: %+v", got.Tools)
			}
		})
	}
}

func (loadRecommendationFixture) Servers() []downstreammcp.Server {
	return []downstreammcp.Server{{Name: "workspace"}}
}

func (loadRecommendationFixture) ListTools(context.Context, string, downstreammcp.ListToolsOptions) (downstreammcp.ListToolsResult, error) {
	tools := make([]downstreammcp.ToolSummary, 50)
	for i := range tools {
		tools[i] = downstreammcp.ToolSummary{Name: fmt.Sprintf("tool_%02d", i), Description: fmt.Sprintf("Inspect workspace artifact category %d and return matching records", i)}
	}
	tools[17] = downstreammcp.ToolSummary{Name: "find_issue", Description: "Find an issue by ID or keyword and return its status"}
	return downstreammcp.ListToolsResult{Tools: tools, Total: len(tools)}, nil
}

func (loadRecommendationFixture) CallTool(context.Context, string, string, map[string]any) (downstreammcp.CallResult, error) {
	panic("recommendation load test must not execute tools")
}

func BenchmarkRecommendToolsLoad(b *testing.B) {
	state := os.Getenv("PRISM_TEST_MODEL_STATE")
	if state == "" {
		b.Skip("set PRISM_TEST_MODEL_STATE after models setup")
	}
	root := makeTestRoot(b, map[string]string{"linear.md": linearSpec()}, nil)
	for _, model := range []string{"potion", "onnx"} {
		for _, parallel := range []bool{false, true} {
			name := model + "/sequential"
			if parallel {
				name = model + "/parallel"
			}
			b.Run(name, func(b *testing.B) {
				runner, err := New(Config{RootDir: root, DownstreamMCP: loadRecommendationFixture{}, ToolModelStateDir: state, ToolRecommendationModel: model})
				if err != nil {
					b.Fatal(err)
				}
				if got, err := runner.RecommendTools(context.Background(), "linear", "Find issue ENG-731", 3); err != nil || len(got.Tools) == 0 {
					b.Fatalf("warmup: %+v, %v", got, err)
				}
				b.ResetTimer()
				if parallel {
					b.SetParallelism(2)
					b.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							if _, err := runner.RecommendTools(context.Background(), "linear", "Find issue ENG-731", 3); err != nil {
								b.Error(err)
							}
						}
					})
				} else {
					for i := 0; i < b.N; i++ {
						if _, err := runner.RecommendTools(context.Background(), "linear", "Find issue ENG-731", 3); err != nil {
							b.Fatal(err)
						}
					}
				}
			})
		}
	}
}

func TestRecommendToolsWithInstalledModels(t *testing.T) {
	state := os.Getenv("PRISM_TEST_MODEL_STATE")
	if state == "" {
		t.Skip("set PRISM_TEST_MODEL_STATE after models setup")
	}
	root := makeTestRoot(t, map[string]string{"linear.md": linearSpec()}, nil)
	for _, model := range []string{"potion", "onnx"} {
		t.Run(model, func(t *testing.T) {
			runner, err := New(Config{AgentDir: root + "/agents", SkillsDir: root + "/skills", DownstreamMCP: &recommendationFixture{}, ToolModelStateDir: state, ToolRecommendationModel: model})
			if err != nil {
				t.Fatal(err)
			}
			got, err := runner.RecommendTools(context.Background(), "linear", "Find open issues by keyword", 2)
			if err != nil {
				t.Fatal(err)
			}
			if got.ScoreKind != model+"_cosine" {
				t.Fatalf("backend = %q, warnings = %v", got.ScoreKind, got.Warnings)
			}
			if len(got.Tools) != 2 || got.Tools[0].Name != "find_issue" {
				t.Fatalf("ranking = %+v", got.Tools)
			}
		})
	}
}

func (f *recommendationFixture) Servers() []downstreammcp.Server {
	return []downstreammcp.Server{{Name: "issues"}, {Name: "admin"}}
}

func (f *recommendationFixture) ListTools(_ context.Context, server string, _ downstreammcp.ListToolsOptions) (downstreammcp.ListToolsResult, error) {
	f.listed = append(f.listed, server)
	if server == "admin" {
		return downstreammcp.ListToolsResult{Tools: []downstreammcp.ToolSummary{{Name: "delete_all", Description: "Delete all issues"}}, Total: 1}, nil
	}
	return downstreammcp.ListToolsResult{Tools: []downstreammcp.ToolSummary{
		{Name: "find_issue", Description: "Search open issues by keywords", InputSchema: map[string]any{"type": "object"}},
		{Name: "create_issue", Description: "Create a new issue"},
	}, Total: 2}, nil
}

func (f *recommendationFixture) CallTool(context.Context, string, string, map[string]any) (downstreammcp.CallResult, error) {
	panic("recommendation must not execute a tool")
}

func TestRecommendToolsOnlyDiscoversAuthorizedServers(t *testing.T) {
	root := makeTestRoot(t, map[string]string{"linear.md": linearSpec()}, nil)
	fixture := &recommendationFixture{}
	runner, err := New(Config{
		AgentDir: root + "/agents", SkillsDir: root + "/skills", DownstreamMCP: fixture,
		MCPAccessConfigured: true,
		MCPAccess: extensions.MCPAccessState{Agents: map[string]extensions.MCPAccessRule{
			"linear": {Mode: extensions.MCPAccessModeCustom, Servers: []string{"issues"}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := runner.RecommendTools(context.Background(), "linear", "find open issues", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixture.listed) != 1 || fixture.listed[0] != "issues" {
		t.Fatalf("discovered %v", fixture.listed)
	}
	if len(got.Tools) != 1 || got.Tools[0].Name != "find_issue" || got.Tools[0].Server != "issues" {
		t.Fatalf("recommendations: %+v", got)
	}
	if strings.Contains(got.Tools[0].Description, "delete") {
		t.Fatalf("unauthorized recommendation: %+v", got)
	}
	if _, err := runner.RecommendTools(context.Background(), "linear", "create an issue", 1); err != nil {
		t.Fatal(err)
	}
	if len(fixture.listed) != 1 {
		t.Fatalf("cached catalog listed %d times", len(fixture.listed))
	}
}
