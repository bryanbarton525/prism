package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	runErr := fn()
	_ = w.Close()
	os.Stdout = old
	data, _ := io.ReadAll(r)
	return string(data), runErr
}

func TestAgentManagedLifecycleCommands(t *testing.T) {
	origState := gf.stateDir
	origJSON := gf.jsonOut
	gf.stateDir = t.TempDir()
	gf.jsonOut = false
	defer func() {
		gf.stateDir = origState
		gf.jsonOut = origJSON
	}()

	source := filepath.Join(t.TempDir(), "managed-agent.md")
	if err := os.WriteFile(source, []byte(`---
id: managed-agent
name: Managed Agent
description: d
model: llama3.1:8b
context_budget: 16000
allowed_skills: [gh-pr-triage]
latency_budget_ms: 10000
---
Body`), 0o644); err != nil {
		t.Fatal(err)
	}

	add := newAgentAddCmd()
	add.SetArgs([]string{source})
	if err := add.Execute(); err != nil {
		t.Fatal(err)
	}

	out, err := captureStdout(t, func() error {
		cmd := newAgentManagedListCmd()
		return cmd.Execute()
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "managed-agent") {
		t.Fatalf("list output = %q", out)
	}
}
