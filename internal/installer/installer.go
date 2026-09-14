package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	prism "github.com/bryanbarton525/prism"
	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/buildinfo"
	"github.com/bryanbarton525/prism/internal/graphify"
	"github.com/bryanbarton525/prism/internal/skill"
)

type Scope string

const (
	Project Scope = "project"
	Global  Scope = "global"
)

var Targets = []string{"codex", "copilot", "antigravity", "claude", "opencode"}

type Options struct {
	Scope           Scope
	Root            string
	Targets         []string
	Skills          []string
	Specialists     []string
	RuntimeStateDir string
	Copy            bool
	Force           bool
	DryRun          bool
	Binary          string
}

type Entry struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Digest string `json:"sha256,omitempty"`
}

type Manifest struct {
	Version      int       `json:"manifest_version"`
	PrismVersion string    `json:"prism_version"`
	BundleDigest string    `json:"bundle_digest"`
	Scope        Scope     `json:"scope"`
	InstalledAt  time.Time `json:"installed_at"`
	Entries      []Entry   `json:"entries"`
}

type Plan struct {
	Version      string
	BundleDigest string
	Scope        Scope
	Targets      []string
	Skills       []string
	Specialists  []string
	Paths        []string
	ManifestPath string
}

func Catalog() (skills []string, specialists []agent.Summary, err error) {
	if err := ValidateGraphifyCapability(); err != nil {
		return nil, nil, err
	}
	entries, err := fs.ReadDir(prism.BundleFS(), "skills")
	if err != nil {
		return nil, nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := fs.Stat(prism.BundleFS(), filepath.ToSlash(filepath.Join("skills", entry.Name(), "SKILL.md"))); err == nil {
			if internalOnlySkill(entry.Name()) {
				continue
			}
			skills = append(skills, entry.Name())
		}
	}
	sort.Strings(skills)
	agentFS, _ := fs.Sub(prism.BundleFS(), "agents")
	registry := agent.NewRegistry(agentFS)
	if err := registry.Load(); err != nil {
		return nil, nil, err
	}
	return skills, registry.List(), nil
}

func internalOnlySkill(name string) bool {
	return name == "graphify-query"
}

