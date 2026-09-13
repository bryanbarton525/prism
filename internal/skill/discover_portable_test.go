package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePortableStructure_AllowsMinimalSkill(t *testing.T) {
	root := t.TempDir()
	name := "minimal-skill"
	if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `---
name: minimal-skill
description: Minimal standard skill.
---
body`
	if err := os.WriteFile(filepath.Join(root, name, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePortableStructure(os.DirFS(root), name); err != nil {
		t.Fatalf("portable validation failed: %v", err)
	}
	if _, err := LoadDir(os.DirFS(root), name); err != nil {
		t.Fatalf("load failed: %v", err)
	}
}

func TestValidateAuthoringStructure_RequiresBundledPaths(t *testing.T) {
	root := t.TempDir()
	name := "minimal-skill"
	if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name, "SKILL.md"), []byte(`---
name: minimal-skill
description: Minimal.
---
body`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateAuthoringStructure(os.DirFS(root), name); err == nil {
		t.Fatal("expected strict authoring failure for missing references/scripts/evals")
	}
}

func TestExecutionLimitationsReportWarnings(t *testing.T) {
	root := t.TempDir()
	name := "script-skill"
	if err := os.MkdirAll(filepath.Join(root, name, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name, "SKILL.md"), []byte(`---
name: script-skill
description: Script skill.
---
body`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name, "scripts", "collect.sh"), []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	warnings := ExecutionLimitations(os.DirFS(root), name)
	if len(warnings) == 0 {
		t.Fatal("expected execution limitation warnings")
	}
}
