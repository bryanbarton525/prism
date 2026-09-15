package mcp

import (
	"net/url"
	"testing"
)

func TestReviewFileURIRejectsRemoteAndRelativeWorkspace(t *testing.T) {
	for _, raw := range []string{"file://remote.example/workspace", "file:relative"} {
		parsed, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fileURIPath(parsed); err == nil {
			t.Fatalf("accepted unsafe workspace URI %q", raw)
		}
	}
}