func ValidateGraphifyCapability() error {
	required := []string{
		"agents/repo-investigator.md",
		"constitutions/repo-investigator.md",
		"skills/graphify-query/SKILL.md",
		"skills/graphify-query/references/REFERENCE.md",
		"skills/graphify-query/references/GRAPHIFY-RELEASE.json",
		"skills/graphify-query/scripts/collect.sh",
		"skills/graphify-query/evals/smoke.yaml",
	}
	for _, name := range required {
		if _, err := fs.Stat(prism.BundleFS(), name); err != nil {
			return fmt.Errorf("Graphify bundle asset %q: %w", name, err)
		}
	}
	metadata, err := fs.ReadFile(prism.BundleFS(), "skills/graphify-query/references/GRAPHIFY-RELEASE.json")
	if err != nil {
		return err
	}
	if err := graphify.ValidateReleaseMetadata(metadata); err != nil {
		return err
	}
	agents, err := fs.Sub(prism.BundleFS(), "agents")
	if err != nil {
		return err
	}
	registry := agent.NewRegistry(agents)
	if err := registry.Load(); err != nil {
		return err
	}
	spec, err := registry.Get("repo-investigator")
	if err != nil {
		return err
	}
	if !spec.AllowsSkill("graphify-query") || !contains(spec.Tools, "graphify") {
		return fmt.Errorf("repo-investigator must retain graphify-query and fixed Graphify capability")
	}
	skills, err := fs.Sub(prism.BundleFS(), "skills")
	if err != nil {
		return err
	}
	if err := skill.ValidateAuthoringStructure(skills, "graphify-query"); err != nil {
		return err
	}
	return nil
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func BuildPlan(opts Options) (Plan, error) {
	if opts.Scope == "" {
		opts.Scope = Project
	}
	if opts.Scope != Project && opts.Scope != Global {
		return Plan{}, fmt.Errorf("unknown install scope %q", opts.Scope)
	}
	root, err := scopeRoot(opts)
	if err != nil {
		return Plan{}, err
	}
	skills, agents, err := Catalog()
	if err != nil {
		return Plan{}, err
	}
	selectedSkills, err := selectNames(opts.Skills, skills, "skill")
	if err != nil {
		return Plan{}, err
	}
	availableAgents := make([]string, 0, len(agents))
	for _, item := range agents {
		availableAgents = append(availableAgents, item.ID)
	}
	selectedAgents, err := selectNames(opts.Specialists, availableAgents, "specialist")
	if err != nil {
		return Plan{}, err
	}
	selectedTargets, err := selectNames(opts.Targets, Targets, "target")
	if err != nil {
		return Plan{}, err
	}
	info := buildinfo.Current()
	plan := Plan{Version: info.Version, BundleDigest: prism.BundleDigest(), Scope: opts.Scope, Targets: selectedTargets, Skills: selectedSkills, Specialists: selectedAgents, ManifestPath: filepath.Join(root, ".prism", "install.json")}
	plan.Paths = plannedPaths(root, plan, opts.Copy)
	return plan, nil
}

func Install(opts Options) (Plan, error) {
	if strings.TrimSpace(opts.RuntimeStateDir) != "" {
		absolute, err := filepath.Abs(opts.RuntimeStateDir)
		if err != nil {
			return Plan{}, fmt.Errorf("canonicalizing runtime state directory: %w", err)
		}
		opts.RuntimeStateDir = absolute
	}
	plan, err := BuildPlan(opts)
	if err != nil || opts.DryRun {
		return plan, err
	}
	root, _ := scopeRoot(opts)
	old, _ := LoadManifest(plan.ManifestPath)
	tx := &transaction{}
	defer tx.rollback()
	managed := make(map[string]bool)
	for _, entry := range old.Entries {
		managed[filepath.Clean(entry.Path)] = true
	}
	var entries []Entry
	canonical := filepath.Join(root, ".agents", "skills")
	for _, name := range plan.Skills {
		dest := filepath.Join(canonical, name)
		if err = installEmbeddedDir(tx, "skills/"+name, dest, managed, opts.Force); err != nil {
			return plan, err
		}
		entries = append(entries, entryFor(dest, "skill"))
	}
	for _, target := range plan.Targets {
		layout := hostLayout(target, root, opts.Scope)
		for _, name := range plan.Skills {
			dest := filepath.Join(layout.skills, name)
			if filepath.Clean(dest) == filepath.Clean(filepath.Join(canonical, name)) {
				continue
			}
			if opts.Copy {
				err = installEmbeddedDir(tx, "skills/"+name, dest, managed, opts.Force)
			} else {
				err = installLink(tx, filepath.Join(canonical, name), dest, managed, opts.Force)
				if err != nil {
					err = installEmbeddedDir(tx, "skills/"+name, dest, managed, opts.Force)
				}
			}
			if err != nil {
				return plan, err
			}
			entries = append(entries, entryFor(dest, "skill-link"))
		}
		for _, id := range plan.Specialists {
			spec, findErr := lookupAgent(id)
			if findErr != nil {
				return plan, findErr
			}
			dest := filepath.Join(layout.agents, wrapperFilename(target, id))
			data := []byte(renderWrapper(target, spec))
			if err = tx.writeFile(dest, data, 0o644, managed, opts.Force); err != nil {
				return plan, err
			}
			entries = append(entries, entryForBytes(dest, "specialist", data))
		}
		if err = installMCP(tx, target, layout.config, opts.Binary, opts.RuntimeStateDir, managed, opts.Force); err != nil {
			return plan, fmt.Errorf("%s MCP configuration: %w", target, err)
		}
		entries = append(entries, entryFor(layout.config, "mcp-config"))
	}
	newSet := make(map[string]bool)
	for _, entry := range entries {
		newSet[filepath.Clean(entry.Path)] = true
	}
	for _, stale := range old.Entries {
		if !newSet[filepath.Clean(stale.Path)] {
			if stale.Kind == "mcp-config" {
				err = removeMCP(tx, stale.Path)
			} else {
				err = tx.remove(stale.Path)
			}
			if err != nil {
				return plan, err
			}
		}
	}
	manifest := Manifest{Version: 1, PrismVersion: plan.Version, BundleDigest: plan.BundleDigest, Scope: plan.Scope, InstalledAt: time.Now().UTC(), Entries: entries}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	data = append(data, '\n')
	if err = tx.writeFile(plan.ManifestPath, data, 0o644, managed, true); err != nil {
		return plan, err
	}
	tx.commit()
	return plan, nil
}

func Uninstall(scope Scope, rootOverride string, dryRun bool) (Manifest, error) {
	root, err := scopeRoot(Options{Scope: scope, Root: rootOverride})
	if err != nil {
		return Manifest{}, err
	}
	path := filepath.Join(root, ".prism", "install.json")
	manifest, err := LoadManifest(path)
	if err != nil {
		return Manifest{}, err
	}
	if dryRun {
		return manifest, nil
	}
	tx := &transaction{}
	for i := len(manifest.Entries) - 1; i >= 0; i-- {
		entry := manifest.Entries[i]
		if entry.Kind == "mcp-config" {
			if err := removeMCP(tx, entry.Path); err != nil {
				tx.rollback()
				return manifest, err
			}
			continue
		}
		if err := tx.remove(entry.Path); err != nil {
			tx.rollback()
			return manifest, err
		}
	}
	if err := tx.remove(path); err != nil {
		tx.rollback()
		return manifest, err
	}
	tx.commit()
	return manifest, nil
}

func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

type layout struct{ skills, agents, config string }

func hostLayout(target, root string, scope Scope) layout {
	switch target {
	case "codex":
		return layout{filepath.Join(root, ".codex", "skills"), filepath.Join(root, ".codex", "agents"), filepath.Join(root, ".codex", "config.toml")}
	case "copilot":
		if scope == Global {
			return layout{filepath.Join(root, ".agents", "skills"), filepath.Join(root, ".copilot", "agents"), vscodeUserMCP(root)}
		}
		return layout{filepath.Join(root, ".agents", "skills"), filepath.Join(root, ".github", "agents"), filepath.Join(root, ".vscode", "mcp.json")}
	case "antigravity":
		if scope == Global {
			return layout{filepath.Join(root, ".gemini", "config", "skills"), filepath.Join(root, ".gemini", "config", "agents"), filepath.Join(root, ".gemini", "antigravity", "mcp_config.json")}
		}
		return layout{filepath.Join(root, ".agents", "skills"), filepath.Join(root, ".agents", "agents"), filepath.Join(root, ".gemini", "settings.json")}
	case "claude":
		config := filepath.Join(root, ".mcp.json")
		if scope == Global {
			config = filepath.Join(root, ".claude.json")
		}
		return layout{filepath.Join(root, ".claude", "skills"), filepath.Join(root, ".claude", "agents"), config}
	case "opencode":
		base := filepath.Join(root, ".opencode")
		config := filepath.Join(root, "opencode.json")
		if scope == Global {
			base = filepath.Join(root, ".config", "opencode")
			config = filepath.Join(base, "opencode.json")
		}
		return layout{filepath.Join(base, "skills"), filepath.Join(base, "agents"), config}
	}
	return layout{}
}

func vscodeUserMCP(root string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(root, "Library", "Application Support", "Code", "User", "mcp.json")
	case "windows":
		return filepath.Join(root, "AppData", "Roaming", "Code", "User", "mcp.json")
	default:
		return filepath.Join(root, ".config", "Code", "User", "mcp.json")
	}
}

