package mcpbridge

import (
	"context"
	"strings"
	"testing"

	"github.com/bryanbarton525/prism/internal/plugins"
)

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
