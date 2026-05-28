package skill_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bryanbarton525/prism/internal/skill"
)

// makeSkillDir creates a temporary skill directory with the given contents.
// Pass empty string to omit a file/dir. Pass a non-empty string for SKILL.md
// content. Pass true/false for references/scripts presence.
func makeSkillDir(t *testing.T, name string, skillMD string, hasReferences, hasScripts bool) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if skillMD != "" {
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
			t.Fatalf("write SKILL.md: %v", err)
		}
	}
	if hasReferences {
		if err := os.MkdirAll(filepath.Join(dir, "references"), 0o755); err != nil {
			t.Fatalf("mkdir references: %v", err)
		}
	}
	if hasScripts {
		if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
			t.Fatalf("mkdir scripts: %v", err)
		}
	}
	return dir
}

const validSkillMD = `---
name: my-skill
description: Does something useful when debugging is needed.
---

# My skill body
`

func TestLoad_Valid(t *testing.T) {
	dir := makeSkillDir(t, "my-skill", validSkillMD, true, true)

	s, err := skill.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.DirName != "my-skill" {
		t.Errorf("DirName = %q, want %q", s.DirName, "my-skill")
	}
	if s.Frontmatter.Name != "my-skill" {
		t.Errorf("Name = %q, want %q", s.Frontmatter.Name, "my-skill")
	}
	if s.Frontmatter.Description == "" {
		t.Error("Description should not be empty")
	}
	if !strings.Contains(s.Body, "My skill body") {
		t.Errorf("Body does not contain expected content, got: %q", s.Body)
	}
}

func TestLoad_MissingReferences(t *testing.T) {
	dir := makeSkillDir(t, "my-skill", validSkillMD, false, true)

	_, err := skill.Load(dir)
	if err == nil {
		t.Fatal("expected validation error for missing references/, got nil")
	}
	if !strings.Contains(err.Error(), "references") {
		t.Errorf("error should mention 'references', got: %v", err)
	}
}

func TestLoad_MissingScripts(t *testing.T) {
	dir := makeSkillDir(t, "my-skill", validSkillMD, true, false)

	_, err := skill.Load(dir)
	if err == nil {
		t.Fatal("expected validation error for missing scripts/, got nil")
	}
	if !strings.Contains(err.Error(), "scripts") {
		t.Errorf("error should mention 'scripts', got: %v", err)
	}
}

func TestLoad_MissingSkillMD(t *testing.T) {
	dir := makeSkillDir(t, "my-skill", "", true, true)

	_, err := skill.Load(dir)
	if err == nil {
		t.Fatal("expected validation error for missing SKILL.md, got nil")
	}
	if !strings.Contains(err.Error(), "SKILL.md") {
		t.Errorf("error should mention 'SKILL.md', got: %v", err)
	}
}