func plannedPaths(root string, plan Plan, copyMode bool) []string {
	var out []string
	for _, skill := range plan.Skills {
		out = append(out, filepath.Join(root, ".agents", "skills", skill))
	}
	for _, target := range plan.Targets {
		layout := hostLayout(target, root, plan.Scope)
		for _, skill := range plan.Skills {
			out = append(out, filepath.Join(layout.skills, skill))
		}
		for _, specialist := range plan.Specialists {
			out = append(out, filepath.Join(layout.agents, wrapperFilename(target, specialist)))
		}
		out = append(out, layout.config)
	}
	out = append(out, plan.ManifestPath)
	return unique(out)
}

func wrapperFilename(target, id string) string {
	if target == "codex" {
		return id + ".toml"
	}
	if target == "copilot" {
		return id + ".agent.md"
	}
	return id + ".md"
}

func renderWrapper(target string, spec *agent.Spec) string {
	skills := strings.Join(spec.AllowedSkills, ", ")
	body := hostDelegationBody(spec, skills)
	if target == "codex" {
		return fmt.Sprintf("name = %q\ndescription = %q\ndeveloper_instructions = %q\n", spec.Name, spec.Description, body)
	}
	front := fmt.Sprintf("---\nname: %s\ndescription: %s\n", spec.ID, yamlQuote(spec.Description))
	switch target {
	case "claude":
		front += "tools: mcp__prism__run_agent\nmodel: inherit\n"
	case "opencode":
		front += "mode: subagent\ntools:\n  prism_*: true\n"
	case "copilot":
		front += "tools: ['prism/*']\n"
	}
	return front + "---\n\n# " + spec.Name + "\n\n" + body + "\n"
}

