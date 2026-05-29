package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bryanbarton525/prism/internal/app"
	"github.com/bryanbarton525/prism/internal/result"
)

// mockRunner satisfies app.AgentRunner without requiring a real Ollama instance.
type mockRunner struct{}

func (m *mockRunner) ListAgents(_ context.Context) ([]result.AgentSummary, error) {
	return []result.AgentSummary{
		{
			ID:              "mock-agent",
			Name:            "Mock Agent",
			Description:     "A mock agent for testing.",
			Model:           "llama3.1:8b",
			AllowedSkills:   []string{"mock-skill"},
			LatencyBudgetMs: 10000,
		},
	}, nil
}

func (m *mockRunner) Run(_ context.Context, _ app.RunRequest) (*result.RunResult, error) {
	return &result.RunResult{
		AgentID:    "mock-agent",
		Model:      "llama3.1:8b",
		Status:     "ok",
		Summary:    "Mock summary.",
		Confidence: "high",
	}, nil
}

func (m *mockRunner) GetConstitution(_ context.Context, agentID string) (*result.Constitution, error) {
	return &result.Constitution{AgentID: agentID, Text: "Mock constitution text."}, nil
}

func (m *mockRunner) Doctor(_ context.Context) (*result.DoctorResult, error) {
	return &result.DoctorResult{
		OllamaHost: "http://127.0.0.1:11434",
		AgentCount: 1,
		SkillCount: 1,
		Status:     "ok",
		Checks: []result.DoctorCheck{
			{Name: "ollama_connectivity", Status: "ok", Message: "reachable"},
		},
	}, nil
}

func TestMarshalJSON(t *testing.T) {
	obj := map[string]string{"key": "value"}
	out := marshalJSON(obj)
	var back map[string]string
	if err := json.Unmarshal([]byte(out), &back); err != nil {
		t.Fatalf("invalid JSON: %v — %s", err, out)
	}
	if back["key"] != "value" {
		t.Errorf("roundtrip mismatch: %v", back)
	}
}

func TestTextResult(t *testing.T) {
	r := textResult("hello world")
	if r == nil {
		t.Fatal("result is nil")
	}
	if len(r.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(r.Content))
	}
}

func TestStatusSummary(t *testing.T) {
	summary := StatusSummary(&mockRunner{})
	if summary == "" {
		t.Error("expected non-empty status summary")
	}
	if len(summary) < 10 {
		t.Errorf("status summary too short: %q", summary)
	}
}
