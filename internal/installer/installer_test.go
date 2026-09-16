package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallCopyAndUninstall(t *testing.T) {
	root := t.TempDir()
	opts := Options{
		Scope:       Project,
		Root:        root,
		Targets:     []string{"copilot"},
		Skills:      []string{"gh-pr-triage"},
		Specialists: []string{"github-cli"},
		Copy:        true,
		Binary:      "/opt/bin/prism",
	}
	plan, err := Install(opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(root, ".agents", "skills", "gh-pr-triage", "SKILL.md"),
		filepath.Join(root, ".github", "agents", "github-cli.agent.md"),
		filepath.Join(root, ".vscode", "mcp.json"),
		plan.ManifestPath,
	} {
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("missing %s: %v", path, err)
		}
	}
	mcpData, _ := os.ReadFile(filepath.Join(root, ".vscode", "mcp.json"))
	if strings.Contains(string(mcpData), "--root") || !strings.Contains(string(mcpData), "/opt/bin/prism") {
		t.Fatalf("unexpected MCP config: %s", mcpData)
	}
	wrapper, _ := os.ReadFile(filepath.Join(root, ".github", "agents", "github-cli.agent.md"))
	if !strings.Contains(string(wrapper), "`run_agent`") || !strings.Contains(string(wrapper), "`github-cli`") {
		t.Fatalf("wrapper does not delegate through Prism: %s", wrapper)
	}
	manifest, err := Uninstall(Project, root, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Entries) == 0 {
		t.Fatal("empty manifest")
	}
	if _, err := os.Stat(filepath.Join(root, ".github", "agents", "github-cli.agent.md")); !os.IsNotExist(err) {
		t.Fatalf("wrapper still exists: %v", err)
	}
}

func TestSymlinkAndIdempotentReinstall(t *testing.T) {
	root := t.TempDir()
	opts := Options{Scope: Project, Root: root, Targets: []string{"claude"}, Skills: []string{"gh-pr-triage"}, Specialists: []string{"github-cli"}, Binary: "/opt/prism"}
	if _, err := Install(opts); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, ".claude", "skills", "gh-pr-triage")
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("host skill is not a symlink: %v %#v", err, info)
	}
	if _, err := Install(opts); err != nil {
		t.Fatalf("idempotent reinstall: %v", err)
	}
}

func TestLinkStagingPreservesUnrelatedUserFile(t *testing.T) {
	root := t.TempDir()
	sentinel := filepath.Join(root, ".claude", "skills", ".prism-stage-link-gh-pr-triage")
	if err := os.MkdirAll(filepath.Dir(sentinel), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sentinel, []byte("user file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(Options{Scope: Project, Root: root, Targets: []string{"claude"}, Skills: []string{"gh-pr-triage"}, Binary: "prism"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(sentinel)
	if err != nil || string(data) != "user file" {
		t.Fatalf("user staging-name file changed: %q, %v", data, err)
	}
}

func TestAdapterFailureRollsBack(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, ".vscode", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Install(Options{Scope: Project, Root: root, Targets: []string{"copilot"}, Skills: []string{"gh-pr-triage"}, Specialists: []string{"github-cli"}, Binary: "/opt/prism"})
	if err == nil {
		t.Fatal("expected invalid config error")
	}
	if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "gh-pr-triage")); !os.IsNotExist(err) {
		t.Fatalf("skill was not rolled back: %v", err)
	}
	data, _ := os.ReadFile(config)
	if string(data) != "not json" {
		t.Fatalf("configuration changed: %q", data)
	}
}

