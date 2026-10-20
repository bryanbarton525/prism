package result

import "testing"

func TestStructuredFieldsPreserved(t *testing.T) {
	r := ParseAgentOutput(`{"summary":"done","findings":[],"confidence":"high","decision":{"accessible":false,"status":null,"ids":["A"]},"linear_actions":{"proposed":[],"executed":[],"blocked":[]},"next_action":"none"}`, 400)
	if r.Decision["accessible"] != false || r.Decision["status"] != nil || r.LinearActions == nil || r.NextAction != "none" {
		t.Fatalf("lost fields: %+v", r)
	}
	contract, err := ResolveDecisionSchema(map[string]any{"type": "object", "required": []any{"accessible"}, "properties": map[string]any{"accessible": map[string]any{"type": "boolean"}}})
	if err != nil || contract.Validate(r.Decision) != nil {
		t.Fatalf("valid contract rejected: %v", err)
	}
	if contract.Validate(map[string]any{"accessible": "false"}) == nil {
		t.Fatal("wrong type accepted")
	}
	if _, err := ResolveDecisionSchema(map[string]any{"type": "object", "$ref": "https://example.com/schema"}); err == nil {
		t.Fatal("external ref accepted")
	}
}
