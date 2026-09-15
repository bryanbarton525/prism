package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bryanbarton525/prism/internal/extensions"
)

func TestRunnerLoadsManagedAgentAndSkillFromCatalogSnapshot(t *testing.T) {
	temp := t.TempDir()
	managedAgentPath := filepath.Join(temp, "managed-agent.md")
	if err := os.WriteFile(managedAgentPath, []byte(`---
id: managed-agent
name: Managed Agent
description: Managed extension agent.
model: llama3.1:8b-instruct-q6_K
context_budget: 16000
allowed_skills: [managed-skill]
latency_budget_ms: 10000
---
Managed body.`), 0o644); err != nil {
		t.Fatal(err)
	}
	managedSkillDir := filepath.Join(temp, "managed-skill")
	if err := os.MkdirAll(managedSkillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(managedSkillDir, "SKILL.md"), []byte(`---
name: managed-skill
description: Managed extension skill.
---
# Managed skill`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	base := os.DirFS(repoRoot)
	agentData, err := os.ReadFile(managedAgentPath)
	if err != nil {
		t.Fatal(err)
	}
	agentDigest := sha256.Sum256(agentData)
	skillDigest, err := extensions.DigestDirectory(managedSkillDir)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := extensions.ComposeCatalog(extensions.ComposeInput{
		BundleFS: base,
		Manifest: extensions.Manifest{
			Version: extensions.ManifestVersion,
			Entries: []extensions.ManifestEntry{
				{Identity: "managed-agent", Kind: "agent", ObjectPath: managedAgentPath, Digest: hex.EncodeToString(agentDigest[:]), Source: "test"},
				{Identity: "managed-skill", Kind: "skill", ObjectPath: managedSkillDir, Digest: skillDigest, Source: "test"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	runner, err := New(Config{
		BundleFS:          base,
		ExtensionSnapshot: &snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}
	agents, err := runner.ListAgents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, summary := range agents {
		if summary.ID == "managed-agent" {
			found = true
		}
	}
	if !found {
		t.Fatalf("managed agent not listed: %#v", agents)
	}
	spec, err := runner.GetSpec(context.Background(), "managed-agent")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(spec.Description, "Managed extension") {
		t.Fatalf("unexpected managed spec: %#v", spec)
	}
}
