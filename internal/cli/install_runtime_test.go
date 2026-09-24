package cli

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/downstreammcp"
	"github.com/bryanbarton525/prism/internal/extensions"
	"github.com/bryanbarton525/prism/internal/graphify"
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
	ownedGraphifyConfig := filepath.Join(workspace, ".prism", "graphify.yaml")
	if err := os.MkdirAll(filepath.Dir(ownedGraphifyConfig), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ownedGraphifyConfig, []byte("operator-owned\n"), 0o600); err != nil {
		t.Fatal(err)
	}
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
	if data, err := os.ReadFile(ownedGraphifyConfig); err != nil || string(data) != "operator-owned\n" {
		t.Fatalf("runtime-only install modified Graphify config: data=%q err=%v", data, err)
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
	cmd.Flags().String("runtime-scope", "user", "")
	if err := cmd.Flags().Set("state-dir", gf.stateDir); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("runtime-scope", "user"); err != nil {
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
	t.Setenv("HOME", t.TempDir())
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	gf.stateDir = filepath.Join(home, ".prism")

	cmd := &cobra.Command{}
	cmd.Flags().String("state-dir", "", "")
	cmd.Flags().String("runtime-scope", "user", "")
	if err := cmd.Flags().Set("state-dir", gf.stateDir); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("runtime-scope", "user"); err != nil {
		t.Fatal(err)
	}
	flags := installFlags{runtimeOnly: true, runtimeScope: "user"}
	if err := runInstall(cmd, flags); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRunInstallUsesCustomStateDirWithDefaultRuntimeScope(t *testing.T) {
	originalStateDir := gf.stateDir
	defer func() { gf.stateDir = originalStateDir }()
	gf.stateDir = filepath.Join(t.TempDir(), "custom-state")
	cmd := &cobra.Command{}
	cmd.Flags().String("state-dir", "", "")
	if err := cmd.Flags().Set("state-dir", gf.stateDir); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runInstall(cmd, installFlags{runtimeOnly: true, runtimeScope: "user"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(gf.stateDir, "extensions.yaml")); err != nil {
		t.Fatalf("runtime manifest missing from selected state dir: %v", err)
	}
	if !strings.Contains(out.String(), gf.stateDir) {
		t.Fatalf("output %q omits selected state dir %q", out.String(), gf.stateDir)
	}
}

func TestRunInstallEmbedsCustomStateDirInCodexHostConfig(t *testing.T) {
	originalStateDir := gf.stateDir
	defer func() { gf.stateDir = originalStateDir }()
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
	gf.stateDir = filepath.Join(workspace, "runtime-state")
	cmd := &cobra.Command{}
	cmd.Flags().String("state-dir", "", "")
	if err := cmd.Flags().Set("state-dir", gf.stateDir); err != nil {
		t.Fatal(err)
	}
	cmd.SetOut(&bytes.Buffer{})
	flags := installFlags{
		project: true, yes: true, copyMode: true, runtimeScope: "user",
		targets: []string{"codex"}, skills: []string{"gh-pr-triage"}, specialists: []string{"github-cli"},
	}
	if err := runInstall(cmd, flags); err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile(filepath.Join(workspace, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), "--state-dir") || !strings.Contains(string(config), gf.stateDir) {
		t.Fatalf("Codex host config omits selected state dir: %s", config)
	}
}

func TestRunInstallGraphifyUsesCustomStateDir(t *testing.T) {
	originalStateDir := gf.stateDir
	defer func() { gf.stateDir = originalStateDir }()
	workspace := t.TempDir()
	gf.stateDir = filepath.Join(workspace, "selected-state")
	index := filepath.Join(workspace, "graph.json")
	if err := os.WriteFile(index, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	cmd.Flags().String("state-dir", "", "")
	if err := cmd.Flags().Set("state-dir", gf.stateDir); err != nil {
		t.Fatal(err)
	}
	cmd.SetOut(&bytes.Buffer{})
	flags := installFlags{
		runtimeOnly: true, runtimeScope: "user", graphifyKind: "self-hosted",
		graphifyWorkspace: workspace, graphifyIndex: index, graphifyFingerprint: "source-sha", graphifyServer: "graphify",
	}
	if err := runInstall(cmd, flags); err != nil {
		t.Fatal(err)
	}
	if _, err := graphify.Load(filepath.Join(gf.stateDir, "graphify.yaml")); err != nil {
		t.Fatalf("Graphify binding missing from selected state dir: %v", err)
	}
}

func TestReadInstallAnswerPropagatesEOF(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader(""))
	if _, err := readInstallAnswer(bufio.NewReader(cmd.InOrStdin()), cmd, "Question: "); err == nil {
		t.Fatal("expected EOF error")
	}
}

func TestRuntimeOnlySkillSelectionRequiresSource(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("state-dir", "", "")
	err := runInstall(cmd, installFlags{runtimeOnly: true, runtimeScope: "user", runtimeSkillAll: true})
	if err == nil || !strings.Contains(err.Error(), "require --runtime-skill-source") {
		t.Fatalf("expected source requirement error, got %v", err)
	}
}

func TestPromptSelectRetriesAndCancellationDoesNotApply(t *testing.T) {
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader("9\n2\n"))
	selected, err := promptSelect(newGuidedInput(cmd), "Choices", []string{"one", "two"})
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 || selected[0] != "two" || !strings.Contains(out.String(), "Invalid selection") {
		t.Fatalf("selection=%v output=%q", selected, out.String())
	}

	root := t.TempDir()
	cmd = &cobra.Command{}
	cmd.SetIn(strings.NewReader("all\nall\nall\nsymlink\nuser\n\nnone\ncancel\n"))
	cmd.SetOut(&out)
	cmd.SetContext(context.Background())
	flags := installFlags{project: true, runtimeScope: "user"}
	originalWD, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalWD) })
	if err := runInstall(cmd, flags); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".prism", "install.json")); !os.IsNotExist(err) {
		t.Fatalf("cancellation wrote host transaction: %v", err)
	}
}

