package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestReviewAgentOverrideUsesSiblingConstitutionFilesystem(t *testing.T) {
	root := t.TempDir()
	agentDir := filepath.Join(root, "agents")
	writeFile(t, filepath.Join(agentDir, "custom.md"), `---
id: custom
name: Custom
description: custom
model: local
context_budget: 100
allowed_skills: [skill]
latency_budget_ms: 30000
constitution_path: constitutions/custom.md
---
`)
	writeFile(t, filepath.Join(root, "constitutions", "custom.md"), "custom constitution")
	runner, err := New(Config{
		BundleFS:       os.DirFS(root),
		AgentDir:       agentDir,
		ConstitutionFS: os.DirFS(root),
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := runner.GetConstitution(context.Background(), "custom")
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != "path" || got.Text != "custom constitution" {
		t.Fatalf("override constitution = %#v", got)
	}
}

func TestSkillResourceToolsEnforceAttachmentAndAggregateBudget(t *testing.T) {
	files := fstest.MapFS{"attached/SKILL.md": {Data: []byte("---\nname: attached\ndescription: attached resources\n---\n")}}
	for i := 0; i < 5; i++ {
		files["attached/resource"+string(rune('a'+i))+".txt"] = &fstest.MapFile{Data: []byte(strings.Repeat("x", 32*1024))}
	}
	runner := &Runner{skillsFS: files}
	resourceBytes := 0
	if _, _, err := runner.dispatchMCPToolCall(context.Background(), "managed", []string{"attached"}, &resourceBytes, "read_skill_resource", map[string]any{"skill_name": "other", "path": "x.txt"}); err == nil || !strings.Contains(err.Error(), "not attached") {
		t.Fatalf("expected unattached-skill rejection, got %v", err)
	}
	for i := 0; i < 4; i++ {
		name := "resource" + string(rune('a'+i)) + ".txt"
		if _, _, err := runner.dispatchMCPToolCall(context.Background(), "managed", []string{"attached"}, &resourceBytes, "read_skill_resource", map[string]any{"skill_name": "attached", "path": name}); err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
	}
	if resourceBytes != 128*1024 {
		t.Fatalf("resource bytes = %d", resourceBytes)
	}
	if _, _, err := runner.dispatchMCPToolCall(context.Background(), "managed", []string{"attached"}, &resourceBytes, "read_skill_resource", map[string]any{"skill_name": "attached", "path": "resourcee.txt"}); err == nil || !strings.Contains(err.Error(), "budget") {
		t.Fatalf("expected aggregate budget rejection, got %v", err)
	}
}

func TestExplicitSkillResourceAttachmentsAreRunScoped(t *testing.T) {
	files := fstest.MapFS{
		"attached/SKILL.md": {Data: []byte("---\nname: attached\ndescription: attached resources\n---\n")},
		"attached/note.txt": {Data: []byte("bounded evidence")},
	}
	evidence, err := collectSkillResourceAttachments(files, []string{"attached"}, []string{"attached:note.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if evidence.byteSize != len("bounded evidence") || len(evidence.artifacts) != 1 || !strings.Contains(evidence.promptBlock, "bounded evidence") {
		t.Fatalf("unexpected evidence: %#v", evidence)
	}
	if _, err := collectSkillResourceAttachments(files, nil, []string{"attached:note.txt"}); err == nil || !strings.Contains(err.Error(), "not attached") {
		t.Fatalf("expected run-scope rejection, got %v", err)
	}
}
