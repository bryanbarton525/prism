package extensions

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bryanbarton525/prism/internal/agent"
)

func TestLocalAgentServiceInstallListCopyRenameRemove(t *testing.T) {
	state := t.TempDir()
	source := filepath.Join(t.TempDir(), "agent.md")
	if err := os.WriteFile(source, []byte("---\nid: agent\nname: Agent\ndescription: d\nmodel: m\ncontext_budget: 1000\nallowed_skills: [x]\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := NewLocalAgentService(state)
	entry, err := svc.InstallLocalAgent(context.Background(), InstallLocalAgentRequest{Source: source, As: "custom-agent"})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Identity != "custom-agent" {
		t.Fatalf("entry %#v", entry)
	}
	data, err := os.ReadFile(entry.ObjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := agent.Parse(data, "custom-agent.md"); err != nil {
		t.Fatalf("renamed managed agent is not runnable: %v", err)
	}
	list, err := svc.ListManagedAgents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("list %#v", list)
	}
	ok, err := svc.CopyManagedAgent(context.Background(), "custom-agent", "copied-agent", false)
	if err != nil || !ok {
		t.Fatalf("copy: %v %v", ok, err)
	}
	ok, err = svc.RenameManagedAgent(context.Background(), "copied-agent", "renamed-agent", false)
	if err != nil || !ok {
		t.Fatalf("rename: %v %v", ok, err)
	}
	ok, err = svc.RemoveManagedAgent(context.Background(), "renamed-agent", false)
	if err != nil || !ok {
		t.Fatalf("remove: %v %v", ok, err)
	}
}