func TestRuntimeOnlyImportsTranslatedAgentWithSelectedModel(t *testing.T) {
	source := filepath.Join(t.TempDir(), "imported.md")
	if err := os.WriteFile(source, []byte("---\nid: imported\nname: Imported\ndescription: d\nmodel: old-model\ncontext_budget: 1000\nallowed_skills: [go-helper-fn]\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}

	originalState := gf.stateDir
	t.Cleanup(func() { gf.stateDir = originalState })
	flags := installFlags{runtimeOnly: true, runtimeScope: "user", runtimeAgentSource: source, runtimeAgentAs: "selected-agent", runtimeAgentModel: "self-hosted/model"}
	workspace := t.TempDir()
	oldWD, _ := os.Getwd()
	if err := os.Chdir(workspace); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })
	state := filepath.Join(workspace, ".prism")
	gf.stateDir = state
	cmd := &cobra.Command{}
	cmd.Flags().String("state-dir", "", "")
	if err := cmd.Flags().Set("state-dir", state); err != nil {
		t.Fatal(err)
	}
	flags.runtimeScope = "project"
	if err := runInstall(cmd, flags); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(state, "extensions.yaml"))
	if err != nil || !strings.Contains(string(data), "selected-agent") {
		t.Fatalf("missing imported agent manifest: %v %s", err, data)
	}
	entries, err := extensions.NewLocalAgentService(state).ListManagedAgents(context.Background())
	if err != nil || len(entries) != 1 {
		t.Fatalf("managed agents: %v %#v", err, entries)
	}
	content, err := os.ReadFile(filepath.Join(entries[0].ObjectPath, "selected-agent.md"))
	if err != nil || !strings.Contains(string(content), `model: self-hosted/model`) {
		t.Fatalf("selected model missing: %v %s", err, content)
	}
}

func TestHostSuccessRollsBackFailedRuntimeActivation(t *testing.T) {
	workspace := t.TempDir()
	oldWD, _ := os.Getwd()
	if err := os.Chdir(workspace); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })
	state := filepath.Join(workspace, ".prism")
	oldSource := filepath.Join(t.TempDir(), "demo")
	newSource := filepath.Join(t.TempDir(), "demo")
	for path, content := range map[string]string{
		oldSource: "---\nname: demo\ndescription: old\n---\nold",
		newSource: "---\nname: demo\ndescription: new\n---\nnew",
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := extensions.NewLocalSkillService(state).InstallLocalSkills(context.Background(), extensions.InstallLocalSkillsRequest{Source: oldSource}); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	flags := installFlags{
		project:            true,
		runtimeScope:       "project",
		runtimeSkillSource: newSource,
		targets:            []string{"copilot"},
		skills:             []string{"go-helper-fn"},
		specialists:        []string{"go-helper"},
		yes:                true,
	}
	err := runInstall(cmd, flags)
	if err == nil || !strings.Contains(err.Error(), "host installation succeeded but runtime activation failed") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".prism", "install.json")); err != nil {
		t.Fatalf("host transaction should remain installed: %v", err)
	}
	entries, err := extensions.NewLocalSkillService(state).ListManagedSkills(context.Background())
	if err != nil || len(entries) != 1 {
		t.Fatalf("runtime rollback: %v %#v", err, entries)
	}
	content, err := os.ReadFile(filepath.Join(entries[0].ObjectPath, "SKILL.md"))
	if err != nil || !strings.Contains(string(content), "description: old") {
		t.Fatalf("runtime state was not restored: %v %q", err, content)
	}
}

