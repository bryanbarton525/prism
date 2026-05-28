package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFile_Valid(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "gh-pr-triage")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillMD := filepath.Join(skillDir, "SKILL.md")
	content := `---
name: gh-pr-triage
description: Triage pull requests with gh by collecting status, changed files, and merge blockers.
compatibility: Requires GitHub CLI access.
---

# GH PR triage

1. Gather PR metadata.
2. Identify blockers.
`
	if err := os.WriteFile(skillMD, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	sk, err := ParseFile(skillMD)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sk.Name != "gh-pr-triage" {
		t.Errorf("Name: want gh-pr-triage, got %q", sk.Name)
	}
	if sk.Body == "" {
		t.Error("Body should not be empty")
	}
	if sk.Dir != skillDir {
		t.Errorf("Dir: want %q, got %q", skillDir, sk.Dir)
	}
}

func TestParse_MissingDescription(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "bad-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillMD := filepath.Join(skillDir, "SKILL.md")
	content := `---
name: bad-skill
---
body
`
	if err := os.WriteFile(skillMD, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ParseFile(skillMD)
	if err == nil {
		t.Fatal("expected error for missing description field")
	}
}

func TestParse_NameMismatch(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "correct-name")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillMD := filepath.Join(skillDir, "SKILL.md")
	content := `---
name: wrong-name
description: A skill.
---
body
`
	if err := os.WriteFile(skillMD, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ParseFile(skillMD)
	if err == nil {
		t.Fatal("expected error when skill name does not match directory name")
	}
}

func TestValidateSkillsDir_MissingSkill(t *testing.T) {
	dir := t.TempDir()
	_, err := ValidateSkillsDir(dir, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for missing skill directory")
	}
}

func TestFullText(t *testing.T) {
	sk := &Skill{
		Name:        "test-skill",
		Description: "A test skill.",
		Body:        "Step 1.\nStep 2.",
	}
	text := sk.FullText()
	if text == "" {
		t.Error("FullText should not be empty")
	}
	for _, want := range []string{"test-skill", "A test skill.", "Step 1."} {
		if !contains(text, want) {
			t.Errorf("FullText missing %q", want)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && (s[:len(sub)] == sub || contains(s[1:], sub)))
}
