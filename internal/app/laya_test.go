package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bryanbarton525/prism/internal/extensions"
)

func TestLayaRecommendationRespectsAccessAndFallsBack(t *testing.T) {
	for _, healthy := range []bool{true, false} {
		t.Run(fmt.Sprint(healthy), func(t *testing.T) {
			root := makeTestRoot(t, map[string]string{"linear.md": linearSpec()}, nil)
			fixture := &recommendationFixture{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					Questions map[string]struct {
						Criteria map[string]string `json:"criteria"`
					} `json:"questions"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				criteria := request.Questions["tool"].Criteria
				if len(criteria) != 2 {
					t.Errorf("unauthorized candidates sent: %v", criteria)
				}
				if healthy {
					// Reverse the initial retrieval order to establish that choice scores rerank it.
					fmt.Fprint(w, `{"answers":{"tool":{"type":"choice","probabilities":{"tool_0":0.1,"tool_1":0.9}}}}`)
				} else {
					fmt.Fprint(w, `{"answers":{"tool":{"type":"choice","probabilities":{}}}}`)
				}
			}))
			defer server.Close()
			runner, err := New(Config{RootDir: root, DownstreamMCP: fixture, DecisionURL: server.URL, DecisionModel: "laya-english", MCPAccessConfigured: true,
				MCPAccess: extensions.MCPAccessState{Agents: map[string]extensions.MCPAccessRule{"linear": {Mode: extensions.MCPAccessModeCustom, Servers: []string{"issues"}}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			got, err := runner.RecommendTools(context.Background(), "linear", "find open issues", 2)
			if err != nil {
				t.Fatal(err)
			}
			if len(fixture.listed) != 1 || fixture.listed[0] != "issues" || len(got.Tools) != 2 {
				t.Fatalf("catalog=%v result=%+v", fixture.listed, got)
			}
			if healthy {
				if got.ScoreKind != "laya_choice" || got.ModelIdentity != "laya-english" || got.Tools[0].Name != "create_issue" || len(got.Warnings) != 0 {
					t.Fatalf("reranking=%+v", got)
				}
			} else {
				if got.ScoreKind != "lexical_overlap" || got.Tools[0].Name != "find_issue" || len(got.Warnings) == 0 {
					t.Fatalf("fallback=%+v", got)
				}
			}
		})
	}
}
