package extensions

import "testing"

func TestMCPAccessAllowedServers(t *testing.T) {
	state := MCPAccessState{
		DefaultServers: []string{"linear"},
		Agents: map[string]MCPAccessRule{
			"a": {Mode: MCPAccessModeCustom, Servers: []string{"docs"}},
			"b": {Mode: MCPAccessModeNone},
		},
	}
	all := []string{"linear", "docs", "github"}
	got := state.AllowedServers("a", all, true)
	if len(got) != 1 || got[0] != "docs" {
		t.Fatalf("custom = %#v", got)
	}
	got = state.AllowedServers("b", all, true)
	if len(got) != 0 {
		t.Fatalf("none = %#v", got)
	}
	got = state.AllowedServers("unknown", all, true)
	if len(got) != 1 || got[0] != "linear" {
		t.Fatalf("default = %#v", got)
	}
	got = state.AllowedServers("unknown", all, false)
	if len(got) != 3 {
		t.Fatalf("legacy expected all, got %#v", got)
	}
}
