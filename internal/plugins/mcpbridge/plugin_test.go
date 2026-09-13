package mcpbridge

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bryanbarton525/prism/internal/downstreammcp"
	"github.com/bryanbarton525/prism/internal/plugins"
)

func TestMarshalBoundedAlwaysValidJSON(t *testing.T) {
	// Build an inventory that exceeds the limit by a wide margin.
	doc := inventoryDoc{Configured: true, Notes: []string{"n"}}
	for i := 0; i < 8; i++ {
		s := serverInventory{Name: strings.Repeat("s", 20), Transport: "command"}
		for j := 0; j < 20; j++ {
			s.Tools = append(s.Tools, downstreammcp.ToolSummary{
				Name:        strings.Repeat("t", 40),
				Description: strings.Repeat("d", 400),
			})
		}
		doc.Servers = append(doc.Servers, s)
	}
	content, err := marshalBounded(doc, 4000)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) > 4000 {
		t.Fatalf("content = %d bytes, want <= 4000", len(content))
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		t.Fatalf("bounded inventory is not valid JSON: %v", err)
	}
	notes, _ := parsed["notes"].([]any)
	foundReduction := false
	for _, n := range notes {
		if s, ok := n.(string); ok && strings.Contains(s, "reduced to fit") {
			foundReduction = true
		}
	}
	if !foundReduction {
		t.Fatal("reduction note missing from bounded inventory")
	}
}

func TestMarshalBoundedMinimalFallback(t *testing.T) {
	// Server metadata alone exceeds the limit even with zero tools listed;
	// the fallback must still return valid JSON under the limit.
	doc := inventoryDoc{Configured: true}
	for i := 0; i < 30; i++ {
		doc.Servers = append(doc.Servers, serverInventory{
			Name:      strings.Repeat("n", 60),
			Transport: "command",
			Command:   strings.Repeat("c", 120),
			URL:       strings.Repeat("u", 120),
		})
	}
	content, err := marshalBounded(doc, 800)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) > 800 {
		t.Fatalf("content = %d bytes, want <= 800", len(content))
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		t.Fatalf("fallback inventory is not valid JSON: %v", err)
	}
	if parsed["configured"] != true {
		t.Fatalf("configured flag lost: %v", parsed)
	}
	notes, _ := parsed["notes"].([]any)
	if len(notes) == 0 || !strings.Contains(notes[0].(string), "exceeded the output limit") {
		t.Fatalf("fallback note missing: %v", notes)
	}
}

func TestInventoryWithoutClientIsBoundedEvidence(t *testing.T) {
	res, err := New(nil).Call(context.Background(), plugins.ToolCall{Tool: ToolInventory})
	if err != nil {
		t.Fatal(err)
	}
	if res.Label != "runtime-plugin:mcp" {
		t.Fatalf("label = %q", res.Label)
	}
	if res.EvidencePack == nil || res.EvidencePack.Kind != "mcp.inventory" {
		t.Fatalf("evidence = %#v", res.EvidencePack)
	}
	if !strings.Contains(res.Content, `"configured":false`) {
		t.Fatalf("content = %s", res.Content)
	}
}
