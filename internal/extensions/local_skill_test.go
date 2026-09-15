package extensions

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
)

func TestInstallResolvedSkillsMaterializesSourceFilesystem(t *testing.T) {
	state := t.TempDir()
	source := fstest.MapFS{
		"SKILL.md": &fstest.MapFile{Data: []byte("---\nname: remote-skill\ndescription: Remote skill\n---\n# Remote skill")},
		"guide.md": &fstest.MapFile{Data: []byte("guidance")},
	}
	entries, err := NewLocalSkillService(state).InstallResolvedSkills(context.Background(), source, "remote-skill", "github://owner/repo", DiscoverSkillsOptions{}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Identity != "remote-skill" || entries[0].Source != "github://owner/repo" {
		t.Fatalf("entries = %#v", entries)
	}
	if _, err := os.Stat(filepath.Join(entries[0].ObjectPath, "guide.md")); err != nil {
		t.Fatalf("resolved skill was not materialized: %v", err)
	}
	before, err := os.ReadFile(NewStore(state).ManifestPath())
	if err != nil {
		t.Fatal(err)
	}
	again, err := NewLocalSkillService(state).InstallResolvedSkills(context.Background(), source, "remote-skill", "github://owner/repo", DiscoverSkillsOptions{}, false, false)
	if err != nil || len(again) != 1 || again[0].Digest != entries[0].Digest {
		t.Fatalf("identical resolved reinstall: entries=%#v err=%v", again, err)
	}
	after, err := os.ReadFile(NewStore(state).ManifestPath())
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("identical resolved reinstall rewrote manifest: err=%v", err)
	}
}

func TestManagedSkillRecoveryDoesNotNeedRunnableCatalogOrDeleteOldObject(t *testing.T) {
	state := t.TempDir()
	svc := NewLocalSkillService(state)
	source := fstest.MapFS{
		"SKILL.md": &fstest.MapFile{Data: []byte("---\nname: recovery-skill\ndescription: Recovery fixture\n---\n# Original")},
	}
	entries, err := svc.InstallResolvedSkills(context.Background(), source, "recovery-skill", "fixture", DiscoverSkillsOptions{}, false, false)
	if err != nil || len(entries) != 1 {
		t.Fatalf("install: entries=%#v err=%v", entries, err)
	}
	oldObject := filepath.Join(entries[0].ObjectPath, "SKILL.md")
	oldContent, err := os.ReadFile(oldObject)
	if err != nil {
		t.Fatal(err)
	}
	if listed, err := svc.ListManagedSkills(context.Background()); err != nil || len(listed) != 1 {
		t.Fatalf("list without catalog: entries=%#v err=%v", listed, err)
	}
	if ok, err := svc.RenameManagedSkill(context.Background(), "recovery-skill", "restored-skill", false); err != nil || !ok {
		t.Fatalf("rename without catalog: ok=%v err=%v", ok, err)
	}
	if ok, err := svc.RemoveManagedSkill(context.Background(), "restored-skill", false); err != nil || !ok {
		t.Fatalf("remove without catalog: ok=%v err=%v", ok, err)
	}
	if data, err := os.ReadFile(oldObject); err != nil || string(data) != string(oldContent) {
		t.Fatalf("old snapshot object lost: data=%q err=%v", data, err)
	}
}

