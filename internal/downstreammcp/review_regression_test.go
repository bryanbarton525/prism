package downstreammcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReviewStrictConfigurationAndStructuredBounds(t *testing.T) {
	for _, server := range []Server{
		{Name: "bad-command", Transport: TransportCommand, Command: "cmd", URL: "https://example.com"},
		{Name: "bad-http", Transport: TransportSSE, URL: "https://example.com", Command: "cmd"},
		{Name: "bad-ref", Transport: TransportSSE, URL: "https://example.com", HeaderRefs: map[string]string{"": "TOKEN"}},
	} {
		if err := server.Validate(); err == nil {
			t.Fatalf("mixed or malformed configuration accepted: %#v", server)
		}
	}
	value, truncated := boundedStructuredContent(map[string]any{"large": strings.Repeat("x", 1000)}, 32)
	if !truncated {
		t.Fatal("large structured content was not truncated")
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > 32 {
		t.Fatalf("bounded structured content is %d bytes", len(data))
	}
}
