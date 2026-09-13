package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestValidateRuntimeScope(t *testing.T) {
	if err := validateRuntimeScope("project"); err != nil {
		t.Fatal(err)
	}
	if err := validateRuntimeScope("user"); err != nil {
		t.Fatal(err)
	}
	if err := validateRuntimeScope("invalid"); err == nil {
		t.Fatal("expected invalid runtime scope error")
	}
}

func TestRunInstallRuntimeOnlyUsesSelectedScope(t *testing.T) {
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	if err := os.Chdir(workspace); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalWD); err != nil {
			t.Fatal(err)
		}
	})
	cmd := &cobra.Command{}
	cmd.Flags().String("state-dir", "", "")
	var out bytes.Buffer
	cmd.SetOut(&out)
	flags := installFlags{runtimeOnly: true, runtimeScope: "project"}
	if err := runInstall(cmd, flags); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(cwd, ".prism")
	if !strings.Contains(out.String(), "Initialized runtime extension state") || !strings.Contains(out.String(), expected) {
		t.Fatalf("output=%q expected state dir %q", out.String(), expected)
	}
	if _, err := os.Stat(filepath.Join(expected, "extensions.yaml")); err != nil {
		t.Fatalf("runtime manifest was not initialized: %v", err)
	}
}

func TestRunInstallRejectsGlobalProjectRuntimeScope(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("state-dir", "", "")
	flags := installFlags{global: true, runtimeOnly: true, runtimeScope: "project"}
	err := runInstall(cmd, flags)
	if err == nil || !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("expected incompatibility error, got %v", err)
	}
}

func TestRunInstallRejectsConflictingStateDirAndScope(t *testing.T) {
	originalStateDir := gf.stateDir
	defer func() { gf.stateDir = originalStateDir }()
	gf.stateDir = t.TempDir()

	cmd := &cobra.Command{}
	cmd.Flags().String("state-dir", "", "")
	if err := cmd.Flags().Set("state-dir", gf.stateDir); err != nil {
		t.Fatal(err)
	}
	flags := installFlags{runtimeOnly: true, runtimeScope: "user"}
	err := runInstall(cmd, flags)
	if err == nil || !strings.Contains(err.Error(), "conflicts with --runtime-scope") {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestRunInstallAllowsMatchingStateDirAndScope(t *testing.T) {
	originalStateDir := gf.stateDir
	defer func() { gf.stateDir = originalStateDir }()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	gf.stateDir = filepath.Join(home, ".prism")

	cmd := &cobra.Command{}
	cmd.Flags().String("state-dir", "", "")
	if err := cmd.Flags().Set("state-dir", gf.stateDir); err != nil {
		t.Fatal(err)
	}
	flags := installFlags{runtimeOnly: true, runtimeScope: "user"}
	if err := runInstall(cmd, flags); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
