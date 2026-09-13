package cli

import (
	"testing"

	"github.com/bryanbarton525/prism/internal/downstreammcp"
)

func TestPrintDownstreamMCPMutationConflict(t *testing.T) {
	err := printDownstreamMCPMutation("linear", downstreammcp.OutcomeConflict)
	if err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestPrintDownstreamMCPMutationSuccessOutcomes(t *testing.T) {
	for _, outcome := range []string{
		downstreammcp.OutcomeCreated,
		downstreammcp.OutcomeUnchanged,
		downstreammcp.OutcomeReplaced,
	} {
		if err := printDownstreamMCPMutation("linear", outcome); err != nil {
			t.Fatalf("outcome %s: %v", outcome, err)
		}
	}
}

func TestParseReferenceAssignments(t *testing.T) {
	got := parseReferenceAssignments([]string{
		"Authorization=OPENAI_TOKEN",
		"X-Key = SOME_ENV ",
		"broken",
		"=missing",
		"missing=",
	})
	if got["Authorization"] != "OPENAI_TOKEN" {
		t.Fatalf("Authorization ref = %q", got["Authorization"])
	}
	if got["X-Key"] != "SOME_ENV" {
		t.Fatalf("X-Key ref = %q", got["X-Key"])
	}
	if _, ok := got["broken"]; ok {
		t.Fatalf("unexpected invalid assignment included: %#v", got)
	}
}
