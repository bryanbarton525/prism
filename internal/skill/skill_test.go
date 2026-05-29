package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validSkillMD = `---
name: gh-pr-triage
description: Triage pull requests with gh by collecting status, changed files, review signals, and merge blockers.
compatibility: Requires GitHub CLI access.
---

# GH PR triage

1. Gather PR metadata.
2. Identify blockers.
3. Return findings.
`

func makeSkillDir(t *testing.T, name, content string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing SKILL.md: %v", err)
	}
	return root
}

func TestParseFile_Valid(t *testing.T) {
	root := makeSkillDir(t, "gh-pr-triage", validSkillMD)
	sk, err := ParseFile(filepath.Join(root, "gh-pr-triage", "SKILL.md"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sk.Name != "gh-pr-triage" {
		t.Errorf("Name: want gh-pr-triage, got %s", sk.Name)
	}
	if sk.Compatibility != "Requires GitHub CLI access." {
		t.Errorf("Compatibility: got %s", sk.Compatibility)
	}
	if !strings.Contains(sk.Body, "Gather PR metadata") {
		t.Errorf("Body missing expected content: %s", sk.Body)
	}
	if sk.Dir == "" {
		t.Error("Dir should be populated")
	}
}

func TestParseFile_NameMismatch(t *testing.T) {
	root := makeSkillDir(t, "wrong-dir-name", validSkillMD)
	_, err := ParseFile(filepath.Join(root, "wrong-dir-name", "SKILL.md"))
	if err == nil {
		t.Fatal("expected error when skill name != directory name")
	}
	if !strings.Contains(err.Error(), "does not match directory name") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseFile_MissingName(t *testing.T) {
	content := "---\ndescription: Some description.\n---\nbody"
	root := makeSkillDir(t, "x", content)
	_, err := ParseFile(filepath.Join(root, "x", "SKILL.md"))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestParseFile_DescriptionTooLong(t *testing.T) {
	long := strings.Repeat("x", 1025)
	content := "---\nname: gh-pr-triage\ndescription: " + long + "\n---\nbody"
	root := makeSkillDir(t, "gh-pr-triage", content)
	_, err := ParseFile(filepath.Join(root, "gh-pr-triage", "SKILL.md"))
	if err == nil {
		t.Fatal("expected error for description > 1024 chars")
	}
	if !strings.Contains(err.Error(), "1024") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadMany_Success(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"gh-pr-triage": validSkillMD,
		"kubectl-triage": `---
name: kubectl-triage
description: Triage Kubernetes workloads with kubectl.
---
body`,
	} {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	skills, err := LoadMany(root, []string{"gh-pr-triage", "kubectl-triage"})
	if err != nil {
		t.Fatalf("LoadMany: %v", err)
	}
	if len(skills) != 2 {
		t.Errorf("want 2 skills, got %d", len(skills))
	}
}

func TestLoadMany_MissingSkill(t *testing.T) {
	root := t.TempDir()
	_, err := LoadMany(root, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for missing skill")
	}
}

func TestFullText_ContainsKeyParts(t *testing.T) {
	root := makeSkillDir(t, "gh-pr-triage", validSkillMD)
	sk, _ := ParseFile(filepath.Join(root, "gh-pr-triage", "SKILL.md"))

	full := sk.FullText()
	if !strings.Contains(full, "## Skill: gh-pr-triage") {
		t.Error("FullText should contain heading")
	}
	if !strings.Contains(full, "Triage pull requests") {
		t.Error("FullText should contain description")
	}
	if !strings.Contains(full, "Requires GitHub CLI access") {
		t.Error("FullText should contain compatibility")
	}
	if !strings.Contains(full, "Gather PR metadata") {
		t.Error("FullText should contain skill body")
	}
}

func TestMetadataSummary(t *testing.T) {
	root := makeSkillDir(t, "gh-pr-triage", validSkillMD)
	sk, _ := ParseFile(filepath.Join(root, "gh-pr-triage", "SKILL.md"))

	summary := sk.MetadataSummary()
	if !strings.Contains(summary, "**gh-pr-triage**") {
		t.Errorf("MetadataSummary should contain bold name, got: %s", summary)
	}
	if !strings.Contains(summary, "Triage pull requests") {
		t.Errorf("MetadataSummary should contain description excerpt, got: %s", summary)
	}
}

func TestLoadMany_RealSkills(t *testing.T) {
	// Integration test against the real skills/ directory.
	skills, err := LoadMany("../../skills", []string{"gh-pr-triage", "kubectl-triage"})
	if err != nil {
		t.Fatalf("loading real skills: %v", err)
	}
	if len(skills) != 2 {
		t.Errorf("want 2 skills, got %d", len(skills))
	}
}
