package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
		if len(res.Content) != 0 {
			t.Fatalf("duplicate wire text: %d blocks", len(res.Content))
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

func TestTicketResultAppearsOnceOnWire(t *testing.T) {
	ctx := context.Background()
	ticket := map[string]any{"id": "ticket-unique-123", "identifier": "BBA-31", "title": "MCP efficiency comparison", "description": strings.Repeat("Retain every issue field without duplicating evidence. ", 20), "url": "https://linear.app/example/issue/BBA-31", "status": "Done"}
	downstream := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "ticket-fixture"}, nil)
	mcpsdk.AddTool(downstream, &mcpsdk.Tool{Name: "get_issue"}, func(context.Context, *mcpsdk.CallToolRequest, struct{}) (*mcpsdk.CallToolResult, map[string]any, error) {
		return nil, ticket, nil
	})
	httpServer := httptest.NewServer(mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return downstream }, nil))
	defer httpServer.Close()
	client := downstreammcp.New(downstreammcp.State{Servers: []downstreammcp.Server{{Name: "linear", Transport: downstreammcp.TransportStreamableHTTP, URL: httpServer.URL}}})
	retained, err := client.CallTool(ctx, "linear", "get_issue", nil)
	if err != nil {
		t.Fatal(err)
	}
	if retained.Content != "" || strings.Count(retained.FullContent, "ticket-unique-123") != 1 {
		t.Fatalf("duplicate retained evidence: %+v", retained)
	}
	prism := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "prism-test"}, nil)
	registerTools(prism, mcpFakeRunner{}, Config{DownstreamMCP: client})
	ct, st := mcpsdk.NewInMemoryTransports()
	ss, err := prism.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	host := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "host"}, nil)
	session, err := host.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "call_mcp_tool", Arguments: map[string]any{"server": "linear", "tool": "get_issue"}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(raw), "ticket-unique-123") != 1 {
		t.Fatalf("duplicated wire payload: %s", raw)
	}
	legacy, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "call_mcp_tool", Arguments: map[string]any{"server": "linear", "tool": "get_issue", "include_text": true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(legacy.Content) != 1 || legacy.StructuredContent == nil {
		t.Fatal("legacy text compatibility lost")
	}
	// Reconstruct the v0.1.36 bridge envelope using the same complete fixture.
	ticketJSON, _ := json.Marshal(ticket)
	old := downstreammcp.CallResult{Server: "linear", Tool: "get_issue", Content: string(ticketJSON), StructuredContent: ticket}
	before := textResult(marshalJSON(old))
	before.StructuredContent = old
	beforeRaw, _ := json.Marshal(before)
	t.Logf("wire bytes: before=%d after=%d", len(beforeRaw), len(raw))
	if dir := os.Getenv("PRISM_RESPONSE_SAMPLE_DIR"); dir != "" {
		for name, data := range map[string][]byte{"before.json": beforeRaw, "after.json": raw} {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
}
