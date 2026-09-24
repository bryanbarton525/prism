//go:build !windows

package extensions

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestLocalAgentAndSkillRejectNamedPipesBeforeReading(t *testing.T) {
	makePipe := func(path string) {
		t.Helper()
		if err := syscall.Mkfifo(path, 0o600); err != nil {
			t.Skipf("named pipes unavailable: %v", err)
		}
	}
	state := t.TempDir()
	barePipe := filepath.Join(t.TempDir(), "agent.md")
	makePipe(barePipe)
	if _, err := NewLocalAgentService(state).InstallLocalAgent(context.Background(), InstallLocalAgentRequest{Source: barePipe}); err == nil {
		t.Fatal("named pipe accepted as agent source")
	}
	agentPackage := filepath.Join(t.TempDir(), "worker")
	if err := os.MkdirAll(agentPackage, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentPackage, "worker.md"), reviewAgent("worker", ""), 0o600); err != nil {
		t.Fatal(err)
	}
	makePipe(filepath.Join(agentPackage, "support.pipe"))
	if _, err := NewLocalAgentService(state).InstallLocalAgent(context.Background(), InstallLocalAgentRequest{Source: agentPackage}); err == nil {
		t.Fatal("named pipe accepted in agent package")
	}
	skillPackage := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(skillPackage, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillPackage, "SKILL.md"), []byte("---\nname: demo\ndescription: d\n---\n# demo"), 0o600); err != nil {
		t.Fatal(err)
	}
	makePipe(filepath.Join(skillPackage, "support.pipe"))
	if _, err := NewLocalSkillService(state).InstallLocalSkills(context.Background(), InstallLocalSkillsRequest{Source: skillPackage}); err == nil {
		t.Fatal("named pipe accepted in skill package")
	}
}
