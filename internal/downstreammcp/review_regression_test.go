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

func TestHTTPEndpointRejectsURLCredentialsBeforePersistence(t *testing.T) {
	for _, transport := range []string{TransportSSE, TransportStreamableHTTP} {
		server := Server{Name: "secret", Transport: transport, URL: "https://user:password@example.com/mcp"}
		err := server.Validate()
		if err == nil || !strings.Contains(err.Error(), "userinfo") || strings.Contains(err.Error(), "password") {
			t.Fatalf("%s URL credentials: %v", transport, err)
		}
		if err := Save(t.TempDir()+"/mcp.yaml", State{Servers: []Server{server}}); err == nil {
			t.Fatalf("%s URL credentials were persisted", transport)
		}
	}
}
