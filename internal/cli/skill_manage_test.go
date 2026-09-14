package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
