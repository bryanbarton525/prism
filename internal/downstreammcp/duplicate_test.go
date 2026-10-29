package downstreammcp

import "testing"

func TestEquivalentJSONPreservesDistinctEvidence(t *testing.T) {
	for _, tc := range []struct {
		text  string
		value any
		equal bool
	}{
		{` { "b":2, "a":"ticket" } `, map[string]any{"a": "ticket", "b": 2}, true},
		{`{"id":9007199254740993}`, map[string]any{"id": 9007199254740992}, false},
		{`{"id":"x"} warning`, map[string]any{"id": "x"}, false},
		{`warning`, map[string]any{"id": "x"}, false},
		{`{"id":"x"}`, map[string]any{"id": "y"}, false},
	} {
		if got := equivalentJSON(tc.text, tc.value); got != tc.equal {
			t.Fatalf("%q: %v", tc.text, got)
		}
	}
}