func TestInstallMCPHandlesExistingJSONNullWithoutPanic(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, ".vscode", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("null\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(Options{Scope: Project, Root: root, Targets: []string{"copilot"}, Binary: "prism"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(config)
	if err != nil || !strings.Contains(string(data), `"prism"`) {
		t.Fatalf("JSON null config was not updated safely: %q, %v", data, err)
	}
}

func TestInstallAndUninstallRejectTamperedManifestPaths(t *testing.T) {
	for _, symlinkedParent := range []bool{false, true} {
		t.Run(map[bool]string{false: "outside-path", true: "symlinked-parent"}[symlinkedParent], func(t *testing.T) {
			root := t.TempDir()
			outside := t.TempDir()
			victim := filepath.Join(outside, "victim")
			if err := os.WriteFile(victim, []byte("keep"), 0o600); err != nil {
				t.Fatal(err)
			}
			entryPath := victim
			if symlinkedParent {
				if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				entryPath = filepath.Join(root, "linked", "victim")
			}
			manifestPath := filepath.Join(root, ".prism", "install.json")
			if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(Manifest{Version: 1, Scope: Project, Entries: []Entry{{Path: entryPath, Kind: "skill"}}})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(manifestPath, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Install(Options{Scope: Project, Root: root}); err == nil {
				t.Fatal("reinstall accepted tampered manifest path")
			}
			if _, err := Uninstall(Project, root, false); err == nil {
				t.Fatal("uninstall accepted tampered manifest path")
			}
			content, err := os.ReadFile(victim)
			if err != nil || string(content) != "keep" {
				t.Fatalf("outside victim changed: %q, %v", content, err)
			}
		})
	}
}

func TestUpgradeRemovesStaleManagedTarget(t *testing.T) {
	root := t.TempDir()
	base := Options{Scope: Project, Root: root, Skills: []string{"gh-pr-triage"}, Specialists: []string{"github-cli"}, Copy: true, Binary: "/opt/prism"}
	first := base
	first.Targets = []string{"copilot"}
	if _, err := Install(first); err != nil {
		t.Fatal(err)
	}
	second := base
	second.Targets = []string{"claude"}
	if _, err := Install(second); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".github", "agents", "github-cli.agent.md")); !os.IsNotExist(err) {
		t.Fatalf("stale wrapper remains: %v", err)
	}
	oldConfig, err := os.ReadFile(filepath.Join(root, ".vscode", "mcp.json"))
	if err != nil || strings.Contains(string(oldConfig), "prism") {
		t.Fatalf("stale MCP configuration remains: %v %v", err, oldConfig)
	}
}

func TestEveryHostAdapterInBothScopes(t *testing.T) {
	for _, scope := range []Scope{Project, Global} {
		for _, target := range Targets {
			t.Run(string(scope)+"/"+target, func(t *testing.T) {
				root := t.TempDir()
				opts := Options{Scope: scope, Root: root, Targets: []string{target}, Skills: []string{"gh-pr-triage"}, Specialists: []string{"github-cli"}, Copy: true, Binary: "/opt/prism"}
				plan, err := Install(opts)
				if err != nil {
					t.Fatal(err)
				}
				layout := hostLayout(target, root, scope)
				wrapper, err := os.ReadFile(filepath.Join(layout.agents, wrapperFilename(target, "github-cli")))
				if err != nil || !strings.Contains(string(wrapper), "run_agent") {
					t.Fatalf("wrapper: %v %s", err, wrapper)
				}
				config, err := os.ReadFile(layout.config)
				if err != nil || !strings.Contains(string(config), "/opt/prism") || strings.Contains(string(config), "--root") {
					t.Fatalf("config: %v %s", err, config)
				}
				if _, err := os.Stat(plan.ManifestPath); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestInstallRejectsUnmanagedCollision(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, ".agents", "skills", "gh-pr-triage")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "mine.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Install(Options{Scope: Project, Root: root, Targets: []string{"copilot"}, Skills: []string{"gh-pr-triage"}, Binary: "prism"})
	if err == nil || !strings.Contains(err.Error(), "unmanaged collision") {
		t.Fatalf("error = %v", err)
	}
	data, readErr := os.ReadFile(filepath.Join(dest, "mine.txt"))
	if readErr != nil || string(data) != "keep" {
		t.Fatalf("collision was not preserved: %q, %v", data, readErr)
	}
}

func TestBuildPlanDryRunDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	opts := Options{Scope: Project, Root: root, Targets: []string{"claude"}, Skills: []string{"go-helper-fn"}, Specialists: []string{"go-helper"}, DryRun: true}
	plan, err := Install(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Paths) == 0 {
		t.Fatal("empty dry-run plan")
	}
	if _, err := os.Stat(plan.ManifestPath); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote manifest: %v", err)
	}
}
