package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	prismbundle "github.com/bryanbarton525/prism"
	agentpkg "github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/agent/importconfig"
	"github.com/bryanbarton525/prism/internal/agent/importer"
	"github.com/bryanbarton525/prism/internal/app"
	"github.com/bryanbarton525/prism/internal/downstreammcp"
	"github.com/bryanbarton525/prism/internal/events"
	"github.com/bryanbarton525/prism/internal/extensions"
	extensionresolver "github.com/bryanbarton525/prism/internal/extensions/resolver"
	"github.com/bryanbarton525/prism/internal/graphify"
	internalpolicy "github.com/bryanbarton525/prism/internal/policy"
	"github.com/bryanbarton525/prism/internal/rootresolver"
	"github.com/bryanbarton525/prism/internal/skill"
	"github.com/bryanbarton525/prism/pkg/observe"
)

func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "agent",
		Aliases: []string{"agents"},
		Short:   "Inspect registered agent specifications",
	}
	cmd.AddCommand(newAgentListCmd())
	cmd.AddCommand(newAgentShowCmd())
	cmd.AddCommand(newAgentConstitutionCmd())
	cmd.AddCommand(newAgentAddCmd(), newAgentRemoveCmd(), newAgentCopyCmd(), newAgentRenameCmd(), newAgentSkillCmd(), newAgentModelCmd(), newAgentManagedCmd())
	return cmd
}

func newAgentAddCmd() *cobra.Command {
	var as string
	var model string
	var useConfiguredModel bool
	var format string
	var replace bool
	var dryRun bool
	var selectedAgents []string
	var all bool
	var listOnly bool
	var ref string
	var subpath string
	var importConfigPath string
	cmd := &cobra.Command{
		Use:   "add <source-file-or-dir>",
		Short: "Install a local managed agent into runtime extensions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			candidates, sourceInfo, cleanup, err := discoverAgentCandidates(cmd.Context(), args[0], ref, subpath, format)
			if err != nil {
				return err
			}
			defer cleanup()
			candidates, err = selectAgentCandidates(candidates, selectedAgents, all || listOnly)
			if err != nil {
				return err
			}
			if listOnly {
				if gf.jsonOut {
					return json.NewEncoder(os.Stdout).Encode(candidates)
				}
				for _, candidate := range candidates {
					fmt.Printf("%s\t%s\t%s\n", candidate.ID, candidate.Adapter, candidate.Path)
				}
				return nil
			}
			if as != "" && len(candidates) != 1 {
				return fmt.Errorf("--as requires exactly one selected agent")
			}
			if model != "" && useConfiguredModel {
				return fmt.Errorf("select exactly one of --model or --use-configured-model")
			}
			if importConfigPath != "" && (model != "" || useConfiguredModel) {
				return fmt.Errorf("--import-config is mutually exclusive with --model and --use-configured-model")
			}
			if model == "" && !useConfiguredModel && importConfigPath == "" && !listOnly {
				return fmt.Errorf("agent imports require --model, --use-configured-model, or --import-config")
			}
			var decisions importconfig.Config
			var importConfigDigest string
			var importConfigData []byte
			if importConfigPath != "" {
				data, readErr := os.ReadFile(importConfigPath)
				if readErr != nil {
					return readErr
				}
				decisions, err = importconfig.Parse(data)
				if err != nil {
					return fmt.Errorf("parse import config: %w", err)
				}
				importConfigDigest = importconfig.Digest(data)
				importConfigData = data
			}
			if useConfiguredModel {
				model = strings.TrimSpace(cfg.ModelRuntime.Primary.Model)
				if model == "" {
					return fmt.Errorf("the effective runtime has no configured model")
				}
			}
			svc := extensions.NewLocalAgentService(gf.stateDir)
			type output struct {
				Entry  extensions.ManifestEntry `json:"entry"`
				Report importer.Report          `json:"translation"`
			}
			reports := []importer.Report{}
			installs := []extensions.TranslatedAgentInstall{}
			for _, candidate := range candidates {
				selectedModel := model
				preview, previewReport, translateErr := importer.TranslateWithFormat(format, candidate.Path, candidate.Data, importer.Config{DefaultModel: "prism-import-preview"})
				if translateErr != nil {
					return translateErr
				}
				_ = preview
				var decision *importconfig.AgentDecision
				if importConfigPath != "" {
					selectedDecision, decisionErr := decisions.For(candidate.ID, previewReport.SourceDigest, previewReport.Adapter, previewReport.AdapterDigest)
					if decisionErr != nil {
						return decisionErr
					}
					decision = &selectedDecision
					selectedModel = strings.TrimSpace(selectedDecision.Model)
					if selectedModel == "" {
						return fmt.Errorf("import config for %q must select model", candidate.ID)
					}
				}
				if selectedModel == "" {
					return fmt.Errorf("agent %q requires --model, --use-configured-model, or --import-config", candidate.ID)
				}
				if configured := strings.TrimSpace(cfg.ModelRuntime.Primary.Model); configured != "" && configured != selectedModel {
					return fmt.Errorf("model %q for agent %q conflicts with global runtime model override %q", selectedModel, candidate.ID, configured)
				}
				translationConfig, decisionErr := importerConfigFromDecision(selectedModel, previewReport, decision)
				if decisionErr != nil {
					if decision == nil && dryRun && gf.jsonOut {
						skeleton := importConfigSkeleton(candidate.ID, previewReport)
						_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"translation": previewReport, "import_config_skeleton": skeleton})
					}
					return decisionErr
				}
				translated, report, translateErr := importer.TranslateWithFormat(format, candidate.Path, candidate.Data, translationConfig)
				if translateErr != nil {
					return translateErr
				}
				target := effectiveRuntimeTarget(selectedModel)
				translatedSpec, parseErr := agentpkg.ParseManaged(translated, "")
				if parseErr != nil {
					return parseErr
				}
				bindings, bindingErr := resolveSkillBindingOrigins(cmd.Context(), translatedSpec.AllowedSkills, nil)
				if bindingErr != nil {
					return bindingErr
				}
				installs = append(installs, extensions.TranslatedAgentInstall{Name: filepath.Base(candidate.Path), Normalized: translated, Source: candidate.Data, SourceName: candidate.Path, SupportFiles: candidate.SupportFiles, ImportConfig: importConfigData, Request: extensions.InstallLocalAgentRequest{
					As: as, Replace: replace, DryRun: dryRun, SourceURI: sourceInfo.CanonicalSource, Revision: sourceInfo.ResolvedRevision, Subpath: candidate.Path, Runtime: &target,
					Provenance: extensions.Provenance{SourceDigest: report.SourceDigest, SourceModel: report.SourceModel, Adapter: report.Adapter, AdapterDigest: report.AdapterDigest, ImportConfig: importConfigDigest}, SkillBindings: bindings,
				}})
				reports = append(reports, report)
			}
			entries, installErr := svc.InstallTranslatedAgents(cmd.Context(), installs)
			if installErr != nil {
				return installErr
			}
			outputs := make([]output, len(entries))
			for i := range entries {
				outputs[i] = output{entries[i], reports[i]}
			}
			if gf.jsonOut {
				data, _ := json.MarshalIndent(outputs, "", "  ")
				fmt.Println(string(data))
				return nil
			}
			for _, item := range outputs {
				action := "installed"
				if dryRun {
					action = "would install"
				}
				fmt.Printf("%s agent %s (%s)\n", action, item.Entry.Identity, item.Entry.Digest)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&as, "as", "", "Rename installed agent identity")
	cmd.Flags().StringVar(&model, "model", "", "Explicit local or self-hosted model ID")
	cmd.Flags().BoolVar(&useConfiguredModel, "use-configured-model", false, "Use the effective configured runtime model")
	cmd.Flags().StringVar(&format, "format", "auto", "Source format: auto, prism-native, codex-toml, or claude-markdown")
	cmd.Flags().BoolVar(&replace, "replace", false, "Replace existing managed agent")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without mutation")
	cmd.Flags().StringSliceVar(&selectedAgents, "agent", nil, "Import only selected agent identities (repeatable)")
	cmd.Flags().BoolVar(&all, "all", false, "Import all discovered agents")
	cmd.Flags().BoolVar(&listOnly, "list", false, "List discovered agents without importing")
	cmd.Flags().StringVar(&ref, "ref", "", "Git branch, tag, or commit to resolve")
	cmd.Flags().StringVar(&subpath, "path", "", "Repository subdirectory to search")
	cmd.Flags().StringVar(&importConfigPath, "import-config", "", "Versioned YAML import decisions")
	return cmd
}

