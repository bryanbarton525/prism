package extensions

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

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

func TestConcurrentMCPAccessUpdatesPreserveDistinctAgentRules(t *testing.T) {
	for trial := 0; trial < 8; trial++ {
		stateDir := t.TempDir()
		start := make(chan struct{})
		var wg sync.WaitGroup
		errors := make(chan error, 3)
		for index := 0; index < 3; index++ {
			identity := fmt.Sprintf("agent-%d", index)
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, err := UpdateMCPAccess(context.Background(), stateDir, func(state *MCPAccessState) error {
					if state.Agents == nil {
						state.Agents = map[string]MCPAccessRule{}
					}
					state.Agents[identity] = MCPAccessRule{Mode: MCPAccessModeNone}
					return nil
				})
				errors <- err
			}()
		}
		close(start)
		wg.Wait()
		close(errors)
		for err := range errors {
			if err != nil {
				t.Fatal(err)
			}
		}
		state, configured, err := LoadMCPAccess(stateDir)
		if err != nil || !configured || len(state.Agents) != 3 {
			t.Fatalf("trial %d lost access update: state=%#v configured=%v err=%v", trial, state, configured, err)
		}
	}
}
