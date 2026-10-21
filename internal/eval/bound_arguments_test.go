package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bryanbarton525/prism/internal/llm/runtime"
)

type boundArgumentsRuntime struct {
	fixtureRuntime
	shape string
}

func (r *boundArgumentsRuntime) Chat(ctx context.Context, req runtime.ChatRequest) (*runtime.ChatResponse, error) {
	response, err := r.fixtureRuntime.Chat(ctx, req)
	if err != nil || len(response.Message.ToolCalls) == 0 {
		return response, err
	}
	properties, _ := req.Tools[0].Function.Parameters["properties"].(map[string]any)
	if properties["id"] == nil || properties["arguments"] != nil {
		return nil, fmt.Errorf("bound routing must advertise the downstream argument schema, got %v", properties)
	}
	f := r.fixture.Tools[0]
	args := map[string]any{"id": f.Arguments["id"]}
	switch r.shape {
	case "legacy nested":
		args = map[string]any{"server": f.Server, "tool": f.Name, "arguments": args}
	case "legacy flat":
		args["server"], args["tool"] = f.Server, f.Name
	case "bypass":
		args = map[string]any{"server": "another-server", "tool": f.Name, "arguments": args}
	case "invalid":
		args["id"] = 123
	}
	response.Message.ToolCalls[0].Function.Arguments = args
	return response, nil
}

func TestBoundRoutingAdvertisesArgumentsAndBindsDestination(t *testing.T) {
	for _, shape := range []string{"direct", "legacy nested", "legacy flat", "bypass", "invalid"} {
		t.Run(shape, func(t *testing.T) {
			suite := fixtureSuite(t)
			suite.Cases[0].Tools[0].Server = "custom-tracker"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				var body struct {
					State     string `json:"state"`
					Questions map[string]struct {
						Criteria map[string]string `json:"criteria"`
					} `json:"questions"`
				}
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				prefix := "custom-tracker.get_issue:"
				if strings.Contains(body.State, "call_mcp_tool:") {
					prefix = "finish:"
				}
				scores := map[string]float64{}
				for key, description := range body.Questions["tool"].Criteria {
					scores[key] = 0
					if strings.HasPrefix(description, prefix) {
						scores[key] = 1
					}
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"answers": map[string]any{"tool": map[string]any{"type": "choice", "probabilities": scores}}})
			}))
			defer server.Close()
			report, err := Run(context.Background(), suite, &boundArgumentsRuntime{fixtureRuntime: fixtureRuntime{fixture: suite.Cases[0]}, shape: shape}, Options{Models: []string{"test"}, Repeat: 1, Timeout: 10 * time.Second, Generation: Generation{MaxTokens: 1024}, LayaURL: server.URL})
			if err != nil {
				t.Fatal(err)
			}
			trial := report.Trials[0]
			wantPass := shape != "bypass" && shape != "invalid"
			if trial.Grade.Passed != wantPass {
				t.Fatalf("passed=%t, want %t: %s", trial.Grade.Passed, wantPass, trial.Result.Summary)
			}
			if wantPass {
				if len(trial.ToolCalls) != 1 || trial.ToolCalls[0].Server != "custom-tracker" || !trial.ToolCalls[0].Matched {
					t.Fatal("runtime destination or arguments changed")
				}
			} else if len(trial.ToolCalls) != 0 {
				t.Fatal("invalid arguments reached the downstream tool")
			}
		})
	}
}