func importerConfigFromDecision(model string, report importer.Report, decision *importconfig.AgentDecision) (importer.Config, error) {
	cfg := importer.Config{DefaultModel: model}
	if decision != nil {
		if decision.ContextBudget < 0 || decision.LatencyMS < 0 {
			return cfg, fmt.Errorf("import config budgets cannot be negative")
		}
		cfg.ContextBudget = decision.ContextBudget
		cfg.LatencyBudgetMS = decision.LatencyMS
	}
	unresolved := map[string]bool{}
	for _, finding := range report.Findings {
		if finding.Severity == "unresolved" {
			unresolved[strings.ToLower(finding.Field)] = true
		}
	}
	if len(unresolved) == 0 {
		if decision != nil && len(decision.Decisions) > 0 {
			return cfg, fmt.Errorf("import config contains decisions for fields that are no longer unresolved")
		}
		return cfg, nil
	}
	if decision == nil {
		keys := make([]string, 0, len(unresolved))
		for key := range unresolved {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return cfg, fmt.Errorf("unresolved import fields require --import-config decisions: %s", strings.Join(keys, ", "))
	}
	seen := map[string]bool{}
	for _, item := range decision.Decisions {
		field := strings.ToLower(strings.TrimSpace(item.Field))
		if !unresolved[field] {
			return cfg, fmt.Errorf("import decision field %q is not unresolved for adapter %s", item.Field, report.Adapter)
		}
		if seen[field] {
			return cfg, fmt.Errorf("duplicate import decision for field %q", item.Field)
		}
		seen[field] = true
		if item.Action == "omit" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(item.Target)) {
		case "allowed_skills":
			if field != "skills" && field != "allowed_skills" {
				return cfg, fmt.Errorf("adapter cannot map %q to allowed_skills", item.Field)
			}
			cfg.OverrideSkills = true
			for _, name := range strings.Split(item.Value, ",") {
				if name = strings.TrimSpace(name); name != "" {
					cfg.AllowedSkills = append(cfg.AllowedSkills, name)
				}
			}
		case "context_budget":
			value, err := strconv.Atoi(item.Value)
			if err != nil || value <= 0 {
				return cfg, fmt.Errorf("context_budget mapping must be a positive integer")
			}
			cfg.ContextBudget = value
		case "latency_budget_ms":
			value, err := strconv.Atoi(item.Value)
			if err != nil || value <= 0 {
				return cfg, fmt.Errorf("latency_budget_ms mapping must be a positive integer")
			}
			cfg.LatencyBudgetMS = value
		default:
			return cfg, fmt.Errorf("adapter cannot enforce mapping target %q", item.Target)
		}
	}
	missing := []string{}
	for field := range unresolved {
		if !seen[field] {
			missing = append(missing, field)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return cfg, fmt.Errorf("import config leaves unresolved fields: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}

func importConfigSkeleton(id string, report importer.Report) importconfig.Config {
	agent := importconfig.AgentDecision{ID: id, SourceDigest: report.SourceDigest, Adapter: report.Adapter, AdapterDigest: report.AdapterDigest}
	for _, finding := range report.Findings {
		if finding.Severity == "unresolved" {
			agent.Decisions = append(agent.Decisions, importconfig.FieldDecision{Field: finding.Field, Action: "omit", Reason: "REVIEW REQUIRED: describe the accepted behavior change"})
		}
	}
	return importconfig.Config{Version: importconfig.Version, Agents: []importconfig.AgentDecision{agent}}
}

type agentCandidate struct {
	ID           string            `json:"id"`
	Path         string            `json:"path"`
	Adapter      string            `json:"adapter"`
	Data         []byte            `json:"-"`
	SupportFiles map[string][]byte `json:"-"`
}

func discoverAgentCandidates(ctx context.Context, source, ref, subpath, format string) ([]agentCandidate, extensionresolver.Result, func(), error) {
	sourceInfo, sourceStatErr := os.Stat(source)
	localDirectory := sourceStatErr == nil && sourceInfo.IsDir()
	if sourceStatErr == nil && !sourceInfo.IsDir() {
		data, readErr := os.ReadFile(source)
		if readErr != nil {
			return nil, extensionresolver.Result{}, func() {}, readErr
		}
		candidate, candidateErr := identifyAgentCandidate(filepath.Base(source), data, format)
		if candidateErr != nil {
			return nil, extensionresolver.Result{}, func() {}, candidateErr
		}
		abs, _ := filepath.Abs(source)
		packageRoot := filepath.Dir(abs)
		if strings.EqualFold(filepath.Base(packageRoot), "agents") {
			packageRoot = filepath.Dir(packageRoot)
		}
		if supportErr := attachAgentSupportFiles(os.DirFS(packageRoot), &candidate); supportErr != nil {
			return nil, extensionresolver.Result{}, func() {}, supportErr
		}
		return []agentCandidate{candidate}, extensionresolver.Result{CanonicalSource: abs}, func() {}, nil
	}
	if strings.Contains(source, "github.com/") && strings.Contains(source, "/tree/") && ref == "" {
		after := strings.SplitN(source, "/tree/", 2)[1]
		if strings.Count(strings.Trim(after, "/"), "/") > 0 {
			return nil, extensionresolver.Result{}, func() {}, fmt.Errorf("GitHub tree URLs with a subpath are ambiguous; pass --ref and --path explicitly")
		}
	}
	resolved, err := extensionresolver.Resolve(ctx, normalizeGitHubSource(source, ref), cfg.GitHubToken, extensionresolver.Bounds{})
	if err != nil {
		return nil, extensionresolver.Result{}, func() {}, err
	}
	fsys := resolved.FS
	if subpath != "" {
		fsys, err = fs.Sub(fsys, path.Clean(strings.TrimSpace(subpath)))
		if err != nil {
			resolved.Cleanup()
			return nil, extensionresolver.Result{}, func() {}, err
		}
	}
	candidates := []agentCandidate{}
	err = fs.WalkDir(fsys, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		base := strings.ToLower(path.Base(name))
		ext := strings.ToLower(path.Ext(name))
		if (ext != ".md" && ext != ".markdown" && ext != ".toml") || base == "skill.md" || base == "readme.md" || base == "agents.md" {
			return nil
		}
		data, readErr := fs.ReadFile(fsys, name)
		if readErr != nil {
			return readErr
		}
		candidate, identifyErr := identifyAgentCandidate(name, data, format)
		if identifyErr != nil {
			return nil
		}
		if !localDirectory && subpath == "" && !standardRemoteAgentPath(name, candidate.Adapter) {
			return nil
		}
		if supportErr := attachAgentSupportFiles(fsys, &candidate); supportErr != nil {
			return supportErr
		}
		candidates = append(candidates, candidate)
		return nil
	})
	if err != nil {
		resolved.Cleanup()
		return nil, extensionresolver.Result{}, func() {}, err
	}
	if len(candidates) == 0 {
		resolved.Cleanup()
		return nil, extensionresolver.Result{}, func() {}, fmt.Errorf("no supported agent definitions discovered")
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ID < candidates[j].ID })
	return candidates, resolved, resolved.Cleanup, nil
}

func standardRemoteAgentPath(name, adapter string) bool {
	clean := strings.ToLower(path.Clean(name))
	switch adapter {
	case "prism-native":
		return strings.HasPrefix(clean, "agents/")
	case "claude-markdown":
		return strings.HasPrefix(clean, ".claude/agents/")
	case "codex-toml":
		return strings.HasPrefix(clean, ".codex/agents/")
	default:
		return false
	}
}

func attachAgentSupportFiles(fsys fs.FS, candidate *agentCandidate) error {
	if candidate.Adapter != "prism-native" {
		return nil
	}
	spec, err := agentpkg.ParseManaged(candidate.Data, candidate.Path)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(spec.ConstitutionPath)
	explicit := name != ""
	if !explicit && strings.TrimSpace(spec.Body) == "" {
		name = path.Join("constitutions", spec.ID+".md")
	}
	if name == "" {
		return nil
	}
	clean := path.Clean(strings.ReplaceAll(name, "\\", "/"))
	if !fs.ValidPath(clean) || clean == "." {
		return fmt.Errorf("agent %q has unsafe constitution path %q", candidate.ID, name)
	}
	data, err := fs.ReadFile(fsys, clean)
	if err != nil {
		if !explicit && errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read constitution for agent %q at %s: %w", candidate.ID, clean, err)
	}
	candidate.SupportFiles = map[string][]byte{clean: data}
	return nil
}

func identifyAgentCandidate(name string, data []byte, format string) (agentCandidate, error) {
	_, ok := importer.Detect(name, data)
	if !ok && (format == "" || format == "auto") {
		return agentCandidate{}, fmt.Errorf("no adapter matched %s", name)
	}
	translated, report, err := importer.TranslateWithFormat(format, name, data, importer.Config{DefaultModel: "prism-import-preview"})
	if err != nil {
		return agentCandidate{}, err
	}
	spec, err := agentpkg.ParseManaged(translated, "")
	if err != nil {
		return agentCandidate{}, err
	}
	return agentCandidate{ID: spec.ID, Path: name, Adapter: report.Adapter, Data: data}, nil
}

func selectAgentCandidates(candidates []agentCandidate, names []string, all bool) ([]agentCandidate, error) {
	if all {
		return candidates, nil
	}
	if len(names) == 0 {
		if len(candidates) == 1 {
			return candidates, nil
		}
		return nil, fmt.Errorf("multiple agents discovered; pass --agent, --all, or --list")
	}
	wanted := map[string]bool{}
	for _, name := range names {
		wanted[strings.ToLower(strings.TrimSpace(name))] = true
	}
	selected := []agentCandidate{}
	for _, candidate := range candidates {
		if wanted[strings.ToLower(candidate.ID)] {
			selected = append(selected, candidate)
			delete(wanted, strings.ToLower(candidate.ID))
		}
	}
	if len(wanted) > 0 {
		return nil, fmt.Errorf("one or more selected agents were not discovered")
	}
	return selected, nil
}

func resolveSkillBindingOrigins(ctx context.Context, names []string, proposedManaged map[string]bool) ([]extensions.SkillBinding, error) {
	if len(names) == 0 {
		return nil, nil
	}
	available := map[string]string{}
	if gf.skillsDir != "" {
		discovered, err := skill.DiscoverAll(os.DirFS(gf.skillsDir))
		if err != nil {
			return nil, err
		}
		for _, item := range discovered {
			available[strings.ToLower(item.Name)] = "development"
		}
	} else {
		store := extensions.NewStore(gf.stateDir)
		manifest, _, err := store.RecoverAndLoadManifest(ctx)
		if err != nil {
			return nil, err
		}
		snapshot, err := extensions.ComposeCatalog(extensions.ComposeInput{BundleFS: prismbundle.BundleFS(), Manifest: manifest, ObjectStoreRoot: store.ObjectRoot()})
		if err != nil {
			return nil, err
		}
		for _, item := range snapshot.Skills {
			if item.Active {
				available[strings.ToLower(item.ID)] = item.Origin
			}
		}
	}
	bindings := make([]extensions.SkillBinding, 0, len(names))
	for _, name := range names {
		key := strings.ToLower(strings.TrimSpace(name))
		origin := available[key]
		if proposedManaged != nil && proposedManaged[key] {
			origin = "managed"
		}
		if origin == "" {
			return nil, fmt.Errorf("agent skill dependency %q is unavailable; install it explicitly or omit it through the import decision", name)
		}
		bindings = append(bindings, extensions.SkillBinding{Name: name, Origin: origin})
	}
	return bindings, nil
}

func newAgentRemoveCmd() *cobra.Command { return newAgentManagedRemoveCmd() }
func newAgentRenameCmd() *cobra.Command { return newAgentManagedRenameCmd() }

func newAgentCopyCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use: "copy <bundled-agent-id> <new-agent-id>", Short: "Copy a bundled agent into managed runtime state", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			entry, err := extensions.NewLocalAgentService(gf.stateDir).CopyBundledAgent(cmd.Context(), prismbundle.BundleFS(), prismbundle.BundleDigest(), args[0], args[1], dryRun)
			if err != nil {
				return err
			}
			if gf.jsonOut {
				data, _ := json.MarshalIndent(entry, "", "  ")
				fmt.Println(string(data))
				return nil
			}
			action := "copied"
			if dryRun {
				action = "would copy"
			}
			fmt.Printf("%s bundled agent %q -> %q\n", action, args[0], args[1])
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without mutation")
	return cmd
}

func newAgentSkillCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "skill", Short: "Manage skill bindings for a managed agent"}
	cmd.AddCommand(newAgentSkillMutationCmd(true), newAgentSkillMutationCmd(false))
	return cmd
}