func hostDelegationBody(spec *agent.Spec, skills string) string {
	delegation := fmt.Sprintf("Delegate this specialist's work to the Prism MCP server by calling `run_agent` with `agent_id` `%s`, one or more `skill_names` from [%s], the user's bounded task, and `workspace.root` when repository evidence is needed. Prism's compiled constitution, model, tools, and policy are authoritative. Return Prism's evidence and result without inventing missing evidence.", spec.ID, skills)
	if spec.ID != "repo-investigator" {
		return delegation
	}
	return "Use this specialist for repository architecture, cross-component relationships, dependency paths, and change-impact investigations. Do not route an ordinary one-file lookup, a direct file read, or a simple symbol search here. " + delegation
}

func lookupAgent(id string) (*agent.Spec, error) {
	agentFS, _ := fs.Sub(prism.BundleFS(), "agents")
	registry := agent.NewRegistry(agentFS)
	if err := registry.Load(); err != nil {
		return nil, err
	}
	return registry.Get(id)
}

func installMCP(tx *transaction, target, path, binary, runtimeStateDir string, managed map[string]bool, force bool) error {
	if binary == "" {
		var err error
		binary, err = os.Executable()
		if err != nil {
			return err
		}
	}
	args := []string{"mcp", "serve"}
	if strings.TrimSpace(runtimeStateDir) != "" {
		absolute, err := filepath.Abs(runtimeStateDir)
		if err != nil {
			return fmt.Errorf("canonicalizing runtime state directory: %w", err)
		}
		args = append(args, "--state-dir", absolute)
	}
	if target == "codex" {
		const begin, end = "# BEGIN PRISM MCP", "# END PRISM MCP"
		block := fmt.Sprintf("%s\n[mcp_servers.prism]\ncommand = %q\nargs = %s\n%s\n", begin, binary, tomlStringArray(args), end)
		old, _ := os.ReadFile(path)
		updated := replaceBlock(string(old), begin, end, block)
		return tx.writeConfig(path, []byte(updated), managed, force)
	}
	root := map[string]any{}
	if data, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		if err := json.Unmarshal(stripJSONComments(data), &root); err != nil {
			if !force {
				return fmt.Errorf("cannot preserve existing JSON configuration: %w", err)
			}
			root = map[string]any{}
		}
	}
	switch target {
	case "copilot":
		servers := object(root, "servers")
		servers["prism"] = map[string]any{"type": "stdio", "command": binary, "args": args}
	case "opencode":
		servers := object(root, "mcp")
		command := append([]string{binary}, args...)
		servers["prism"] = map[string]any{"type": "local", "command": command, "enabled": true}
	default:
		servers := object(root, "mcpServers")
		servers["prism"] = map[string]any{"command": binary, "args": args}
	}
	data, _ := json.MarshalIndent(root, "", "  ")
	return tx.writeConfig(path, append(data, '\n'), managed, force)
}