func TestRuntimeOnlyUpdatesExistingAgentAndAccessAtomically(t *testing.T) {
	workspace := t.TempDir()
	oldWD, _ := os.Getwd()
	if err := os.Chdir(workspace); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })
	state := filepath.Join(workspace, ".prism")
	source := filepath.Join(t.TempDir(), "worker.md")
	data := "---\nid: worker\nname: Old Name\ndescription: d\nmodel: local\ncontext_budget: 100\nallowed_skills: []\n---\nbody"
	if err := os.WriteFile(source, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := extensions.NewLocalAgentService(state).InstallLocalAgent(context.Background(), extensions.InstallLocalAgentRequest{Source: source}); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	cmd.Flags().String("state-dir", "", "")
	flags := installFlags{runtimeOnly: true, runtimeScope: "project", runtimeAgentExisting: "worker", runtimeAgentName: "New Name", runtimeAgentSkillsSet: true, runtimeMCPSet: true, runtimeMCPMode: extensions.MCPAccessModeNone}
	if err := runInstall(cmd, flags); err != nil {
		t.Fatal(err)
	}
	manifest, err := extensions.NewStore(state).LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.MCPAccess == nil || manifest.MCPAccess.Agents["worker"].Mode != extensions.MCPAccessModeNone {
		t.Fatalf("access not in manifest snapshot: %#v", manifest)
	}
	entry := manifest.Entries[0]
	content, err := os.ReadFile(filepath.Join(entry.ObjectPath, "worker.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "name: New Name") || !strings.Contains(string(content), "allowed_skills: []") {
		t.Fatalf("updated agent = %s", content)
	}
}

func TestRuntimeAgentDirectoryImportPreservesSupportFiles(t *testing.T) {
	workspace := t.TempDir()
	oldWD, _ := os.Getwd()
	if err := os.Chdir(workspace); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })
	packageDir := filepath.Join(t.TempDir(), "package")
	if err := os.MkdirAll(filepath.Join(packageDir, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(packageDir, "constitutions"), 0o755); err != nil {
		t.Fatal(err)
	}
	data := "---\nid: imported\nname: Imported\ndescription: d\nmodel: old\ncontext_budget: 100\nallowed_skills: []\nconstitution_path: constitutions/imported.md\n---\nbody"
	if err := os.WriteFile(filepath.Join(packageDir, "agents", "imported.md"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packageDir, "constitutions", "imported.md"), []byte("rules"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	cmd.Flags().String("state-dir", "", "")
	flags := installFlags{runtimeOnly: true, runtimeScope: "project", runtimeAgentSource: packageDir, runtimeAgentSelect: "imported", runtimeAgentModel: "local/model"}
	if err := runInstall(cmd, flags); err != nil {
		t.Fatal(err)
	}
	manifest, err := extensions.NewStore(filepath.Join(workspace, ".prism")).LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(manifest.Entries[0].ObjectPath, "constitutions", "imported.md")); err != nil {
		t.Fatalf("support file missing: %v", err)
	}
}

func TestGraphifyDiscoveryIsOfflineAndFindsUserChoices(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX executable name")
	}
	bin := t.TempDir()
	executable := filepath.Join(bin, graphify.PinnedMCPEntypoint)
	if err := os.WriteFile(executable, []byte("must not be executed"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	state := t.TempDir()
	if err := downstreammcp.Save(filepath.Join(state, "mcp-servers.yaml"), downstreammcp.State{Servers: []downstreammcp.Server{{Name: "hosted", Transport: downstreammcp.TransportStreamableHTTP, URL: "https://example.test/mcp"}}}); err != nil {
		t.Fatal(err)
	}
	discovery, err := discoverGraphifyCandidates(state)
	if err != nil {
		t.Fatal(err)
	}
	if discovery.Executable != executable || len(discovery.Endpoints) != 1 || discovery.Endpoints[0].Name != "hosted" {
		t.Fatalf("discovery = %#v", discovery)
	}
}
