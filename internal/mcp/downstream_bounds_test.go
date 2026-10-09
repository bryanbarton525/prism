package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bryanbarton525/prism/internal/downstreammcp"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCallMCPToolCanRequestCompleteLargeResult(t *testing.T) {
	ctx := context.Background()
	payload := strings.Repeat("CVE-2026-1234\n", 5000) + "FINAL_CVE"
	downstream := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "kubescape-fixture"}, nil)
	mcpsdk.AddTool(downstream, &mcpsdk.Tool{Name: "vulnerabilities"}, func(context.Context, *mcpsdk.CallToolRequest, struct{}) (*mcpsdk.CallToolResult, map[string]any, error) {
		return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: payload}}}, map[string]any{"cves": payload}, nil
	})
	httpServer := httptest.NewServer(mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return downstream }, nil))
	defer httpServer.Close()
	client := downstreammcp.New(downstreammcp.State{Servers: []downstreammcp.Server{{Name: "kubescape", Transport: downstreammcp.TransportStreamableHTTP, URL: httpServer.URL, MaxBytes: 50000}}})
	prism := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "prism-test"}, nil)
	registerTools(prism, mcpFakeRunner{}, Config{DownstreamMCP: client})
	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	serverSession, err := prism.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	host := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "host"}, nil)
	session, err := host.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	call := func(maxBytes int) downstreammcp.CallResult {
		t.Helper()
		args := map[string]any{"server": "kubescape", "tool": "vulnerabilities"}
		if maxBytes != 0 {
			args["max_bytes"] = maxBytes
		}
		res, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "call_mcp_tool", Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		if res.IsError {
			t.Fatalf("call failed: %s", res.Content[0].(*mcpsdk.TextContent).Text)
		}
		raw, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		var result downstreammcp.CallResult
		if err := json.Unmarshal(raw, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	if result := call(0); !result.Truncated || strings.Contains(result.Content, "FINAL_CVE") {
		t.Fatal("fixture did not reproduce the configured cap")
	}
	result := call(200000)
	if result.Truncated || result.Content != payload {
		t.Fatalf("explicit larger budget still lost CVEs: truncated=%v bytes=%d", result.Truncated, len(result.Content))
	}
	if cves, ok := result.StructuredContent.(map[string]any); !ok || cves["cves"] != payload {
		t.Fatal("structured CVEs were lost")
	}
	if result := call(0); !result.Truncated {
		t.Fatal("per-call override changed the configured default")
	}
}
