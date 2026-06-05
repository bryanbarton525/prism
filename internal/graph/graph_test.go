package graph

import (
	"context"
	"strings"
	"testing"

	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/app"
	internalpolicy "github.com/bryanbarton525/prism/internal/policy"
	"github.com/bryanbarton525/prism/internal/result"
	graphpkg "github.com/bryanbarton525/prism/pkg/graph"
	"github.com/bryanbarton525/prism/pkg/observe"
	policypkg "github.com/bryanbarton525/prism/pkg/policy"
)

func TestValidateRejectsCycle(t *testing.T) {
	def := graphpkg.Definition{
		ID:      "cycle",
		Version: 1,
		Nodes: map[string]graphpkg.Node{
			"a": {DependsOn: []string{"b"}, Agent: "go-helper", Skills: []string{"go-helper-fn"}, Task: "a"},
			"b": {DependsOn: []string{"a"}, Agent: "go-helper", Skills: []string{"go-helper-fn"}, Task: "b"},
		},
	}
	res := Validate(def)
	if res.Valid {
		t.Fatalf("expected invalid cycle")
	}
}

func TestValidateRejectsUnsupportedParallelism(t *testing.T) {
	def := graphpkg.Definition{
		ID:      "parallel",
		Version: 1,
		Limits:  graphpkg.Limits{MaxParallel: 2},
		Nodes: map[string]graphpkg.Node{
			"a": {Agent: "go-helper", Skills: []string{"go-helper-fn"}, Task: "a"},
		},
	}
	res := Validate(def)
	if res.Valid {
		t.Fatalf("expected max_parallel > 1 to be invalid")
	}
}

func TestValidateRejectsMissingVersionTaskAndRetries(t *testing.T) {
	def := graphpkg.Definition{
		ID:     "invalid",
		Limits: graphpkg.Limits{MaxRetries: 1},
		Nodes: map[string]graphpkg.Node{
			"a": {Agent: "go-helper", Skills: []string{"go-helper-fn"}},
		},
	}
	res := Validate(def)
	if res.Valid {
		t.Fatalf("expected invalid graph")
	}
	text := strings.Join(res.Errors, "; ")
	for _, want := range []string{"version is required", "node \"a\" missing task", "node retries are not supported"} {
		if !strings.Contains(text, want) {
			t.Fatalf("errors %q missing %q", text, want)
		}
	}
}

func TestRunWithOptionsPolicyPrecheckDeniesBeforeNodeExecution(t *testing.T) {
	runner := &fakeGraphRunner{}
	policy := internalpolicy.New(policypkg.Policy{
		Version: 1,
		Agents: map[string]policypkg.Agent{
			"kubectl": {Allowed: false},
		},
		Sources: map[string]policypkg.Source{"cli": {Allowed: true}},
	})
	sink := &graphSink{}
	res, err := RunWithOptions(context.Background(), runner, graphDef(), RunOptions{Source: "cli", Policy: policy, EventSink: sink})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != result.StatusValidationFail {
		t.Fatalf("status = %s, want validation_fail", res.Status)
	}
	if runner.calls != 0 {
		t.Fatalf("runner should not execute nodes after policy denial, calls=%d", runner.calls)
	}
	if len(sink.events) != 1 || sink.events[0].EventKind != "graph" {
		t.Fatalf("expected aggregate graph event, got %#v", sink.events)
	}
}

func TestRunWithOptionsEmitsGraphEventAndPassesPriorArtifacts(t *testing.T) {
	runner := &fakeGraphRunner{}
	sink := &graphSink{}
	res, err := RunWithOptions(context.Background(), runner, graphDef(), RunOptions{Source: "cli", EventSink: sink})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != result.StatusOK {
		t.Fatalf("status = %s", res.Status)
	}
	if len(runner.requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(runner.requests))
	}
	if runner.requests[0].GraphID != "g" || runner.requests[0].GraphNodeID != "collect" {
		t.Fatalf("first request graph metadata = %#v", runner.requests[0])
	}
	if !strings.Contains(runner.requests[1].Task, "runtime-plugin:kubernetes") {
		t.Fatalf("second task missing prior artifact context: %s", runner.requests[1].Task)
	}
	if len(res.Artifacts) == 0 || res.Artifacts[0].NodeID == "" {
		t.Fatalf("typed artifacts not populated: %#v", res.Artifacts)
	}
	if len(sink.events) != 1 || sink.events[0].EventKind != "graph" || sink.events[0].Status != result.StatusOK {
		t.Fatalf("aggregate event = %#v", sink.events)
	}
}

type fakeGraphRunner struct {
	calls    int
	requests []app.RunRequest
}

func (f *fakeGraphRunner) ListAgents(context.Context) ([]agent.Summary, error) {
	return nil, nil
}

func (f *fakeGraphRunner) Run(_ context.Context, req app.RunRequest) (result.RunResult, error) {
	f.calls++
	f.requests = append(f.requests, req)
	return result.RunResult{
		AgentID: req.AgentID,
		Status:  result.StatusOK,
		Summary: "summary for " + req.GraphNodeID,
		Artifacts: []result.Artifact{{
			Type:    "evidence",
			Label:   "runtime-plugin:kubernetes",
			Content: strings.Repeat("artifact ", 100),
		}},
		SkillsUsed: req.SkillNames,
	}, nil
}

func (f *fakeGraphRunner) GetConstitution(context.Context, string) (app.Constitution, error) {
	return app.Constitution{}, nil
}

func (f *fakeGraphRunner) Doctor(context.Context) (result.DoctorResult, error) {
	return result.DoctorResult{}, nil
}

func (f *fakeGraphRunner) GetSpec(context.Context, string) (*agent.Spec, error) {
	return &agent.Spec{Tools: []string{"kubernetes"}}, nil
}

type graphSink struct {
	events []observe.RunEvent
}

func (g *graphSink) ObserveRun(_ context.Context, event observe.RunEvent) error {
	g.events = append(g.events, event)
	return nil
}

func graphDef() graphpkg.Definition {
	return graphpkg.Definition{
		ID:      "g",
		Version: 1,
		Limits:  graphpkg.Limits{MaxNodes: 2, MaxDepth: 2, MaxParallel: 1},
		Nodes: map[string]graphpkg.Node{
			"collect": {Agent: "kubectl", Skills: []string{"k8s-rollout-diagnostics"}, Task: "collect"},
			"analyze": {DependsOn: []string{"collect"}, Agent: "kubectl", Skills: []string{"kubectl-triage"}, Task: "analyze"},
		},
	}
}