func newAgentSkillMutationCmd(add bool) *cobra.Command {
	verb := "remove"
	if add {
		verb = "add"
	}
	var dryRun bool
	label := "Remove"
	if add {
		label = "Add"
	}
	cmd := &cobra.Command{Use: verb + " <agent-id> <skill-name>", Short: label + " a managed-agent skill binding", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		origin := ""
		if add {
			fsys, err := configuredSkillsFSChecked(cmd.Context())
			if err != nil {
				return err
			}
			if _, err := skill.LoadDir(fsys, args[1]); err != nil {
				return fmt.Errorf("skill %q is not active: %w", args[1], err)
			}
			bindings, err := resolveSkillBindingOrigins(cmd.Context(), []string{args[1]}, nil)
			if err != nil {
				return err
			}
			origin = bindings[0].Origin
		}
		entry, err := extensions.NewLocalAgentService(gf.stateDir).UpdateManagedAgentSkillBinding(cmd.Context(), args[0], args[1], origin, add, dryRun)
		if err != nil {
			return err
		}
		if gf.jsonOut {
			data, _ := json.MarshalIndent(entry, "", "  ")
			fmt.Println(string(data))
			return nil
		}
		fmt.Printf("%s skill %q %s agent %q\n", map[bool]string{true: "added", false: "removed"}[add], args[1], map[bool]string{true: "to", false: "from"}[add], args[0])
		return nil
	}}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without mutation")
	return cmd
}

func newAgentModelCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "model", Short: "Manage a managed agent runtime target"}
	var model string
	var useConfigured, dryRun bool
	set := &cobra.Command{Use: "set <agent-id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if (model == "") == !useConfigured {
			return fmt.Errorf("select exactly one of --model or --use-configured-model")
		}
		if useConfigured {
			model = strings.TrimSpace(cfg.ModelRuntime.Primary.Model)
			if model == "" {
				return fmt.Errorf("the effective runtime has no configured model")
			}
		} else if configured := strings.TrimSpace(cfg.ModelRuntime.Primary.Model); configured != "" && configured != model {
			return fmt.Errorf("--model %q conflicts with the global runtime model override %q", model, configured)
		}
		entry, err := extensions.NewLocalAgentService(gf.stateDir).SetManagedAgentRuntime(cmd.Context(), args[0], effectiveRuntimeTarget(model), dryRun)
		if err != nil {
			return err
		}
		if gf.jsonOut {
			data, _ := json.MarshalIndent(entry, "", "  ")
			fmt.Println(string(data))
			return nil
		}
		fmt.Printf("set agent %q model to %q\n", args[0], model)
		return nil
	}}
	set.Flags().StringVar(&model, "model", "", "Explicit local or self-hosted model ID")
	set.Flags().BoolVar(&useConfigured, "use-configured-model", false, "Use the effective configured runtime model")
	set.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without mutation")
	cmd.AddCommand(set)
	return cmd
}

func newAgentManagedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "managed",
		Short: "Manage installed runtime agents",
	}
	cmd.AddCommand(newAgentManagedListCmd(), newAgentManagedRemoveCmd(), newAgentManagedCopyCmd(), newAgentManagedRenameCmd())
	return cmd
}

func newAgentManagedListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List managed runtime agents",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc := extensions.NewLocalAgentService(gf.stateDir)
			entries, err := svc.ListManagedAgents(cmd.Context())
			if err != nil {
				return err
			}
			if gf.jsonOut {
				data, _ := json.MarshalIndent(entries, "", "  ")
				fmt.Println(string(data))
				return nil
			}
			for _, e := range entries {
				fmt.Printf("%s\t%s\t%s\n", e.Identity, e.Source, e.Digest)
			}
			return nil
		},
	}
}

func newAgentManagedRemoveCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "remove <agent-id>",
		Short: "Remove managed runtime agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := extensions.NewLocalAgentService(gf.stateDir)
			removed, err := svc.RemoveManagedAgent(cmd.Context(), args[0], dryRun)
			if err != nil {
				return err
			}
			if !removed {
				fmt.Printf("managed agent %q not found\n", args[0])
				return nil
			}
			if dryRun {
				fmt.Printf("would remove managed agent %q\n", args[0])
			} else {
				fmt.Printf("removed managed agent %q\n", args[0])
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without mutation")
	return cmd
}

func newAgentManagedCopyCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "copy <from> <to>",
		Short: "Copy managed runtime agent entry",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := extensions.NewLocalAgentService(gf.stateDir)
			ok, err := svc.CopyManagedAgent(cmd.Context(), args[0], args[1], dryRun)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Printf("managed agent %q not found\n", args[0])
				return nil
			}
			if dryRun {
				fmt.Printf("would copy managed agent %q -> %q\n", args[0], args[1])
			} else {
				fmt.Printf("copied managed agent %q -> %q\n", args[0], args[1])
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without mutation")
	return cmd
}

func newAgentManagedRenameCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "rename <from> <to>",
		Short: "Rename managed runtime agent entry",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := extensions.NewLocalAgentService(gf.stateDir)
			ok, err := svc.RenameManagedAgent(cmd.Context(), args[0], args[1], dryRun)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Printf("managed agent %q not found\n", args[0])
				return nil
			}
			if dryRun {
				fmt.Printf("would rename managed agent %q -> %q\n", args[0], args[1])
			} else {
				fmt.Printf("renamed managed agent %q -> %q\n", args[0], args[1])
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without mutation")
	return cmd
}

// ---------------------------------------------------------------------------
// prism agent list
// ---------------------------------------------------------------------------

func newAgentListCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all registered agents",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return agentList(cmd.Context(), jsonOut)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON array")
	return cmd
}

func agentList(ctx context.Context, jsonOut bool) error {
	runner, cleanup, err := newRunner(ctx)
	if err != nil {
		return err
	}
	defer cleanup()
	summaries, err := runner.ListAgents(ctx)
	if err != nil {
		return err
	}
	activeManaged := map[string]bool{}
	for _, item := range runner.CatalogSnapshot().Agents {
		if item.Origin == "managed" && item.Active {
			activeManaged[strings.ToLower(item.ID)] = true
		}
	}
	type listedAgent struct {
		agentpkg.Summary
		Origin      string                            `json:"origin"`
		Active      bool                              `json:"active"`
		Reason      string                            `json:"reason,omitempty"`
		Diagnostics []extensions.ActivationDiagnostic `json:"diagnostics,omitempty"`
	}
	listed := make([]listedAgent, 0, len(summaries))
	for _, summary := range summaries {
		origin := "bundled"
		if gf.agentDir != "" {
			origin = "development"
		}
		if activeManaged[strings.ToLower(summary.ID)] {
			origin = "managed"
		}
		listed = append(listed, listedAgent{Summary: summary, Origin: origin, Active: true})
	}
	for _, item := range runner.CatalogSnapshot().Agents {
		if item.Active {
			continue
		}
		listed = append(listed, listedAgent{Summary: agentpkg.Summary{ID: item.ID}, Origin: item.Origin, Active: false, Reason: item.Reason, Diagnostics: item.Diagnostics})
	}
	if len(summaries) == 0 {
		fmt.Fprintln(os.Stderr, "No agents found in", resolvedAgentDir())
		return nil
	}
	if jsonOut || gf.jsonOut {
		data, _ := json.MarshalIndent(listed, "", "  ")
		fmt.Println(string(data))
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tORIGIN\tSTATE\tNAME\tMODEL\tDESCRIPTION/RECOVERY")
	for _, item := range listed {
		s := item.Summary
		desc := s.Description
		if len(desc) > 72 {
			desc = desc[:69] + "..."
		}
		state := "active"
		if !item.Active {
			state = "inactive"
			if len(item.Diagnostics) > 0 {
				desc = item.Diagnostics[0].Message
			} else {
				desc = item.Reason
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", s.ID, item.Origin, state, s.Name, s.Model, desc)
	}
	return w.Flush()
}

// ---------------------------------------------------------------------------
// prism agent show <agent-id>
// ---------------------------------------------------------------------------

func newAgentShowCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "show <agent-id>",
		Short: "Show full details for one agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return agentShow(cmd.Context(), args[0], jsonOut)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	return cmd
}

func agentShow(ctx context.Context, agentID string, jsonOut bool) error {
	runner, cleanup, err := newRunner(ctx)
	if err != nil {
		return err
	}
	defer cleanup()
	spec, err := runner.GetSpec(ctx, agentID)
	if err != nil {
		for _, item := range runner.CatalogSnapshot().Agents {
			if strings.EqualFold(item.ID, agentID) && !item.Active {
				if jsonOut || gf.jsonOut {
					return json.NewEncoder(os.Stdout).Encode(item)
				}
				fmt.Printf("ID: %s\nOrigin: %s\nState: inactive\nReason: %s\n", item.ID, item.Origin, item.Reason)
				for _, diagnostic := range item.Diagnostics {
					fmt.Printf("Diagnostic: %s\n", diagnostic.Message)
				}
				return nil
			}
		}
		return err
	}
	origin := "bundled"
	if gf.agentDir != "" {
		origin = "development"
	}
	var managed *extensions.ManifestEntry
	managedActive := false
	for _, item := range runner.CatalogSnapshot().Agents {
		if strings.EqualFold(item.ID, agentID) && item.Origin == "managed" && item.Active {
			managedActive = true
			origin = "managed"
		}
	}
	if managedActive {
		entries, listErr := extensions.NewLocalAgentService(gf.stateDir).ListManagedAgents(ctx)
		if listErr == nil {
			for i := range entries {
				if strings.EqualFold(entries[i].Identity, agentID) {
					managed = &entries[i]
					break
				}
			}
		}
	}
	if jsonOut || gf.jsonOut {
		inactive := []extensions.CatalogItem{}
		for _, item := range runner.CatalogSnapshot().Agents {
			if strings.EqualFold(item.ID, agentID) && !item.Active {
				inactive = append(inactive, item)
			}
		}
		data, _ := json.MarshalIndent(map[string]any{"spec": spec, "origin": origin, "active": true, "managed_entry": managed, "inactive_variants": inactive}, "", "  ")
		fmt.Println(string(data))
		return nil
	}
	fmt.Printf("Origin:          %s\n", origin)
	fmt.Printf("State:           active\n")
	if managed != nil {
		fmt.Printf("Source:          %s\n", managed.Source)
		if managed.Revision != "" {
			fmt.Printf("Revision:        %s\n", managed.Revision)
		}
		if managed.Runtime != nil {
			fmt.Printf("Runtime target:  %s %s %s\n", managed.Runtime.Engine, managed.Runtime.BaseURL, managed.Runtime.Model)
		}
		if managed.Provenance.Adapter != "" {
			fmt.Printf("Import adapter:  %s\n", managed.Provenance.Adapter)
		}
		if managed.Provenance.SourceModel != "" {
			fmt.Printf("Source model:    %s\n", managed.Provenance.SourceModel)
		}
		if managed.Provenance.ImportConfig != "" {
			fmt.Printf("Import config:   %s\n", managed.Provenance.ImportConfig)
		}
	}
	fmt.Printf("ID:              %s\n", spec.ID)
	fmt.Printf("Name:            %s\n", spec.Name)
	fmt.Printf("Description:     %s\n", spec.Description)
	fmt.Printf("Model:           %s\n", spec.Model)
	fmt.Printf("Context budget:  %d\n", spec.ContextBudget)
	fmt.Printf("Latency budget:  %d ms\n", spec.LatencyBudgetMS)
	if spec.Temperature != nil {
		fmt.Printf("Temperature:     %.2f\n", *spec.Temperature)
	} else {
		fmt.Printf("Temperature:     (engine default)\n")
	}
	fmt.Printf("Allowed skills:  %s\n", strings.Join(spec.AllowedSkills, ", "))
	if len(spec.Tools) > 0 {
		fmt.Printf("Tools:           %s\n", strings.Join(spec.Tools, ", "))
	}
	if spec.ConstitutionPath != "" {
		fmt.Printf("Constitution:    %s\n", spec.ConstitutionPath)
	}
	if spec.Outputs != "" {
		fmt.Printf("Outputs:         %s\n", spec.Outputs)
	}
	fmt.Println("\n--- Constitution body ---")
	fmt.Println(spec.Body)
	for _, item := range runner.CatalogSnapshot().Agents {
		if strings.EqualFold(item.ID, agentID) && !item.Active {
			fmt.Printf("\nInactive %s variant: %s\n", item.Origin, item.Reason)
			for _, diagnostic := range item.Diagnostics {
				fmt.Printf("  %s\n", diagnostic.Message)
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// prism agent constitution <agent-id>
// ---------------------------------------------------------------------------

func newAgentConstitutionCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "constitution <agent-id>",
		Short: "Show the resolved constitution for an agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return agentConstitution(cmd.Context(), args[0], jsonOut)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	return cmd
}

func agentConstitution(ctx context.Context, agentID string, jsonOut bool) error {
	runner, cleanup, err := newRunner(ctx)
	if err != nil {
		return err
	}
	defer cleanup()
	c, err := runner.GetConstitution(ctx, agentID)
	if err != nil {
		return err
	}
	if jsonOut || gf.jsonOut {
		data, _ := json.MarshalIndent(c, "", "  ")
		fmt.Println(string(data))
		return nil
	}
	fmt.Printf("Agent:  %s\n", c.AgentID)
	fmt.Printf("Source: %s\n", c.Source)
	if c.Path != "" {
		fmt.Printf("Path:   %s\n", c.Path)
	}
	fmt.Println("\n--- Constitution ---")
	fmt.Println(c.Text)
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newRunner resolves gf.rootDir (local path or remote GitHub URL) to an fs.FS,
// then constructs an app.Runner. The caller must call cleanup() when finished.
// GitHubToken is read from config and passed to rootresolver so the GitHub
// Contents API is used when available; git clone is the fallback.
func newRunner(ctx context.Context) (*app.Runner, func(), error) {
	sink, closeEventSink, err := configuredEventSink()
	if err != nil {
		return nil, func() {}, err
	}
	policyEngine, err := configuredPolicyEngine()
	if err != nil {
		closeEventSink()
		return nil, func() {}, err
	}
	runner, cleanup, err := newRunnerWithControls(ctx, sink, policyEngine, true)
	if err != nil {
		closeEventSink()
		return nil, func() {}, err
	}
	return runner, func() {
		cleanup()
		closeEventSink()
	}, nil
}

func newRunnerWithControls(ctx context.Context, sink observe.Sink, policyEngine *internalpolicy.Engine, useCWD bool) (*app.Runner, func(), error) {
	workspaceRoot := gf.rootDir
	if workspaceRoot == "" && useCWD {
		var err error
		workspaceRoot, err = os.Getwd()
		if err != nil {
			return nil, func() {}, fmt.Errorf("getting current directory: %w", err)
		}
	}
	var workspaceFS fs.FS
	cleanup := func() {}
	if workspaceRoot != "" {
		var err error
		workspaceFS, cleanup, err = rootresolver.Resolve(ctx, workspaceRoot, cfg.GitHubToken)
		if err != nil {
			return nil, func() {}, fmt.Errorf("resolving workspace %q: %w", workspaceRoot, err)
		}
	}
	mcpState, err := configuredDownstreamMCPState()
	if err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("loading downstream MCP servers: %w", err)
	}
	manifest, _, err := extensions.NewStore(gf.stateDir).RecoverAndLoadManifest(ctx)
	if err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("loading runtime extension snapshot: %w", err)
	}
	mcpAccess, mcpAccessConfigured := extensions.MCPAccessState{}, manifest.MCPAccess != nil
	if manifest.MCPAccess != nil {
		mcpAccess = *manifest.MCPAccess
	} else {
		mcpAccess, mcpAccessConfigured, err = extensions.LoadMCPAccess(gf.stateDir)
		if err != nil {
			cleanup()
			return nil, func() {}, fmt.Errorf("loading MCP access policy: %w", err)
		}
	}
	graphifyConfig, err := graphify.Load(filepath.Join(gf.stateDir, "graphify.yaml"))
	if err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("loading Graphify configuration: %w", err)
	}
	modelRuntime, err := configuredModelRuntime()
	if err != nil {
		cleanup()
		return nil, func() {}, err
	}
	bundleMode := "embedded"
	bundleDigest := ""
	if gf.agentDir != "" || gf.skillsDir != "" {
		bundleMode = "development"
		embedded := prismbundle.BundleFS()
		agentsFS, _ := fs.Sub(embedded, "agents")
		skillsFS, _ := fs.Sub(embedded, "skills")
		constitutionsFS, _ := fs.Sub(embedded, "constitutions")
		if gf.agentDir != "" {
			agentsFS = os.DirFS(gf.agentDir)
			if root := configuredConstitutionFS(); root != nil {
				if overrideConstitutions, subErr := fs.Sub(root, "constitutions"); subErr == nil {
					constitutionsFS = overrideConstitutions
				}
			}
		}
		if gf.skillsDir != "" {
			skillsFS = os.DirFS(gf.skillsDir)
		}
		bundleDigest = prismbundle.DigestParts(map[string]fs.FS{"agents": agentsFS, "skills": skillsFS, "constitutions": constitutionsFS})
	}
	catalogSnapshot, err := extensions.ComposeCatalog(extensions.ComposeInput{BundleFS: prismbundle.BundleFS(), Manifest: manifest, ObjectStoreRoot: extensions.NewStore(gf.stateDir).ObjectRoot(), AgentOverride: gf.agentDir != "", SkillOverride: gf.skillsDir != ""})
	if err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("composing runtime extension snapshot: %w", err)
	}
	runner, err := app.New(app.Config{
		BundleFS:            prismbundle.BundleFS(),
		BundleDigest:        bundleDigest,
		BundleMode:          bundleMode,
		WorkspaceFS:         workspaceFS,
		WorkspaceLabel:      workspaceRoot,
		GitHubToken:         cfg.GitHubToken,
		AgentDir:            gf.agentDir,
		ConstitutionFS:      configuredConstitutionFS(),
		SkillsDir:           gf.skillsDir,
		OllamaHost:          gf.ollamaHost,
		EventSink:           sink,
		PolicyEngine:        policyEngine,
		DownstreamMCP:       downstreammcp.New(mcpState),
		ModelRuntime:        modelRuntime,
		ExtensionsStateDir:  gf.stateDir,
		ExtensionSnapshot:   &catalogSnapshot,
		MCPAccess:           mcpAccess,
		MCPAccessConfigured: mcpAccessConfigured,
		Graphify:            graphifyConfig,
		RuntimeTarget:       effectiveRuntimeTarget(""),
	})
	if err != nil {
		cleanup()
		return nil, func() {}, err
	}
	return runner, cleanup, nil
}

func effectiveRuntimeTarget(model string) extensions.RuntimeTarget {
	engine := string(cfg.ModelRuntime.Primary.Engine)
	baseURL := cfg.ModelRuntime.Primary.BaseURL
	configuredModel := cfg.ModelRuntime.Primary.Model
	if engine == "" {
		engine = "ollama"
	}
	if baseURL == "" {
		baseURL = gf.ollamaHost
	}
	if model == "" {
		model = configuredModel
	}
	return extensions.RuntimeTarget{Engine: engine, BaseURL: baseURL, Model: model}
}

func configuredConstitutionFS() fs.FS {
	if gf.agentDir == "" {
		return nil
	}
	// --agent-dir names the agents/ directory, so constitution_path retains
	// its documented sibling-relative form (constitutions/<name>.md).
	return os.DirFS(filepath.Dir(gf.agentDir))
}

func configuredEventSink() (observe.Sink, func(), error) {
	if gf.eventStore == "" {
		return nil, func() {}, nil
	}
	store, err := events.Open(gf.eventStore)
	if err != nil {
		return nil, func() {}, fmt.Errorf("opening event store: %w", err)
	}
	return store, func() { _ = store.Close() }, nil
}

func configuredPolicyEngine() (*internalpolicy.Engine, error) {
	if gf.policyFile == "" {
		return nil, nil
	}
	policyEngine, err := internalpolicy.Load(gf.policyFile)
	if err != nil {
		return nil, fmt.Errorf("loading policy: %w", err)
	}
	return policyEngine, nil
}

func resolvedAgentDir() string {
	if gf.agentDir != "" {
		return gf.agentDir
	}
	return "embedded://agents"
}
