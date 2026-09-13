package extensions

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

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
	if _, err := svc.InstallLocalSkills(context.Background(), InstallLocalSkillsRequest{Source: sourceA, Discover: DiscoverSkillsOptions{All: true}}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.InstallLocalSkills(context.Background(), InstallLocalSkillsRequest{Source: sourceB, Discover: DiscoverSkillsOptions{All: true}}); err == nil {
		t.Fatal("expected replace guard error")
	}
}
