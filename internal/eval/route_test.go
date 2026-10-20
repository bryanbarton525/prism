package eval

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bryanbarton525/prism/internal/llm/runtime"
)

type orderedRuntime struct {
	fixtureRuntime
	requests *int
	bypass   bool
}

func (r *orderedRuntime) Chat(ctx context.Context, req runtime.ChatRequest) (*runtime.ChatResponse, error) {
	if *r.requests == 0 {
		panic("generation before Laya")
	}
	out, err := r.fixtureRuntime.Chat(ctx, req)
	if r.bypass && len(out.Message.ToolCalls) > 0 {
		out.Message.ToolCalls[0].Function.Arguments["server"] = "unauthorized"
	}
	return out, err
}

func TestDecisionFirstOrderEnforcementAndOptionalDefault(t *testing.T) {
	for _, scenario := range []string{"selected", "bypass", "unavailable", "unconfigured"} {
		t.Run(scenario, func(t *testing.T) {
			suite := fixtureSuite(t)
			suite.Cases[0].Tools[0].Server = "custom-tracker"
			suite.Cases[0].Task = "Retrieve EVAL-17 and report its current status and assignee in decision."
			count := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count++
				if scenario == "unavailable" {
					http.Error(w, "unavailable", 503)
					return
				}
				var request struct {
					State     string `json:"state"`
					Questions map[string]struct {
						Criteria map[string]string `json:"criteria"`
					} `json:"questions"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				probabilities := map[string]float64{}
				for key, description := range request.Questions["tool"].Criteria {
					probabilities[key] = 0
					if (!strings.Contains(request.State, "call_mcp_tool:") && strings.Contains(description, "custom-tracker.get_issue")) || (strings.Contains(request.State, "call_mcp_tool:") && strings.HasPrefix(description, "finish:")) {
						probabilities[key] = 1
					}
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"answers": map[string]any{"tool": map[string]any{"type": "choice", "probabilities": probabilities}}})
			}))
			defer server.Close()
			options := Options{Models: []string{"test"}, Repeat: 1, Timeout: 10 * time.Second, Generation: Generation{MaxTokens: 1024}, LayaURL: server.URL}
			var model runtime.ModelRuntime = &orderedRuntime{fixtureRuntime: fixtureRuntime{fixture: suite.Cases[0]}, requests: &count, bypass: scenario == "bypass"}
			if scenario == "unconfigured" {
				options.LayaURL = ""
				model = &fixtureRuntime{fixture: suite.Cases[0]}
			}
			report, err := Run(context.Background(), suite, model, options)
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateReport(report); err != nil {
				t.Fatal(err)
			}
			trial := report.Trials[0]
			if scenario == "selected" {
				if !trial.Grade.Passed || len(trial.Routes) != 2 || count != 2 {
					t.Fatalf("trial=%+v requests=%d", trial, count)
				}
			}
			if scenario == "unconfigured" {
				if count != 0 || len(trial.Routes) != 0 || !trial.Grade.Passed {
					t.Fatalf("optional default failed: %+v", trial)
				}
			}
			if scenario == "bypass" || scenario == "unavailable" {
				if trial.Grade.Passed || len(trial.ToolCalls) > 0 || len(trial.Routes) == 0 || trial.Routes[0].Error == "" {
					t.Fatalf("routing failure escaped: %+v", trial)
				}
			}
			if scenario == "unavailable" && len(trial.Calls) > 0 {
				t.Fatal("configured routing silently fell back to generation")
			}
		})
	}
}

func TestDecisionContractSurvivesProductionMCPAndCannotBeSpoofed(t *testing.T) {
	suite, err := LoadSuite("")
	if err != nil {
		t.Fatal(err)
	}
	suite.Cases = suite.Cases[:1]
	report := runScript(t, suite, &scriptedRuntime{}, "test")
	trial := report.Trials[0]
	if !trial.Grade.Passed || trial.Result.Decision["assignee"] != "Alex Rivera" {
		t.Fatalf("contract lost: %+v", trial)
	}
	trial.Result.Decision = nil
	if GradeTrial(suite.Cases[0], trial).Passed {
		t.Fatal("raw output without structured delivery passed")
	}
	for _, call := range report.Trials[0].Calls[:1] {
		data, _ := json.Marshal(call.Request)
		if !strings.Contains(string(data), "Required decision schema") || strings.Contains(string(data), "Alex Rivera") {
			t.Fatal("schema missing or expected value leaked")
		}
	}
}
