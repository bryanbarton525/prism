package installer

import (
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

func TestInstallMCPIncludesRuntimeStateDirInCommands(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, ".prism-runtime")
	_, err := Install(Options{
		Scope:           Project,
		Root:            root,
		Targets:         []string{"copilot", "codex", "opencode", "claude"},
		Skills:          []string{"gh-pr-triage"},
		Specialists:     []string{"github-cli"},
		RuntimeStateDir: stateDir,
		Binary:          "/opt/prism",
	})
	if err != nil {
		t.Fatal(err)
	}

	copilotData, err := os.ReadFile(filepath.Join(root, ".vscode", "mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(copilotData), "--state-dir") || !strings.Contains(string(copilotData), stateDir) {
		t.Fatalf("copilot config missing runtime state dir: %s", copilotData)
	}

	codexData, err := os.ReadFile(filepath.Join(root, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(codexData), "--state-dir") || !strings.Contains(string(codexData), stateDir) {
		t.Fatalf("codex config missing runtime state dir: %s", codexData)
	}

	opencodeData, err := os.ReadFile(filepath.Join(root, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(opencodeData), "--state-dir") || !strings.Contains(string(opencodeData), stateDir) {
		t.Fatalf("opencode config missing runtime state dir: %s", opencodeData)
	}

	claudeData, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(claudeData), "--state-dir") || !strings.Contains(string(claudeData), stateDir) {
		t.Fatalf("claude config missing runtime state dir: %s", claudeData)
	}
}

func TestGraphifyCapabilityCatalogAndHostWrappers(t *testing.T) {
	skills, specialists, err := Catalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, skill := range skills {
		if skill == "graphify-query" {
			t.Fatal("internal Graphify query instructions must not be exported as a host skill")
		}
	}
	found := false
	for _, specialist := range specialists {
		if specialist.ID == "repo-investigator" {
			found = true
		}
	}
	if !found {
		t.Fatalf("repo-investigator missing from installer catalog: %#v", specialists)
	}

	root := t.TempDir()
	_, err = Install(Options{
		Scope:       Project,
		Root:        root,
		Targets:     []string{"claude", "codex"},
		Specialists: []string{"repo-investigator"},
		Binary:      "/opt/prism",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"claude", "codex"} {
		data, err := os.ReadFile(filepath.Join(hostLayout(target, root, Project).agents, wrapperFilename(target, "repo-investigator")))
		if err != nil {
			t.Fatal(err)
		}
		wrapper := string(data)
		for _, marker := range []string{"run_agent", "architecture", "relationships", "dependency paths", "change-impact", "ordinary one-file lookup"} {
			if !strings.Contains(wrapper, marker) {
				t.Fatalf("%s wrapper missing %q:\n%s", target, marker, wrapper)
			}
		}
		for _, internal := range []string{"query_graph", "get_node", "get_neighbors", "shortest_path", "graphify-mcp"} {
			if strings.Contains(wrapper, internal) {
				t.Fatalf("%s wrapper exports internal Graphify workflow %q:\n%s", target, internal, wrapper)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "graphify-query")); !os.IsNotExist(err) {
		t.Fatalf("internal Graphify query skill was exported to host: %v", err)
	}
}