func tomlStringArray(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("%q", value))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func removeMCP(tx *transaction, path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if strings.HasSuffix(path, ".toml") {
		updated := replaceBlock(string(data), "# BEGIN PRISM MCP", "# END PRISM MCP", "")
		return tx.writeFile(path, []byte(updated), 0o644, map[string]bool{filepath.Clean(path): true}, true)
	}
	root := map[string]any{}
	if err := json.Unmarshal(stripJSONComments(data), &root); err != nil {
		return err
	}
	for _, key := range []string{"servers", "mcpServers", "mcp"} {
		if values, ok := root[key].(map[string]any); ok {
			delete(values, "prism")
		}
	}
	out, _ := json.MarshalIndent(root, "", "  ")
	return tx.writeFile(path, append(out, '\n'), 0o644, map[string]bool{filepath.Clean(path): true}, true)
}

func object(root map[string]any, key string) map[string]any {
	if value, ok := root[key].(map[string]any); ok {
		return value
	}
	value := map[string]any{}
	root[key] = value
	return value
}

func installEmbeddedDir(tx *transaction, source, dest string, managed map[string]bool, force bool) error {
	var files []string
	err := fs.WalkDir(prism.BundleFS(), source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(dest), ".prism-stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	payload := filepath.Join(stage, "payload")
	for _, path := range files {
		data, err := fs.ReadFile(prism.BundleFS(), path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(filepath.FromSlash(source), filepath.FromSlash(path))
		out := filepath.Join(payload, rel)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(out, data, 0o644); err != nil {
			return err
		}
	}
	if err := tx.prepare(dest, managed, force); err != nil {
		return err
	}
	return os.Rename(payload, dest)
}

func installLink(tx *transaction, source, dest string, managed map[string]bool, force bool) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	stage := filepath.Join(filepath.Dir(dest), ".prism-stage-link-"+filepath.Base(dest))
	_ = os.Remove(stage)
	if err := os.Symlink(source, stage); err != nil {
		return err
	}
	defer os.Remove(stage)
	if err := tx.prepare(dest, managed, force); err != nil {
		return err
	}
	return os.Rename(stage, dest)
}

type snapshot struct {
	path, backup string
	existed      bool
}

type transaction struct {
	snapshots []snapshot
	done      bool
}

func (t *transaction) prepare(path string, managed map[string]bool, force bool) error {
	clean := filepath.Clean(path)
	_, err := os.Lstat(clean)
	if errors.Is(err, os.ErrNotExist) {
		t.snapshots = append(t.snapshots, snapshot{path: clean})
		return nil
	}
	if err != nil {
		return err
	}
	if !managed[clean] && !force {
		return fmt.Errorf("refusing unmanaged collision at %s (use --force to replace)", clean)
	}
	backup, err := os.MkdirTemp(filepath.Dir(clean), ".prism-rollback-")
	if err != nil {
		return err
	}
	backupPath := filepath.Join(backup, "item")
	if err := os.Rename(clean, backupPath); err != nil {
		return err
	}
	t.snapshots = append(t.snapshots, snapshot{path: clean, backup: backupPath, existed: true})
	return nil
}

func (t *transaction) writeFile(path string, data []byte, mode fs.FileMode, managed map[string]bool, force bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	stage, err := os.CreateTemp(filepath.Dir(path), ".prism-stage-")
	if err != nil {
		return err
	}
	stagePath := stage.Name()
	defer func() { _ = os.Remove(stagePath) }()
	if err := stage.Chmod(mode); err != nil {
		_ = stage.Close()
		return err
	}
	if _, err := stage.Write(data); err != nil {
		_ = stage.Close()
		return err
	}
	if err := stage.Close(); err != nil {
		return err
	}
	if err := t.prepare(path, managed, force); err != nil {
		return err
	}
	return os.Rename(stagePath, path)
}

func (t *transaction) writeConfig(path string, data []byte, managed map[string]bool, force bool) error {
	if existing, err := os.ReadFile(path); err == nil {
		backupManaged := map[string]bool{filepath.Clean(path + ".prism-backup"): true}
		if err := t.writeFile(path+".prism-backup", existing, 0o600, backupManaged, true); err != nil {
			return err
		}
		managed[filepath.Clean(path)] = true // config merging is safe even when pre-existing
	}
	return t.writeFile(path, data, 0o644, managed, force)
}

func (t *transaction) remove(path string) error {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return t.prepare(path, map[string]bool{filepath.Clean(path): true}, true)
}

func (t *transaction) rollback() {
	if t.done {
		return
	}
	for i := len(t.snapshots) - 1; i >= 0; i-- {
		s := t.snapshots[i]
		_ = os.RemoveAll(s.path)
		if s.existed {
			_ = os.Rename(s.backup, s.path)
		}
	}
}

func (t *transaction) commit() {
	for _, s := range t.snapshots {
		if s.backup != "" {
			_ = os.RemoveAll(filepath.Dir(s.backup))
		}
	}
	t.done = true
}

func scopeRoot(opts Options) (string, error) {
	if opts.Root != "" {
		return filepath.Abs(opts.Root)
	}
	if opts.Scope == Global {
		return os.UserHomeDir()
	}
	return os.Getwd()
}

func selectNames(selected, available []string, kind string) ([]string, error) {
	allowed := map[string]bool{}
	for _, item := range available {
		allowed[item] = true
	}
	selected = unique(selected)
	for _, item := range selected {
		if !allowed[item] {
			return nil, fmt.Errorf("unknown %s %q", kind, item)
		}
	}
	sort.Strings(selected)
	return selected, nil
}

func unique(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func entryFor(path, kind string) Entry {
	data, _ := os.ReadFile(path)
	return entryForBytes(path, kind, data)
}

func entryForBytes(path, kind string, data []byte) Entry {
	sum := sha256.Sum256(data)
	return Entry{Path: path, Kind: kind, Digest: hex.EncodeToString(sum[:])}
}

func replaceBlock(content, begin, end, replacement string) string {
	if start := strings.Index(content, begin); start >= 0 {
		if finish := strings.Index(content[start:], end); finish >= 0 {
			finish = start + finish + len(end)
			for finish < len(content) && content[finish] == '\n' {
				finish++
			}
			return content[:start] + replacement + content[finish:]
		}
	}
	if replacement == "" {
		return content
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + replacement
}

func stripJSONComments(data []byte) []byte {
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		inString := false
		escaped := false
		for j := 0; j+1 < len(line); j++ {
			if line[j] == '\\' && inString {
				escaped = !escaped
				continue
			}
			if line[j] == '"' && !escaped {
				inString = !inString
			}
			escaped = false
			if !inString && line[j:j+2] == "//" {
				lines[i] = line[:j]
				break
			}
		}
	}
	clean := strings.Join(lines, "\n")
	return []byte(regexp.MustCompile(`,\s*([}\]])`).ReplaceAllString(clean, "$1"))
}

func yamlQuote(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}