func TestDiscoverSkillsFSFindsRepositorySkillsDirectory(t *testing.T) {
	source := fstest.MapFS{
		"skills/demo/SKILL.md": &fstest.MapFile{Data: []byte("---\nname: demo\ndescription: Demo\n---\n# Demo")},
	}
	skills, err := DiscoverSkillsFS(source, "ignored", DiscoverSkillsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 || skills[0].Name != "demo" || skills[0].Path != "skills/demo" {
		t.Fatalf("skills = %#v", skills)
	}
}

func TestDiscoverLocalSkills(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a", "b"} {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: d\n---"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := DiscoverLocalSkills(root, DiscoverSkillsOptions{All: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "a" || got[1].Name != "b" {
		t.Fatalf("got %#v", got)
	}
}

func TestInstallLocalSkillsAndRemove(t *testing.T) {
	state := t.TempDir()
	source := t.TempDir()
	skillPath := filepath.Join(source, "demo")
	if err := os.MkdirAll(filepath.Join(skillPath, "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("---\nname: demo\ndescription: d\n---"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillPath, "references", "REFERENCE.md"), []byte("# ref"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := NewLocalSkillService(state)
	entries, err := svc.InstallLocalSkills(context.Background(), InstallLocalSkillsRequest{
		Source: source, Discover: DiscoverSkillsOptions{All: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Identity != "demo" || entries[0].ObjectPath == "" {
		t.Fatalf("entries %#v", entries)
	}
	managed, err := svc.ListManagedSkills(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(managed) != 1 {
		t.Fatalf("managed %#v", managed)
	}
	removed, err := svc.RemoveManagedSkill(context.Background(), "demo", false)
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Fatal("expected removal")
	}
	managed, err = svc.ListManagedSkills(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(managed) != 0 {
		t.Fatalf("managed %#v", managed)
	}
}

func TestInstallLocalSkillsReplaceGuard(t *testing.T) {
	state := t.TempDir()
	sourceA := t.TempDir()
	sourceB := t.TempDir()
	for i, src := range []string{sourceA, sourceB} {
		dir := filepath.Join(src, "demo")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: demo\ndescription: d\n---\n" + string(rune('a'+i))
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	svc := NewLocalSkillService(state)
	first, err := svc.InstallLocalSkills(context.Background(), InstallLocalSkillsRequest{Source: sourceA, Discover: DiscoverSkillsOptions{All: true}})
	if err != nil {
		t.Fatal(err)
	}
	beforeManifest, err := os.ReadFile(NewStore(state).ManifestPath())
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.InstallLocalSkills(context.Background(), InstallLocalSkillsRequest{Source: sourceA, Discover: DiscoverSkillsOptions{All: true}})
	if err != nil || len(second) != 1 || second[0].Digest != first[0].Digest {
		t.Fatalf("identical reinstall should be a no-op: first=%#v second=%#v err=%v", first, second, err)
	}
	afterManifest, err := os.ReadFile(NewStore(state).ManifestPath())
	if err != nil || !bytes.Equal(beforeManifest, afterManifest) {
		t.Fatalf("identical reinstall rewrote manifest: err=%v", err)
	}
	if _, err := svc.InstallLocalSkills(context.Background(), InstallLocalSkillsRequest{Source: sourceB, Discover: DiscoverSkillsOptions{All: true}}); err == nil {
		t.Fatal("expected replace guard error")
	}
}

func TestConcurrentManagedSkillMutationsDoNotBypassConflictOrLoseEntries(t *testing.T) {
	makeSource := func(name, body string) string {
		source := filepath.Join(t.TempDir(), name)
		if err := os.MkdirAll(source, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: fixture\n---\n"+body), 0o600); err != nil {
			t.Fatal(err)
		}
		return source
	}
	for trial := 0; trial < 12; trial++ {
		state := t.TempDir()
		service := NewLocalSkillService(state)
		sourceA, sourceB := makeSource("shared", "A"), makeSource("shared", "B")
		start := make(chan struct{})
		var wg sync.WaitGroup
		outcomes := make(chan error, 2)
		for _, source := range []string{sourceA, sourceB} {
			wg.Add(1)
			go func(source string) {
				defer wg.Done()
				<-start
				_, err := service.InstallLocalSkills(context.Background(), InstallLocalSkillsRequest{Source: source})
				outcomes <- err
			}(source)
		}
		close(start)
		wg.Wait()
		close(outcomes)
		successes, conflicts := 0, 0
		for err := range outcomes {
			if err == nil {
				successes++
			} else if strings.Contains(err.Error(), "--replace") {
				conflicts++
			} else {
				t.Fatalf("trial %d unexpected concurrent install error: %v", trial, err)
			}
		}
		manifest, _, err := NewStore(state).RecoverAndLoadManifest(context.Background())
		if err != nil || successes != 1 || conflicts != 1 || len(manifest.Entries) != 1 {
			t.Fatalf("trial %d install state: success=%d conflicts=%d manifest=%#v err=%v", trial, successes, conflicts, manifest, err)
		}
	}

	for trial := 0; trial < 12; trial++ {
		state := t.TempDir()
		service := NewLocalSkillService(state)
		if _, err := service.InstallLocalSkills(context.Background(), InstallLocalSkillsRequest{Source: makeSource("old", "old")}); err != nil {
			t.Fatal(err)
		}
		newSource := makeSource("new", "new")
		start := make(chan struct{})
		var wg sync.WaitGroup
		outcomes := make(chan error, 2)
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, err := service.RemoveManagedSkill(context.Background(), "old", false)
			outcomes <- err
		}()
		go func() {
			defer wg.Done()
			<-start
			_, err := service.InstallLocalSkills(context.Background(), InstallLocalSkillsRequest{Source: newSource})
			outcomes <- err
		}()
		close(start)
		wg.Wait()
		close(outcomes)
		for err := range outcomes {
			if err != nil {
				t.Fatalf("trial %d remove/install error: %v", trial, err)
			}
		}
		manifest, _, err := NewStore(state).RecoverAndLoadManifest(context.Background())
		if err != nil || len(manifest.Entries) != 1 || manifest.Entries[0].Identity != "new" {
			t.Fatalf("trial %d lost concurrent skill install: manifest=%#v err=%v", trial, manifest, err)
		}
	}
}

func TestIdenticalSkillReinstallRejectsCorruptManagedObject(t *testing.T) {
	state := t.TempDir()
	source := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("---\nname: demo\ndescription: fixture\n---\noriginal"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewLocalSkillService(state)
	entries, err := service.InstallLocalSkills(context.Background(), InstallLocalSkillsRequest{Source: source})
	if err != nil || len(entries) != 1 {
		t.Fatalf("install: entries=%#v err=%v", entries, err)
	}
	if err := os.WriteFile(filepath.Join(entries[0].ObjectPath, "SKILL.md"), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.InstallLocalSkills(context.Background(), InstallLocalSkillsRequest{Source: source}); err == nil || !strings.Contains(err.Error(), "integrity") {
		t.Fatalf("identical reinstall trusted corrupt object: %v", err)
	}
}

func TestDirectoryPublishersRejectCorruptExistingObjectWithoutDeleting(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("---\nname: demo\ndescription: fixture\n---\noriginal"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		publish func(*Store) (string, string, error)
	}{
		{"local", func(store *Store) (string, string, error) { return store.putDirectoryObject(source) }},
		{"filesystem", func(store *Store) (string, string, error) { return store.putFSDirectory(os.DirFS(source), ".") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := NewStore(t.TempDir())
			_, objectPath, err := tc.publish(store)
			if err != nil {
				t.Fatal(err)
			}
			objectFile := filepath.Join(objectPath, "SKILL.md")
			if err := os.WriteFile(objectFile, []byte("tampered"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := tc.publish(store); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
				t.Fatalf("corrupt digest-named object was trusted or rebuilt: %v", err)
			}
			if data, err := os.ReadFile(objectFile); err != nil || string(data) != "tampered" {
				t.Fatalf("corrupt object was deleted or overwritten: data=%q err=%v", data, err)
			}
		})
	}
}

func TestRemoveManagedSkillRejectsAgentBinding(t *testing.T) {
	state := t.TempDir()
	source := t.TempDir()
	skillPath := filepath.Join(source, "demo")
	if err := os.MkdirAll(skillPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("---\nname: demo\ndescription: d\n---"), 0o644); err != nil {
		t.Fatal(err)
	}
	skills := NewLocalSkillService(state)
	if _, err := skills.InstallLocalSkills(context.Background(), InstallLocalSkillsRequest{Source: source, Discover: DiscoverSkillsOptions{All: true}}); err != nil {
		t.Fatal(err)
	}
	agentPath := filepath.Join(t.TempDir(), "agent.md")
	agentSource := []byte("---\nid: agent\nname: Agent\ndescription: d\nmodel: m\ncontext_budget: 1\nallowed_skills: [demo]\n---\nbody")
	if err := os.WriteFile(agentPath, agentSource, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLocalAgentService(state).InstallLocalAgent(context.Background(), InstallLocalAgentRequest{Source: agentPath}); err != nil {
		t.Fatal(err)
	}
	if _, err := skills.RemoveManagedSkill(context.Background(), "demo", false); err == nil {
		t.Fatal("expected skill binding rejection")
	}
}
