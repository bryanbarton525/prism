package app

import (
	"reflect"
	"testing"
)

func TestSelectedRoutePreservesFieldsNamedLikeRoutingMetadata(t *testing.T) {
	selected := &ToolRecommendation{Server: "authorized", Name: "inspect", InputSchema: map[string]any{
		"type": "object", "additionalProperties": false,
		"required": []string{"server", "tool", "arguments"},
		"properties": map[string]any{
			"server":    map[string]any{"type": "string"},
			"tool":      map[string]any{"type": "string"},
			"arguments": map[string]any{"type": "object"},
		},
	}}
	input := map[string]any{"server": "application-data", "tool": "a-record-field", "arguments": map[string]any{"value": "retained"}}
	got, err := selectedRouteArguments(selected, input)
	if err != nil || !reflect.DeepEqual(got, input) {
		t.Fatalf("valid downstream data was interpreted as a destination: %v %v", got, err)
	}
}

func TestDecisionRoutingIsOptional(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  Config
		want bool
	}{
		{"default", Config{}, false},
		{"agent enabled without endpoint", Config{ToolRecommendationAgents: []string{"linear"}, DecisionModel: "laya-english"}, false},
		{"endpoint without agent opt-in", Config{DecisionURL: "http://unused.invalid", DecisionModel: "laya-english"}, false},
		{"different decision model", Config{ToolRecommendationAgents: []string{"linear"}, DecisionURL: "http://unused.invalid", DecisionModel: "kev"}, false},
		{"configured", Config{ToolRecommendationAgents: []string{"linear"}, DecisionURL: "http://unused.invalid", DecisionModel: "laya-english"}, true},
		{"legacy advisory", Config{ToolRecommendationAgents: []string{"linear"}, DecisionURL: "http://unused.invalid", DecisionModel: "laya-english", DecisionRoutingMode: "advisory"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &Runner{cfg: tc.cfg}
			if got := r.decisionRoutingEnabled("linear"); got != tc.want {
				t.Fatalf("enabled = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestInvalidDecisionRoutingMode(t *testing.T) {
	if _, err := New(Config{DecisionRoutingMode: "typo"}); err == nil {
		t.Fatal("invalid routing mode accepted")
	}
}
