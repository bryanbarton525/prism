package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/bryanbarton525/prism/internal/extensions"
)

func TestBookNavigationAndLocalLinks(t *testing.T) {
	book := filepath.Join("..", "..", "docs", "book")
	index, err := os.ReadFile(filepath.Join(book, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "docs/book/README.md") {
		t.Fatal("main README does not link the book")
	}
	chapters := []string{"quick-start.md", "installation.md", "examples.md", "architecture.md", "testing.md", "troubleshooting.md", "contributing.md"}
	links := regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)
	for _, chapter := range append(chapters, "README.md") {
		if chapter != "README.md" && !strings.Contains(string(index), "("+chapter+")") {
			t.Errorf("index missing %s", chapter)
		}
		data, err := os.ReadFile(filepath.Join(book, chapter))
		if err != nil {
			t.Fatal(err)
		}
		if chapter != "README.md" && !strings.Contains(string(data), "[Contents](README.md)") {
			t.Errorf("%s lacks return navigation", chapter)
		}
		for _, match := range links.FindAllStringSubmatch(string(data), -1) {
			link, _, _ := strings.Cut(match[1], "#")
			if link == "" || strings.Contains(link, "://") {
				continue
			}
			if _, err := os.Stat(filepath.Join(book, link)); err != nil {
				t.Errorf("%s broken link %s: %v", chapter, link, err)
			}
		}
	}
}

func TestBookCommandsMatchCLI(t *testing.T) {
	install := newInstallCmd()
	for _, flag := range []string{"decision-service", "laya-port", "laya-uv", "decision-key-env", "tool-model", "runtime-only", "runtime-scope", "primary-engine", "primary-url", "primary-model"} {
		if install.Flags().Lookup(flag) == nil {
			t.Errorf("book documents missing install flag %s", flag)
		}
	}
	for _, args := range [][]string{{"agent", "set", "linear"}, {"agent", "show", "linear"}, {"default", "set"}, {"default", "show"}} {
		cmd, remaining, err := newMCPAccessCmd().Find(args)
		if err != nil {
			t.Fatal(err)
		}
		if cmd.Name() != args[1] || (args[0] == "agent" && (len(remaining) != 1 || remaining[0] != "linear")) {
			t.Fatalf("command %v resolves to %s, args=%v", args, cmd.CommandPath(), remaining)
		}
	}
	for _, file := range []string{"quick-start.md", "examples.md"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "docs", "book", file))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "--task ") {
			t.Errorf("%s documents unsupported run --task; use stdin or --input", file)
		}
	}
}

func TestDocumentedMCPAccessCommandsExecute(t *testing.T) {
	previous := gf
	gf.stateDir = t.TempDir()
	defer func() { gf = previous }()
	cmd := newMCPAccessCmd()
	cmd.SetArgs([]string{"agent", "set", "linear", "--mode", "none"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	state, configured, err := extensions.LoadMCPAccess(gf.stateDir)
	if err != nil || !configured || state.Agents["linear"].Mode != extensions.MCPAccessModeNone {
		t.Fatalf("state=%+v configured=%v error=%v", state, configured, err)
	}
	cmd = newMCPAccessCmd()
	cmd.SetArgs([]string{"agent", "show", "linear"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}
