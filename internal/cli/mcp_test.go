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