func TestLoad_NameMismatch(t *testing.T) {
	md := `---
name: wrong-name
description: A skill description that is clearly stated here.
---

# Body
`
	dir := makeSkillDir(t, "my-skill", md, true, true)

	_, err := skill.Load(dir)
	if err == nil {
		t.Fatal("expected validation error for name mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "wrong-name") {
		t.Errorf("error should contain mismatched name, got: %v", err)
	}
}

func TestLoad_MissingName(t *testing.T) {
	md := `---
description: A skill description.
---

# Body
`
	dir := makeSkillDir(t, "my-skill", md, true, true)

	_, err := skill.Load(dir)
	if err == nil {
		t.Fatal("expected validation error for missing name, got nil")
	}
	if !strings.Contains(err.Error(), "'name'") {
		t.Errorf("error should mention 'name' field, got: %v", err)
	}
}

func TestLoad_MissingDescription(t *testing.T) {
	md := `---
name: my-skill
---

# Body
`
	dir := makeSkillDir(t, "my-skill", md, true, true)

	_, err := skill.Load(dir)
	if err == nil {
		t.Fatal("expected validation error for missing description, got nil")
	}
	if !strings.Contains(err.Error(), "'description'") {
		t.Errorf("error should mention 'description' field, got: %v", err)
	}
}

func TestLoad_DescriptionTooLong(t *testing.T) {
	longDesc := strings.Repeat("a", 1025)
	md := "---\nname: my-skill\ndescription: " + longDesc + "\n---\n\n# Body\n"
	dir := makeSkillDir(t, "my-skill", md, true, true)

	_, err := skill.Load(dir)
	if err == nil {
		t.Fatal("expected validation error for description too long, got nil")
	}
	if !strings.Contains(err.Error(), "1024") {
		t.Errorf("error should mention 1024-character limit, got: %v", err)
	}
}

func TestLoad_NoFrontmatterDelimiter(t *testing.T) {
	md := "# My skill body\nNo frontmatter here.\n"
	dir := makeSkillDir(t, "my-skill", md, true, true)

	_, err := skill.Load(dir)
	if err == nil {
		t.Fatal("expected error for missing frontmatter delimiter, got nil")
	}
}

func TestLoad_UnclosedFrontmatter(t *testing.T) {
	md := "---\nname: my-skill\ndescription: A description.\n"
	dir := makeSkillDir(t, "my-skill", md, true, true)

	_, err := skill.Load(dir)
	if err == nil {
		t.Fatal("expected error for unclosed frontmatter, got nil")
	}
}

func TestLoadAll_ValidSkillsRoot(t *testing.T) {
	root := t.TempDir()

	for _, name := range []string{"skill-a", "skill-b"} {
		dir := filepath.Join(root, name)
		_ = os.MkdirAll(filepath.Join(dir, "references"), 0o755)
		_ = os.MkdirAll(filepath.Join(dir, "scripts"), 0o755)
		md := "---\nname: " + name + "\ndescription: A valid description for " + name + ".\n---\n\n# Body\n"
		_ = os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(md), 0o644)
	}
	// Add a non-directory file that should be ignored.
	_ = os.WriteFile(filepath.Join(root, "README.md"), []byte("# readme"), 0o644)

	skills, err := skill.LoadAll(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
	}
}

func TestLoadAll_PartialErrors(t *testing.T) {
	root := t.TempDir()

	// Valid skill
	good := filepath.Join(root, "skill-good")
	_ = os.MkdirAll(filepath.Join(good, "references"), 0o755)
	_ = os.MkdirAll(filepath.Join(good, "scripts"), 0o755)
	_ = os.WriteFile(filepath.Join(good, "SKILL.md"), []byte("---\nname: skill-good\ndescription: A good description.\n---\n\n# Body\n"), 0o644)

	// Invalid skill: name mismatch
	bad := filepath.Join(root, "skill-bad")
	_ = os.MkdirAll(filepath.Join(bad, "references"), 0o755)
	_ = os.MkdirAll(filepath.Join(bad, "scripts"), 0o755)
	_ = os.WriteFile(filepath.Join(bad, "SKILL.md"), []byte("---\nname: wrong-name\ndescription: A bad description.\n---\n\n# Body\n"), 0o644)

	skills, err := skill.LoadAll(root)
	if err == nil {
		t.Fatal("expected error for invalid skill, got nil")
	}
	if len(skills) != 1 {
		t.Errorf("expected 1 valid skill, got %d", len(skills))
	}
	if !strings.Contains(err.Error(), "wrong-name") {
		t.Errorf("error should mention the mismatched name, got: %v", err)
	}
}

func TestLoadAll_RealSkillsDirectory(t *testing.T) {
	// Validate all skills in the actual repository skills/ directory.
	// This test uses a relative path and will pass when run from the module root.
	skillsRoot := "../../skills"
	if _, err := os.Stat(skillsRoot); os.IsNotExist(err) {
		t.Skipf("skills root %q not found, skipping integration test", skillsRoot)
	}

	skills, err := skill.LoadAll(skillsRoot)
	if err != nil {
		t.Errorf("real skills directory contains validation errors: %v", err)
	}
	if len(skills) == 0 {
		t.Error("expected at least one skill to be loaded from real skills directory")
	}
	for _, s := range skills {
		t.Logf("loaded skill: %s (%q)", s.DirName, s.Frontmatter.Name)
	}
}
