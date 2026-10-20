package app

import "testing"

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
