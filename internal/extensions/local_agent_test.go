package extensions

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	verifyRunnableIdentity := func(identity string) {
		entries, err := svc.ListManagedAgents(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.Identity != identity {
				continue
			}
			data, err := os.ReadFile(entry.ObjectPath)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := agent.Parse(data, identity+".md"); err != nil {
				t.Fatalf("%s managed object is not runnable: %v", identity, err)
			}
			return
		}
		t.Fatalf("managed %s entry missing after mutation", identity)
	}
	verifyRunnableIdentity("copied-agent")
	ok, err = svc.RenameManagedAgent(context.Background(), "copied-agent", "renamed-agent", false)
	if err != nil || !ok {
		t.Fatalf("rename: %v %v", ok, err)
	}
	verifyRunnableIdentity("renamed-agent")
	ok, err = svc.RemoveManagedAgent(context.Background(), "renamed-agent", false)
	if err != nil || !ok {
		t.Fatalf("remove: %v %v", ok, err)
	}
}

func TestIdenticalManagedAgentReinstallIsNoopButChecksObjectIntegrity(t *testing.T) {
	state := t.TempDir()
	source := filepath.Join(t.TempDir(), "agent.md")
	content := []byte("---\nid: agent\nname: Agent\ndescription: fixture\nmodel: local\ncontext_budget: 1000\nlatency_budget_ms: 1000\nallowed_skills: []\n---\nbody")
	if err := os.WriteFile(source, content, 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewLocalAgentService(state)
	first, err := service.InstallLocalAgent(context.Background(), InstallLocalAgentRequest{Source: source})
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(NewStore(state).ManifestPath())
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.InstallLocalAgent(context.Background(), InstallLocalAgentRequest{Source: source})
	if err != nil || second.Digest != first.Digest {
		t.Fatalf("identical agent reinstall: first=%#v second=%#v err=%v", first, second, err)
	}
	after, err := os.ReadFile(NewStore(state).ManifestPath())
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("identical agent reinstall rewrote manifest: err=%v", err)
	}
	if err := os.WriteFile(first.ObjectPath, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.InstallLocalAgent(context.Background(), InstallLocalAgentRequest{Source: source}); err == nil || !strings.Contains(err.Error(), "integrity") {
		t.Fatalf("identical agent reinstall trusted corrupt object: %v", err)
	}
}

func TestConcurrentManagedAgentMutationsPreserveConflictAndNewEntries(t *testing.T) {
	makeSource := func(id, body string) string {
		source := filepath.Join(t.TempDir(), id+".md")
		content := "---\nid: " + id + "\nname: Agent\ndescription: fixture\nmodel: local\ncontext_budget: 1000\nlatency_budget_ms: 1000\nallowed_skills: []\n---\n" + body
		if err := os.WriteFile(source, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return source
	}
	for trial := 0; trial < 12; trial++ {
		service := NewLocalAgentService(t.TempDir())
		start := make(chan struct{})
		results := make(chan error, 2)
		var wg sync.WaitGroup
		for _, source := range []string{makeSource("shared", "A"), makeSource("shared", "B")} {
			wg.Add(1)
			go func(source string) {
				defer wg.Done()
				<-start
				_, err := service.InstallLocalAgent(context.Background(), InstallLocalAgentRequest{Source: source})
				results <- err
			}(source)
		}
		close(start)
		wg.Wait()
		close(results)
		successes, conflicts := 0, 0
		for err := range results {
			if err == nil {
				successes++
			} else if strings.Contains(err.Error(), "--replace") {
				conflicts++
			} else {
				t.Fatalf("trial %d unexpected concurrent install error: %v", trial, err)
			}
		}
		entries, err := service.ListManagedAgents(context.Background())
		if err != nil || successes != 1 || conflicts != 1 || len(entries) != 1 {
			t.Fatalf("trial %d install state: successes=%d conflicts=%d entries=%#v err=%v", trial, successes, conflicts, entries, err)
		}
	}
	for trial := 0; trial < 12; trial++ {
		service := NewLocalAgentService(t.TempDir())
		if _, err := service.InstallLocalAgent(context.Background(), InstallLocalAgentRequest{Source: makeSource("old", "old")}); err != nil {
			t.Fatal(err)
		}
		newSource := makeSource("new", "new")
		start := make(chan struct{})
		results := make(chan error, 2)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, err := service.RemoveManagedAgent(context.Background(), "old", false)
			results <- err
		}()
		go func() {
			defer wg.Done()
			<-start
			_, err := service.InstallLocalAgent(context.Background(), InstallLocalAgentRequest{Source: newSource})
			results <- err
		}()
		close(start)
		wg.Wait()
		close(results)
		for err := range results {
			if err != nil {
				t.Fatalf("trial %d concurrent remove/install error: %v", trial, err)
			}
		}
		entries, err := service.ListManagedAgents(context.Background())
		if err != nil || len(entries) != 1 || entries[0].Identity != "new" {
			t.Fatalf("trial %d lost concurrent agent install: entries=%#v err=%v", trial, entries, err)
		}
	}
}
