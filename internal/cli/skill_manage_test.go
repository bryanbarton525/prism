package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bryanbarton525/prism/internal/extensions"
)

func TestSkillAddAndManagedList(t *testing.T) {
	origState := gf.stateDir
	origJSON := gf.jsonOut
	gf.stateDir = t.TempDir()
	gf.jsonOut = false
	defer func() {
		gf.stateDir = origState
		gf.jsonOut = origJSON
	}()

	source := t.TempDir()
	skillDir := filepath.Join(source, "demo-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo-skill\ndescription: demo\n---"), 0o644); err != nil {
		t.Fatal(err)
	}

	add := newSkillAddCmd()
	add.SetArgs([]string{source, "--all"})
	if err := add.Execute(); err != nil {
		t.Fatal(err)
	}

	list := newSkillManagedListCmd()
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := list.Execute()
	_ = w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(r)
	if !strings.Contains(string(data), "demo-skill") {
		t.Fatalf("output = %q", string(data))
	}
}

func TestManagedSkillResourcesUseEffectiveCatalog(t *testing.T) {
	originalState, originalSkillsDir, originalJSON := gf.stateDir, gf.skillsDir, gf.jsonOut
	gf.stateDir, gf.skillsDir, gf.jsonOut = t.TempDir(), "", false
	t.Cleanup(func() { gf.stateDir, gf.skillsDir, gf.jsonOut = originalState, originalSkillsDir, originalJSON })
	source := filepath.Join(t.TempDir(), "managed-resource-skill")
	if err := os.MkdirAll(filepath.Join(source, "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("---\nname: managed-resource-skill\ndescription: Managed resource fixture\n---\n# Skill"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "references", "guide.md"), []byte("managed resource evidence"), 0o600); err != nil {
		t.Fatal(err)
	}
	add := newSkillAddCmd()
	add.SetArgs([]string{source})
	if _, err := captureStdout(t, add.Execute); err != nil {
		t.Fatal(err)
	}
	list := newSkillResourcesListCmd()
	list.SetArgs([]string{"managed-resource-skill"})
	listed, err := captureStdout(t, list.Execute)
	if err != nil || !strings.Contains(listed, "references/guide.md") {
		t.Fatalf("managed resource list: output=%q err=%v", listed, err)
	}
	read := newSkillResourcesReadCmd()
	read.SetArgs([]string{"managed-resource-skill", "references/guide.md"})
	content, err := captureStdout(t, read.Execute)
	if err != nil || !strings.Contains(content, "managed resource evidence") {
		t.Fatalf("managed resource read: output=%q err=%v", content, err)
	}
	manifest, _, err := extensions.NewStore(gf.stateDir).RecoverAndLoadManifest(context.Background())
	if err != nil || len(manifest.Entries) != 1 {
		t.Fatalf("managed manifest: %#v err=%v", manifest, err)
	}
	if err := os.WriteFile(filepath.Join(manifest.Entries[0].ObjectPath, "SKILL.md"), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	corruptList := newSkillResourcesListCmd()
	corruptList.SetArgs([]string{"managed-resource-skill"})
	if _, err := captureStdout(t, corruptList.Execute); err == nil || !strings.Contains(err.Error(), "digest") {
		t.Fatalf("corrupt managed object was hidden by embedded fallback: %v", err)
	}
	managedList := newSkillManagedListCmd()
	if _, err := captureStdout(t, managedList.Execute); err != nil {
		t.Fatalf("manifest-only recovery command should remain usable: %v", err)
	}
}
