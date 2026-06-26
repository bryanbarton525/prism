package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallInstructionsCreatesFile(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "AGENTS.md")

	action, err := installInstructions(dest, instructionsBlock(instructionsTarget{}), "")
	if err != nil {
		t.Fatal(err)
	}
	if action != "created" {
		t.Fatalf("action = %q, want created", action)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, instructionsBeginMarker) || !strings.Contains(content, instructionsEndMarker) {
		t.Fatalf("missing sentinel markers:\n%s", content)
	}
	if !strings.Contains(content, "Prism is a local-first specialist agent runner") {
		t.Fatalf("missing body:\n%s", content)
	}
}

func TestInstallInstructionsWritesPreambleForNewFile(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, ".cursor", "rules", "prism.mdc")
	preamble := "---\ndescription: x\nalwaysApply: true\n---\n\n"

	if _, err := installInstructions(dest, instructionsBlock(instructionsTarget{}), preamble); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), preamble) {
		t.Fatalf("preamble not written first:\n%s", string(data))
	}
}

func TestInstallInstructionsAppendsToExistingFile(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "AGENTS.md")
	original := "# My project\n\nSome existing guidance.\n"
	if err := os.WriteFile(dest, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	action, err := installInstructions(dest, instructionsBlock(instructionsTarget{}), "")
	if err != nil {
		t.Fatal(err)
	}
	if action != "appended" {
		t.Fatalf("action = %q, want appended", action)
	}
	data, _ := os.ReadFile(dest)
	content := string(data)
	if !strings.HasPrefix(content, original) {
		t.Fatalf("existing content not preserved:\n%s", content)
	}
	if !strings.Contains(content, instructionsBeginMarker) {
		t.Fatalf("block not appended:\n%s", content)
	}
}

func TestInstallInstructionsIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "AGENTS.md")
	original := "# My project\n\nSome existing guidance.\n"
	if err := os.WriteFile(dest, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := installInstructions(dest, instructionsBlock(instructionsTarget{}), ""); err != nil {
		t.Fatal(err)
	}
	afterFirst, _ := os.ReadFile(dest)

	action, err := installInstructions(dest, instructionsBlock(instructionsTarget{}), "")
	if err != nil {
		t.Fatal(err)
	}
	if action != "updated" {
		t.Fatalf("action = %q, want updated", action)
	}
	afterSecond, _ := os.ReadFile(dest)
	if string(afterFirst) != string(afterSecond) {
		t.Fatalf("repeated install changed content:\nfirst:\n%s\nsecond:\n%s", afterFirst, afterSecond)
	}
	if strings.Count(string(afterSecond), instructionsBeginMarker) != 1 {
		t.Fatalf("expected exactly one block, got:\n%s", afterSecond)
	}
}

func TestUninstallInstructionsRemovesBlockKeepsContent(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "AGENTS.md")
	original := "# My project\n\nSome existing guidance.\n"
	if err := os.WriteFile(dest, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := installInstructions(dest, instructionsBlock(instructionsTarget{}), ""); err != nil {
		t.Fatal(err)
	}

	action, err := uninstallInstructions(dest)
	if err != nil {
		t.Fatal(err)
	}
	if action != "removed" {
		t.Fatalf("action = %q, want removed", action)
	}
	data, _ := os.ReadFile(dest)
	content := string(data)
	if strings.Contains(content, instructionsBeginMarker) {
		t.Fatalf("block still present:\n%s", content)
	}
	if !strings.Contains(content, "Some existing guidance.") {
		t.Fatalf("user content lost:\n%s", content)
	}
}

func TestUninstallInstructionsDeletesFileWhenOnlyBlock(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "AGENTS.md")
	if _, err := installInstructions(dest, instructionsBlock(instructionsTarget{}), ""); err != nil {
		t.Fatal(err)
	}

	action, err := uninstallInstructions(dest)
	if err != nil {
		t.Fatal(err)
	}
	if action != "removed (file deleted)" {
		t.Fatalf("action = %q, want file deleted", action)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("file should be deleted, stat err = %v", err)
	}
}

func TestUninstallInstructionsAbsent(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "AGENTS.md")

	action, err := uninstallInstructions(dest)
	if err != nil {
		t.Fatal(err)
	}
	if action != "absent" {
		t.Fatalf("action = %q, want absent", action)
	}
}

func TestResolveInstructionsTargets(t *testing.T) {
	all, err := resolveInstructionsTargets(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != len(instructionsTargets()) {
		t.Fatalf("all = %d targets, want %d", len(all), len(instructionsTargets()))
	}

	one, err := resolveInstructionsTargets([]string{"agents", "agents"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(one) != 1 || one[0].key != "agents" {
		t.Fatalf("dedup failed: %#v", one)
	}

	if _, err := resolveInstructionsTargets([]string{"nope"}, false); err == nil {
		t.Fatal("expected error for unknown target")
	}
}
