package eval

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bryanbarton525/prism/internal/llm/runtime"
	"github.com/bryanbarton525/prism/internal/result"
)

type scriptedRuntime struct {
	runtime.ModelRuntime
	wrong, skip, malformed bool
	sequence               int
}

func (*scriptedRuntime) Engine() runtime.Engine { return runtime.EngineOllama }
func (s *scriptedRuntime) Chat(_ context.Context, r runtime.ChatRequest) (*runtime.ChatResponse, error) {
	s.sequence++
	hasResult := false
	for _, message := range r.Messages {
		if message.Role == "tool" {
			hasResult = true
		}
	}
	response := &runtime.ChatResponse{Model: r.Model, Message: runtime.Message{Role: "assistant"}}
	if !hasResult && !s.skip {
		response.Message.ToolCalls = []runtime.ToolCall{{ID: "eval-call", Type: "function", Function: runtime.ToolCallFunction{Name: "call_mcp_tool", Arguments: map[string]any{"server": "tracker", "tool": "get_issue", "arguments": map[string]any{"id": "EVAL-17"}}}}}
		return response, nil
	}
	status := "In Review"
	if s.wrong {
		status = "Closed"
	}
	object := map[string]any{"summary": strings.Repeat("Reviewed. ", s.sequence%3+1), "findings": []string{}, "confidence": "high", "decision": map[string]any{"issue_id": "EVAL-17", "status": status, "assignee": "Alex Rivera"}}
	data, _ := json.Marshal(object)
	response.Message.Content = string(data)
	if s.malformed {
		response.Message.Content = "This is just prose."
	}
	return response, nil
}
func fixtureSuite(t *testing.T) Suite {
	t.Helper()
	s, err := LoadSuite("../../evals/mcp-response-v1/suite.yaml")
	if err != nil {
		t.Fatal(err)
	}
	s.Cases = s.Cases[:1]
	return s
}
func runScript(t *testing.T, s Suite, rt *scriptedRuntime, model string) Report {
	t.Helper()
	r, err := Run(context.Background(), s, rt, Options{Models: []string{model}, Repeat: 3, Timeout: 10 * time.Second, Generation: Generation{Temperature: 0.1, MaxTokens: 1024}})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestProductionMCPLoopAndBaseline(t *testing.T) {
	suite := fixtureSuite(t)
	r := runScript(t, suite, &scriptedRuntime{}, "approved")
	for _, trial := range r.Trials {
		if !trial.Grade.Passed {
			data, _ := json.MarshalIndent(trial, "", "  ")
			t.Fatalf("production loop failed: %s", data)
		}
		if len(trial.Calls) != 2 || len(trial.ToolCalls) != 1 {
			t.Fatalf("expected real bridge loop: %+v", trial)
		}
	}
	baseline, err := CreateBaseline(r, "approved")
	if err != nil {
		t.Fatal(err)
	}
	candidate := runScript(t, suite, &scriptedRuntime{}, "candidate")
	regressions, err := Compare(baseline, candidate, "candidate")
	if err != nil || len(regressions) != 0 {
		t.Fatalf("harmless wording variation rejected: %v %v", regressions, err)
	}
	for _, test := range []struct {
		name    string
		runtime *scriptedRuntime
	}{{"incorrect answer", &scriptedRuntime{wrong: true}}, {"skipped MCP", &scriptedRuntime{skip: true}}, {"invalid schema", &scriptedRuntime{malformed: true}}} {
		t.Run(test.name, func(t *testing.T) {
			bad := runScript(t, suite, test.runtime, "candidate")
			regressions, err := Compare(baseline, bad, "candidate")
			if err != nil || len(regressions) == 0 {
				t.Fatalf("regression not detected: %v %v", regressions, err)
			}
			if _, err := CreateBaseline(bad, "candidate"); err == nil {
				t.Fatal("weak run approved")
			}
		})
	}
	dir := filepath.Join(t.TempDir(), "dataset")
	if err := Export(r, dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"trials.jsonl", "summary.csv", "suite.json", "metadata.json", "manifest.json", "metrics.jsonl", "README.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := Export(r, dir); err == nil {
		t.Fatal("dataset overwritten")
	}
}
func TestReportIntegrityAndCompatibility(t *testing.T) {
	suite := fixtureSuite(t)
	r := runScript(t, suite, &scriptedRuntime{}, "approved")
	baseline, err := CreateBaseline(r, "approved")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("missing trials", func(t *testing.T) {
		copy := r
		copy.Trials = r.Trials[:2]
		if _, err := CreateBaseline(copy, "approved"); err == nil {
			t.Fatal("accepted incomplete matrix")
		}
	})
	t.Run("edited grade", func(t *testing.T) {
		data, _ := json.Marshal(r)
		var copy Report
		_ = json.Unmarshal(data, &copy)
		copy.Trials[0].Grade.Passed = false
		if err := ValidateReport(copy); err == nil {
			t.Fatal("trusted edited grade")
		}
	})
	t.Run("suite changed", func(t *testing.T) {
		copy := baseline
		copy.SuiteHash = "different"
		if _, err := Compare(copy, r, "approved"); err == nil {
			t.Fatal("accepted mismatched suite")
		}
	})
	t.Run("generation changed", func(t *testing.T) {
		copy := baseline
		copy.Generation.Temperature = 0.8
		if _, err := Compare(copy, r, "approved"); err == nil {
			t.Fatal("accepted mismatched settings")
		}
	})
	t.Run("single trial", func(t *testing.T) {
		copy := r
		copy.Repeat = 1
		copy.Trials = r.Trials[:1]
		if _, err := CreateBaseline(copy, "approved"); err == nil {
			t.Fatal("approved one-trial baseline")
		}
	})
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := WriteJSON(path, baseline); err != nil {
		t.Fatal(err)
	}
	if err := WriteJSON(path, baseline); err == nil {
		t.Fatal("baseline overwritten")
	}
}
func TestSemanticConsistencyAndRequiredChecks(t *testing.T) {
	suite := fixtureSuite(t)
	r := runScript(t, suite, &scriptedRuntime{}, "approved")
	c := suite.Cases[0]
	trial := r.Trials[0]
	t.Run("wrong arguments", func(t *testing.T) {
		copy := trial
		copy.ToolCalls = append([]FixtureCall(nil), trial.ToolCalls...)
		copy.ToolCalls[0].Matched = false
		if GradeTrial(c, copy).Passed {
			t.Fatal("wrong arguments passed")
		}
	})
	t.Run("wrong served model", func(t *testing.T) {
		copy := trial
		copy.Model = "unexpected"
		if GradeTrial(c, copy).Passed {
			t.Fatal("wrong model passed")
		}
	})
	t.Run("semantic drift", func(t *testing.T) {
		data, _ := json.Marshal(r)
		var copy Report
		_ = json.Unmarshal(data, &copy)
		copy.Trials[0].Result.RawOutput = strings.ReplaceAll(copy.Trials[0].Result.RawOutput, "In Review", "Closed")
		copy.Trials[0].Grade = GradeTrial(c, copy.Trials[0])
		score := Scores(copy, "approved")[c.ID]
		if score.Consistency != 2.0/3 || score.PassRate != 2.0/3 {
			t.Fatalf("bad scores %+v", score)
		}
	})
}

// This is a deterministic harness test, not measured model performance.
type fixtureRuntime struct {
	runtime.ModelRuntime
	fixture Case
}

func (*fixtureRuntime) Engine() runtime.Engine { return runtime.EngineOllama }
func (r *fixtureRuntime) Chat(_ context.Context, req runtime.ChatRequest) (*runtime.ChatResponse, error) {
	response := &runtime.ChatResponse{Model: req.Model, Message: runtime.Message{Role: "assistant"}}
	evidenceCount := 0
	for _, message := range req.Messages {
		if message.Role == "tool" {
			evidenceCount++
		}
	}
	if evidenceCount < len(r.fixture.Tools) {
		f := r.fixture.Tools[evidenceCount]
		response.Message.ToolCalls = []runtime.ToolCall{{ID: "fixture-call", Type: "function", Function: runtime.ToolCallFunction{Name: "call_mcp_tool", Arguments: map[string]any{"server": f.Server, "tool": f.Name, "arguments": f.Arguments}}}}
		return response, nil
	}
	decision := map[string]any{}
	for _, check := range r.fixture.Checks {
		decision[strings.TrimPrefix(check.Path, "/decision/")] = check.Equals
	}
	data, _ := json.Marshal(map[string]any{"summary": "Fixture response verified.", "findings": []string{}, "confidence": "high", "decision": decision})
	response.Message.Content = string(data)
	return response, nil
}
func TestAllBundledReplayCases(t *testing.T) {
	suite, err := LoadSuite("")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range suite.Cases {
		t.Run(c.ID, func(t *testing.T) {
			single := suite
			single.Cases = []Case{c}
			report, err := Run(context.Background(), single, &fixtureRuntime{fixture: c}, Options{Models: []string{"fixture-model"}, Repeat: 1, Timeout: 10 * time.Second, Generation: Generation{Temperature: 0.1, MaxTokens: 1024}})
			if err != nil {
				t.Fatal(err)
			}
			trial := report.Trials[0]
			if !trial.Grade.Passed {
				data, _ := json.MarshalIndent(trial, "", "  ")
				t.Fatal(string(data))
			}
			if len(trial.ToolCalls) != len(c.Tools) || !trial.ToolCalls[0].Matched {
				t.Fatal("replay tool was not called correctly")
			}
		})
	}
}
func TestExpectedAnswersNotInInitialPrompt(t *testing.T) {
	report := runScript(t, fixtureSuite(t), &scriptedRuntime{}, "approved")
	for _, trial := range report.Trials {
		data, _ := json.Marshal(trial.Calls[0].Request)
		if strings.Contains(string(data), "Alex Rivera") {
			t.Fatal("hidden answer leaked into initial prompt")
		}
	}
}

func TestToolSequenceAndArtifactSpoofing(t *testing.T) {
	suite := fixtureSuite(t)
	report := runScript(t, suite, &scriptedRuntime{}, "approved")
	trial := report.Trials[0]
	c := suite.Cases[0]
	c.ExpectedToolSequence = []ToolRef{{Server: "tracker", Tool: "get_issue"}, {Server: "projects", Tool: "get_project"}}
	trial.ToolCalls = append(trial.ToolCalls, FixtureCall{Server: "projects", Tool: "get_project", Matched: true})
	if !GradeTrial(c, trial).Checks["tool_sequence"] {
		t.Fatal("correct order failed")
	}
	trial.ToolCalls[0], trial.ToolCalls[1] = trial.ToolCalls[1], trial.ToolCalls[0]
	if GradeTrial(c, trial).Checks["tool_sequence"] {
		t.Fatal("reversed calls passed")
	}
	c.RequiredBridgeCalls = []string{"read_tool_result"}
	trial.Result.Artifacts = append(trial.Result.Artifacts, result.Artifact{Type: "mcp_tool_call", Label: "mcp-tool:read_tool_result"})
	if GradeTrial(c, trial).Checks["required_bridge:read_tool_result"] {
		t.Fatal("model-written artifact counted as execution")
	}
}

func TestMetricDiagnostics(t *testing.T) {
	r := runScript(t, fixtureSuite(t), &scriptedRuntime{wrong: true}, "candidate")
	found := false
	for _, metric := range r.Trials[0].Grade.Metrics {
		if strings.HasSuffix(metric.Name, "/decision/status") {
			found = true
			if metric.Passed || metric.Score != 0 || metric.Threshold != 1 || !strings.Contains(metric.Reason, "In Review") || !strings.Contains(metric.Reason, "Closed") {
				t.Fatalf("missing diagnostic: %+v", metric)
			}
		}
	}
	if !found {
		t.Fatal("outcome metric missing")
	}
}
func TestCancelledRunCannotBecomeBaseline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, err := Run(ctx, fixtureSuite(t), &scriptedRuntime{}, Options{Models: []string{"approved"}, Repeat: 3, Timeout: time.Second, Generation: Generation{MaxTokens: 1024}})
	if err == nil || r.Completed {
		t.Fatal("cancelled run marked complete")
	}
	if _, err := CreateBaseline(r, "approved"); err == nil {
		t.Fatal("cancelled baseline approved")
	}
}
