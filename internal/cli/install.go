package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	prismbundle "github.com/bryanbarton525/prism"
	agentpkg "github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/agent/importconfig"
	"github.com/bryanbarton525/prism/internal/agent/importer"
	"github.com/bryanbarton525/prism/internal/downstreammcp"
	"github.com/bryanbarton525/prism/internal/extensions"
	"github.com/bryanbarton525/prism/internal/extensions/resolver"
	internalgithub "github.com/bryanbarton525/prism/internal/github"
	"github.com/bryanbarton525/prism/internal/graphify"
	"github.com/bryanbarton525/prism/internal/installer"
)

type installFlags struct {
	project                    bool
	global                     bool
	runtimeOnly                bool
	runtimeScope               string
	runtimeSkillSource         string
	runtimeSkillAll            bool
	runtimeSkillNames          []string
	runtimeAgentSource         string
	runtimeAgentExisting       string
	runtimeAgentCopy           string
	runtimeAgentAs             string
	runtimeAgentName           string
	runtimeAgentModel          string
	runtimeAgentFormat         string
	runtimeAgentRef            string
	runtimeAgentPath           string
	runtimeAgentSelect         string
	runtimeImportConfig        string
	runtimeAgentSkills         []string
	runtimeAgentSkillsSet      bool
	runtimeMCPMode             string
	runtimeMCPServers          []string
	runtimeMCPSet              bool
	runtimeDecision            *importconfig.AgentDecision
	runtimeReplace             bool
	targets                    []string
	skills                     []string
	specialists                []string
	copyMode                   bool
	yes                        bool
	all                        bool
	dryRun                     bool
	force                      bool
	graphifyManaged            bool
	graphifyKind               string
	graphifyExecutable         string
	graphifyEnvironment        string
	graphifyEnvironmentVersion string
	graphifyWorkspace          string
	graphifyIndex              string
	graphifyFingerprint        string
	graphifyServer             string
	graphifyUV                 string
	toolModel                  string
	toolRecommendAgents        []string
	primaryEngine              string
	primaryURL                 string
	primaryModel               string
	primaryAPIKeyEnv           string
	decisionService            string
	decisionURL                string
	decisionKeyEnv             string
	decisionModel              string
	kevPort                    int
	kevUV                      string
	kevDevice                  string
}

var errInstallCancelled = errors.New("setup cancelled")

// guidedInput centralizes interactive setup input so buffered input is never
// lost between questions and every selection has the same retry/EOF behavior.
type guidedInput struct {
	reader *bufio.Reader
	cmd    *cobra.Command
}

func newGuidedInput(cmd *cobra.Command) *guidedInput {
	return &guidedInput{reader: bufio.NewReader(cmd.InOrStdin()), cmd: cmd}
}

func (in *guidedInput) answer(prompt string) (string, error) {
	if ctx := in.cmd.Context(); ctx != nil && ctx.Err() != nil {
		err := ctx.Err()
		return "", err
	}
	fmt.Fprint(in.cmd.OutOrStdout(), prompt)
	answer, err := in.reader.ReadString('\n')
	if err != nil && len(answer) == 0 {
		return "", fmt.Errorf("reading setup input: %w", err)
	}
	answer = strings.TrimSpace(answer)
	if strings.EqualFold(answer, "cancel") || strings.EqualFold(answer, "quit") {
		return "", errInstallCancelled
	}
	return answer, nil
}

func newInstallCmd() *cobra.Command {
	var flags installFlags
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install embedded Prism skills, specialists, and MCP configuration into agent hosts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			flags.runtimeAgentSkillsSet = cmd.Flags().Changed("runtime-agent-skill")
			flags.runtimeMCPSet = cmd.Flags().Changed("runtime-agent-mcp-mode") || cmd.Flags().Changed("runtime-agent-mcp-server")
			if flags.runtimeMCPMode == "" && len(flags.runtimeMCPServers) > 0 {
				flags.runtimeMCPMode = extensions.MCPAccessModeCustom
			}
			return runInstall(cmd, flags)
		},
	}
	cmd.Flags().BoolVar(&flags.project, "project", false, "Install into the current project")
	cmd.Flags().BoolVar(&flags.global, "global", false, "Install into user-global host directories")
	cmd.Flags().BoolVar(&flags.runtimeOnly, "runtime-only", false, "Configure runtime extension state only (skip host installer changes)")
	cmd.Flags().StringVar(&flags.runtimeScope, "runtime-scope", "user", "Runtime extension scope: user|project")
	cmd.Flags().StringVar(&flags.runtimeSkillSource, "runtime-skill-source", "", "Explicit local skill directory to activate in runtime state")
	cmd.Flags().BoolVar(&flags.runtimeSkillAll, "runtime-skill-all", false, "Activate all skills discovered in --runtime-skill-source")
	cmd.Flags().StringSliceVar(&flags.runtimeSkillNames, "runtime-skill-name", nil, "Named skill in --runtime-skill-source to activate")
	cmd.Flags().StringVar(&flags.runtimeAgentSource, "runtime-agent-source", "", "Explicit local agent file to translate and activate in runtime state")
	cmd.Flags().StringVar(&flags.runtimeAgentExisting, "runtime-agent-existing", "", "Existing managed agent to update in the atomic runtime batch")
	cmd.Flags().StringVar(&flags.runtimeAgentCopy, "runtime-agent-copy", "", "Bundled agent ID to copy into independently managed runtime state")
	cmd.Flags().StringVar(&flags.runtimeAgentAs, "runtime-agent-as", "", "Identity for an imported or copied runtime agent")
	cmd.Flags().StringVar(&flags.runtimeAgentName, "runtime-agent-name", "", "Display name for an imported or copied runtime agent")
	cmd.Flags().StringVar(&flags.runtimeAgentModel, "runtime-agent-model", "", "Explicit local or self-hosted model used when translating an imported agent")
	cmd.Flags().StringVar(&flags.runtimeAgentFormat, "runtime-agent-format", "auto", "Agent source format: auto, prism-native, codex-toml, or claude-markdown")
	cmd.Flags().StringVar(&flags.runtimeAgentRef, "runtime-agent-ref", "", "Git branch, tag, or commit for runtime agent import")
	cmd.Flags().StringVar(&flags.runtimeAgentPath, "runtime-agent-path", "", "Repository subdirectory for runtime agent discovery")
	cmd.Flags().StringVar(&flags.runtimeAgentSelect, "runtime-agent-select", "", "Agent identity to select from a multi-agent source")
	cmd.Flags().StringVar(&flags.runtimeImportConfig, "runtime-agent-import-config", "", "Versioned import decisions for runtime agent translation")
	cmd.Flags().StringSliceVar(&flags.runtimeAgentSkills, "runtime-agent-skill", nil, "Final skill binding for the selected managed agent (repeatable)")
	cmd.Flags().StringVar(&flags.runtimeMCPMode, "runtime-agent-mcp-mode", "", "Final MCP access mode for the selected managed agent: default|custom|none")
	cmd.Flags().StringSliceVar(&flags.runtimeMCPServers, "runtime-agent-mcp-server", nil, "MCP server for runtime-agent custom access (repeatable)")
	cmd.Flags().BoolVar(&flags.runtimeReplace, "runtime-replace", false, "Replace existing managed runtime skills or agents selected by this command")
	cmd.Flags().StringSliceVar(&flags.targets, "target", nil, "Host target: codex, copilot, antigravity, claude, or opencode")
	cmd.Flags().StringSliceVar(&flags.skills, "skill", nil, "Bundled skill to install")
	cmd.Flags().StringSliceVar(&flags.specialists, "specialist", nil, "Bundled specialist wrapper to install")
	cmd.Flags().BoolVar(&flags.copyMode, "copy", false, "Copy into host directories instead of symlinking the universal installation")
	cmd.Flags().BoolVarP(&flags.yes, "yes", "y", false, "Accept the preview without prompting")
	cmd.Flags().BoolVar(&flags.all, "all", false, "Install all skills, specialists, and hosts")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "Preview changes without writing files")
	cmd.Flags().BoolVar(&flags.force, "force", false, "Replace unmanaged collisions")
	cmd.Flags().BoolVar(&flags.graphifyManaged, "graphify-managed", false, "Explicitly install and bind Prism's pinned managed Graphify environment")
	cmd.Flags().StringVar(&flags.graphifyWorkspace, "graphify-workspace", "", "Workspace for the explicitly selected Graphify binding")
	cmd.Flags().StringVar(&flags.graphifyIndex, "graphify-index", "", "Existing Graphify graph.json to bind (Prism never builds it)")
	cmd.Flags().StringVar(&flags.graphifyFingerprint, "graphify-fingerprint", "", "Current Graphify index generation fingerprint")
	cmd.Flags().StringVar(&flags.graphifyServer, "graphify-server", "graphify", "Downstream MCP name for Prism's managed Graphify endpoint")
	cmd.Flags().StringVar(&flags.graphifyUV, "graphify-uv", "uv", "uv executable for the explicitly selected managed Graphify install")
	cmd.Flags().StringVar(&flags.toolModel, "tool-model", "", "Tool recommendation model to set up: potion|onnx")
	cmd.Flags().StringSliceVar(&flags.toolRecommendAgents, "tool-recommend-agent", nil, "Agent identity to opt into tool recommendations (repeatable)")
	cmd.Flags().StringVar(&flags.primaryEngine, "primary-engine", "", "Primary model runtime: ollama|sglang|vllm")
	cmd.Flags().StringVar(&flags.primaryURL, "primary-url", "", "Primary model runtime endpoint")
	cmd.Flags().StringVar(&flags.primaryModel, "primary-model", "", "Primary model name")
	cmd.Flags().StringVar(&flags.primaryAPIKeyEnv, "primary-api-key-env", "", "Environment variable containing the primary runtime API key")
	cmd.Flags().StringVar(&flags.decisionService, "decision-service", "", "Optional decision service: local|jev|install")
	cmd.Flags().StringVar(&flags.decisionURL, "decision-url", "", "Existing Kev or Jev endpoint origin")
	cmd.Flags().StringVar(&flags.decisionKeyEnv, "decision-key-env", "", "Environment variable containing the Kev/Jev API key")
	cmd.Flags().StringVar(&flags.decisionModel, "decision-model", "", "Decision model: kev-latest|jev-latest")
	cmd.Flags().IntVar(&flags.kevPort, "kev-port", 8009, "Loopback port for explicitly installed Kev")
	cmd.Flags().StringVar(&flags.kevUV, "kev-uv", "uv", "uv executable for explicitly installed Kev")
	cmd.Flags().StringVar(&flags.kevDevice, "kev-device", "cpu", "Managed Kev device: cpu|auto")
	cmd.AddCommand(newInstallStatusCmd())
	return cmd
}

func runInstall(cmd *cobra.Command, flags installFlags) error {
	if flags.project && flags.global {
		return fmt.Errorf("--project and --global are mutually exclusive")
	}
	if err := validateRuntimeScope(flags.runtimeScope); err != nil {
		return err
	}
	runtimeStateDir, err := resolveInstallRuntimeStateDir(cmd, flags.runtimeScope)
	if err != nil {
		return err
	}
	scope := installer.Project
	if flags.global {
		scope = installer.Global
	}
	if flags.global && strings.EqualFold(strings.TrimSpace(flags.runtimeScope), "project") {
		return fmt.Errorf("--global host install is incompatible with --runtime-scope project")
	}
	if flags.runtimeOnly {
		if flags.runtimeSkillSource == "" && (flags.runtimeSkillAll || len(flags.runtimeSkillNames) > 0) {
			return fmt.Errorf("--runtime-skill-all and --runtime-skill-name require --runtime-skill-source")
		}
		if _, err := runtimeSkillPlan(flags); err != nil {
			return err
		}
		if _, err := runtimeAgentPlan(flags); err != nil {
			return err
		}
		if err := validateInstallGraphify(flags); err != nil {
			return err
		}
		if err := validateInstallToolRouting(flags); err != nil {
			return err
		}
		if flags.dryRun {
			printRuntimePlan(cmd, flags, runtimeStateDir)
			printToolRoutingPlan(cmd, flags, runtimeStateDir)
			fmt.Fprintln(cmd.OutOrStdout(), "Host installer changes are skipped. Prism never builds or refreshes Graphify indexes.")
			if err := activateRuntime(cmd, flags, runtimeStateDir); err != nil {
				return err
			}
			return runInstallGraphify(cmd, flags, runtimeStateDir, true)
		}
		if err := prepareInstallToolRouting(cmd, flags, runtimeStateDir); err != nil {
			return err
		}
		if err := activateRuntimeTransaction(cmd, flags, runtimeStateDir, true); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Initialized runtime extension state (scope=%s, state-dir=%s). Host installer changes are skipped.\n", flags.runtimeScope, runtimeStateDir)
		if err := runInstallGraphify(cmd, flags, runtimeStateDir, false); err != nil {
			return err
		}
		return saveInstallToolRouting(flags, runtimeStateDir)
	}
	skills, specialists, err := installer.Catalog()
	if err != nil {
		return err
	}
	if flags.all {
		flags.skills = skills
		flags.targets = append([]string{}, installer.Targets...)
		for _, spec := range specialists {
			flags.specialists = append(flags.specialists, spec.ID)
		}
	} else if flags.yes {
		if len(flags.skills) == 0 && len(flags.specialists) == 0 && len(flags.targets) == 0 {
			return fmt.Errorf("--yes requires explicit --skill/--specialist and --target selections, or --all")
		}
	} else if len(flags.skills) == 0 && len(flags.specialists) == 0 && len(flags.targets) == 0 {
		input := newGuidedInput(cmd)
		identity, _ := installer.BuildPlan(installer.Options{Scope: scope})
		fmt.Fprintf(cmd.OutOrStdout(), "Prism %s\nBundle SHA-256: %s\n\n", identity.Version, identity.BundleDigest)
		flags.skills, err = promptSelect(input, "Bundled skills", skills)
		if err != nil {
			return finishInstallInput(err)
		}
		agentIDs := make([]string, 0, len(specialists))
		for _, spec := range specialists {
			agentIDs = append(agentIDs, spec.ID)
		}
		flags.specialists, err = promptSelect(input, "Prism specialists", agentIDs)
		if err != nil {
			return finishInstallInput(err)
		}
		detected := detectedTargets(scope)
		if len(detected) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Detected hosts: %s\n", strings.Join(detected, ", "))
		}
		flags.targets, err = promptSelect(input, "Additional agent hosts (universal .agents/skills is always covered)", installer.Targets)
		if err != nil {
			return finishInstallInput(err)
		}
		if !flags.project && !flags.global {
			answer, err := promptChoice(input, "Scope [project/global] (project): ", "project", []string{"project", "global"})
			if err != nil {
				return finishInstallInput(err)
			}
			if answer == "global" {
				scope = installer.Global
			}
		}
		answer, err := promptChoice(input, "Mode [symlink/copy] (symlink): ", "symlink", []string{"symlink", "copy"})
		if err != nil {
			return finishInstallInput(err)
		}
		flags.copyMode = answer == "copy"
		runtimeStateDir, err = promptGuidedRuntime(input, &flags)
		if err != nil {
			return finishInstallInput(err)
		}
		if scope == installer.Global && flags.runtimeScope == "project" {
			return fmt.Errorf("--global host install is incompatible with --runtime-scope project")
		}
		if err := promptGuidedGraphify(input, &flags, runtimeStateDir); err != nil {
			return finishInstallInput(err)
		}
		if err := promptGuidedToolRouting(input, &flags); err != nil {
			return finishInstallInput(err)
		}
		return applyInstall(cmd, flags, scope, runtimeStateDir, input)
	}
	return applyInstall(cmd, flags, scope, runtimeStateDir, nil)
}

func applyInstall(cmd *cobra.Command, flags installFlags, scope installer.Scope, runtimeStateDir string, input *guidedInput) error {
	if len(flags.skills) == 0 && len(flags.specialists) == 0 {
		return fmt.Errorf("select at least one --skill or --specialist")
	}
	if len(flags.targets) == 0 {
		return fmt.Errorf("select at least one --target")
	}
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	opts := installer.Options{Scope: scope, Targets: flags.targets, Skills: flags.skills, Specialists: flags.specialists, RuntimeStateDir: runtimeStateDir, Copy: flags.copyMode, Force: flags.force, DryRun: flags.dryRun, Binary: binary}
	plan, err := installer.BuildPlan(opts)
	if err != nil {
		return err
	}
	if _, err := runtimeSkillPlan(flags); err != nil {
		return err
	}
	if _, err := runtimeAgentPlan(flags); err != nil {
		return err
	}
	if err := validateInstallGraphify(flags); err != nil {
		return err
	}
	if err := validateInstallToolRouting(flags); err != nil {
		return err
	}
	printInstallPlan(cmd, plan, flags.copyMode)
	printRuntimePlan(cmd, flags, runtimeStateDir)
	printToolRoutingPlan(cmd, flags, runtimeStateDir)
	if !flags.yes && !flags.dryRun {
		if input == nil {
			input = newGuidedInput(cmd)
		}
		answer, err := input.answer("Apply these changes? [y/N]: ")
		if err != nil {
			return finishInstallInput(err)
		}
		if !strings.EqualFold(strings.TrimSpace(answer), "y") && !strings.EqualFold(strings.TrimSpace(answer), "yes") {
			return nil
		}
	}
	if flags.dryRun {
		if err := activateRuntime(cmd, flags, runtimeStateDir); err != nil {
			return err
		}
		return runInstallGraphify(cmd, flags, runtimeStateDir, true)
	}
	if err := prepareInstallToolRouting(cmd, flags, runtimeStateDir); err != nil {
		return err
	}
	if _, err := installer.Install(opts); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Installed Prism %s (%s)\nManifest: %s\n", plan.Version, shortDigest(plan.BundleDigest), plan.ManifestPath)
	if err := activateRuntimeTransaction(cmd, flags, runtimeStateDir, false); err != nil {
		return fmt.Errorf("host installation succeeded but runtime activation failed: %w; retry runtime setup with `prism install --runtime-only --runtime-scope %s`", err, flags.runtimeScope)
	}
	if err := runInstallGraphify(cmd, flags, runtimeStateDir, false); err != nil {
		return fmt.Errorf("host/runtime installation succeeded but Graphify dependency setup failed: %w; retry with `prism graphify setup`", err)
	}
	if err := saveInstallToolRouting(flags, runtimeStateDir); err != nil {
		return fmt.Errorf("host/runtime installation succeeded but tool routing configuration failed: %w", err)
	}
	return nil
}

func promptGuidedRuntime(input *guidedInput, flags *installFlags) (string, error) {
	scope, err := promptChoice(input, "Runtime scope [user/project] (user): ", "user", []string{"user", "project"})
	if err != nil {
		return "", err
	}
	flags.runtimeScope = scope
	source, err := input.answer("Runtime skill source (local path or GitHub URL; blank to skip): ")
	if err != nil {
		return "", err
	}
	if source != "" {
		flags.runtimeSkillSource = source
		discoveryFlags := *flags
		discoveryFlags.runtimeSkillAll = true
		skills, err := runtimeSkillPlan(discoveryFlags)
		if err != nil {
			return "", err
		}
		if len(skills) > 1 {
			names := make([]string, 0, len(skills))
			for _, skill := range skills {
				names = append(names, skill.Name)
			}
			selected, err := promptSelect(input, "Runtime skills", names)
			if err != nil {
				return "", err
			}
			flags.runtimeSkillNames = selected
		} else if len(skills) == 1 {
			flags.runtimeSkillNames = []string{skills[0].Name}
		}
	}
	runtimeStateDir, err := resolveInstallRuntimeStateDir(input.cmd, flags.runtimeScope)
	if err != nil {
		return "", err
	}
	mode, err := promptChoice(input, "Runtime agent [none/import/copy/existing] (none): ", "none", []string{"none", "import", "copy", "existing"})
	if err != nil {
		return "", err
	}
	switch mode {
	case "import":
		if flags.runtimeAgentSource, err = input.answer("Agent source (local file/directory or GitHub URL): "); err != nil {
			return "", err
		}
		if flags.runtimeAgentSource == "" {
			return "", fmt.Errorf("agent import requires a source")
		}
		flags.runtimeAgentFormat, err = promptChoice(input, "Source format [auto/prism-native/codex-toml/claude-markdown] (auto): ", "auto", []string{"auto", "prism-native", "codex-toml", "claude-markdown"})
		if err != nil {
			return "", err
		}
		if internalgithub.IsURL(flags.runtimeAgentSource) {
			if flags.runtimeAgentRef, err = input.answer("Git ref (blank uses repository default): "); err != nil {
				return "", err
			}
			if flags.runtimeAgentPath, err = input.answer("Repository subdirectory (blank searches standard agent paths): "); err != nil {
				return "", err
			}
		}
		candidates, _, cleanup, discoverErr := discoverAgentCandidates(input.cmd.Context(), flags.runtimeAgentSource, flags.runtimeAgentRef, flags.runtimeAgentPath, flags.runtimeAgentFormat)
		if discoverErr != nil {
			return "", discoverErr
		}
		defer cleanup()
		if len(candidates) > 1 {
			ids := make([]string, len(candidates))
			for i := range candidates {
				ids[i] = candidates[i].ID
			}
			selected, selectErr := promptSelect(input, "Discovered agents (select one)", ids)
			if selectErr != nil {
				return "", selectErr
			}
			if len(selected) != 1 {
				return "", fmt.Errorf("select exactly one runtime agent")
			}
			flags.runtimeAgentSelect = selected[0]
		} else {
			flags.runtimeAgentSelect = candidates[0].ID
		}
		if flags.runtimeImportConfig, err = input.answer("Import decisions YAML (blank to decide unresolved fields now): "); err != nil {
			return "", err
		}
		if flags.runtimeImportConfig == "" {
			if flags.runtimeAgentModel, err = input.answer("Execution model (local or self-hosted): "); err != nil {
				return "", err
			}
			if flags.runtimeAgentModel == "" {
				return "", fmt.Errorf("agent import requires an execution model")
			}
			candidate := candidates[0]
			for _, item := range candidates {
				if strings.EqualFold(item.ID, flags.runtimeAgentSelect) {
					candidate = item
				}
			}
			_, report, translateErr := importer.TranslateWithFormat(flags.runtimeAgentFormat, candidate.Path, candidate.Data, importer.Config{DefaultModel: "prism-import-preview"})
			if translateErr != nil {
				return "", translateErr
			}
			decision := importconfig.AgentDecision{ID: candidate.ID, SourceDigest: report.SourceDigest, Adapter: report.Adapter, AdapterDigest: report.AdapterDigest, Model: flags.runtimeAgentModel}
			for _, finding := range report.Findings {
				if finding.Severity != "unresolved" {
					continue
				}
				action, choiceErr := promptChoice(input, fmt.Sprintf("Resolve %s [omit/map]: ", finding.Field), "", []string{"omit", "map"})
				if choiceErr != nil {
					return "", choiceErr
				}
				fieldDecision := importconfig.FieldDecision{Field: finding.Field, Action: action}
				if action == "omit" {
					fieldDecision.Reason, err = input.answer("Reason for accepting this omission: ")
					if err != nil {
						return "", err
					}
					if fieldDecision.Reason == "" {
						return "", fmt.Errorf("omission reason is required")
					}
				} else {
					fieldDecision.Target, err = input.answer("Mapping target (allowed_skills/context_budget/latency_budget_ms): ")
					if err != nil {
						return "", err
					}
					fieldDecision.Value, err = input.answer("Mapping value: ")
					if err != nil {
						return "", err
					}
				}
				decision.Decisions = append(decision.Decisions, fieldDecision)
			}
			flags.runtimeDecision = &decision
		}
		if flags.runtimeAgentAs, err = input.answer("Imported agent identity (blank uses source identity): "); err != nil {
			return "", err
		}
	case "copy":
		if flags.runtimeAgentCopy, err = input.answer("Bundled agent ID to copy: "); err != nil {
			return "", err
		}
		if flags.runtimeAgentCopy == "" {
			return "", fmt.Errorf("agent copy requires a bundled agent ID")
		}
		if flags.runtimeAgentAs, err = input.answer("New independent agent identity: "); err != nil {
			return "", err
		}
		if flags.runtimeAgentAs == "" {
			return "", fmt.Errorf("agent copy requires a new identity")
		}
	case "existing":
		manifest, _, loadErr := extensions.NewStore(runtimeStateDir).RecoverAndLoadManifest(input.cmd.Context())
		if loadErr != nil {
			return "", loadErr
		}
		ids := []string{}
		for _, entry := range manifest.Entries {
			if entry.Kind == "agent" {
				ids = append(ids, entry.Identity)
			}
		}
		if len(ids) == 0 {
			return "", fmt.Errorf("no managed runtime agents are installed in %s scope", flags.runtimeScope)
		}
		selected, selectErr := promptSelect(input, "Managed runtime agents", ids)
		if selectErr != nil {
			return "", selectErr
		}
		if len(selected) != 1 {
			return "", fmt.Errorf("select exactly one managed runtime agent")
		}
		flags.runtimeAgentExisting = selected[0]
	}
	if mode != "none" {
		if flags.runtimeAgentName, err = input.answer("Display name (blank keeps the current name): "); err != nil {
			return "", err
		}
		if manifest, _, loadErr := extensions.NewStore(runtimeStateDir).RecoverAndLoadManifest(input.cmd.Context()); loadErr == nil {
			if snapshot, composeErr := extensions.ComposeCatalog(extensions.ComposeInput{BundleFS: prismbundle.BundleFS(), Manifest: manifest, ObjectStoreRoot: extensions.NewStore(runtimeStateDir).ObjectRoot()}); composeErr == nil {
				available := []string{}
				for _, item := range snapshot.Skills {
					if item.Active {
						available = append(available, fmt.Sprintf("%s (%s)", item.ID, item.Origin))
					}
				}
				if len(available) > 0 {
					fmt.Fprintf(input.cmd.OutOrStdout(), "Available skill bindings: %s\n", strings.Join(available, ", "))
				}
			}
		}
		bindings, answerErr := input.answer("Final skill bindings (comma-separated; blank keeps defaults, 'none' grants none): ")
		if answerErr != nil {
			return "", answerErr
		}
		if bindings != "" {
			flags.runtimeAgentSkillsSet = true
			if !strings.EqualFold(bindings, "none") {
				for _, value := range strings.Split(bindings, ",") {
					if value = strings.TrimSpace(value); value != "" {
						flags.runtimeAgentSkills = append(flags.runtimeAgentSkills, value)
					}
				}
			}
		}
		flags.runtimeMCPMode, err = promptChoice(input, "MCP access [default/custom/none] (default): ", "default", []string{"default", "custom", "none"})
		if err != nil {
			return "", err
		}
		flags.runtimeMCPSet = true
		if flags.runtimeMCPMode == "custom" {
			servers, answerErr := input.answer("Allowed MCP servers (comma-separated): ")
			if answerErr != nil {
				return "", answerErr
			}
			for _, value := range strings.Split(servers, ",") {
				if value = strings.TrimSpace(value); value != "" {
					flags.runtimeMCPServers = append(flags.runtimeMCPServers, value)
				}
			}
			if len(flags.runtimeMCPServers) == 0 {
				return "", fmt.Errorf("custom MCP access requires at least one server")
			}
		}
	}
	return runtimeStateDir, nil
}

func promptGuidedGraphify(input *guidedInput, flags *installFlags, runtimeStateDir string) error {
	environment := filepath.Join(runtimeStateDir, "graphify", "environments", "v0.9.61")
	fmt.Fprintln(input.cmd.OutOrStdout(), "Optional Graphify support is bundled; executables, endpoints, and indexes remain separate and are never contacted during discovery.")
	discovery, _ := discoverGraphifyCandidates(runtimeStateDir)
	if discovery.Executable != "" {
		fmt.Fprintf(input.cmd.OutOrStdout(), "Detected user-managed executable candidate: %s (not invoked)\n", discovery.Executable)
	}
	for _, server := range discovery.Endpoints {
		fmt.Fprintf(input.cmd.OutOrStdout(), "Configured self-hosted endpoint candidate: %s (%s; not contacted)\n", server.Name, server.Transport)
	}
	kind, err := promptChoice(input, "Graphify endpoint [none/local/self-hosted/managed] (none): ", "none", []string{"none", "local", "self-hosted", "managed"})
	if err != nil {
		return err
	}
	if kind == "none" {
		return nil
	}
	flags.graphifyKind = kind
	if kind == "local" {
		defaultExecutable := discovery.Executable
		flags.graphifyExecutable, err = input.answer(fmt.Sprintf("User-managed Graphify executable (%s): ", defaultExecutable))
		if err != nil {
			return err
		}
		if flags.graphifyExecutable == "" {
			flags.graphifyExecutable = defaultExecutable
		}
		if flags.graphifyExecutable == "" {
			return fmt.Errorf("local Graphify setup requires an executable")
		}
	} else if kind == "managed" {
		fmt.Fprintf(input.cmd.OutOrStdout(), "The managed dependency operation is exactly: %s sync --frozen --no-dev --no-managed-python --project %s\n", flags.graphifyUV, environment)
		answer, answerErr := input.answer("Install that pinned Prism-managed environment? [y/N]: ")
		if answerErr != nil {
			return answerErr
		}
		if !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
			flags.graphifyKind = ""
			return nil
		}
		flags.graphifyManaged = true
		flags.graphifyEnvironment = environment
		flags.graphifyEnvironmentVersion = graphify.PinnedUpstreamVersion
	}
	if flags.graphifyServer, err = input.answer("Downstream MCP server name (graphify): "); err != nil {
		return err
	}
	if flags.graphifyServer == "" {
		flags.graphifyServer = "graphify"
	}
	if flags.graphifyWorkspace, err = input.answer("Graphify workspace (blank uses current directory): "); err != nil {
		return err
	}
	if flags.graphifyIndex, err = input.answer("Existing Graphify graph.json path: "); err != nil {
		return err
	}
	if flags.graphifyFingerprint, err = input.answer("Index generation fingerprint: "); err != nil {
		return err
	}
	if flags.graphifyIndex == "" || flags.graphifyFingerprint == "" {
		return fmt.Errorf("Graphify setup requires an existing index path and generation fingerprint")
	}
	return nil
}

type graphifyDiscovery struct {
	Executable string
	Endpoints  []downstreammcp.Server
}

// discoverGraphifyCandidates is deliberately offline: it inspects PATH and
// persisted endpoint metadata but never starts a binary or contacts a server.
func discoverGraphifyCandidates(stateDir string) (graphifyDiscovery, error) {
	discovery := graphifyDiscovery{}
	if executable, err := exec.LookPath(graphify.PinnedMCPEntypoint); err == nil {
		discovery.Executable = executable
	}
	state, err := downstreammcp.Load(filepath.Join(stateDir, "mcp-servers.yaml"))
	if err != nil {
		return discovery, err
	}
	for _, server := range state.Servers {
		if server.Transport == downstreammcp.TransportSSE || server.Transport == downstreammcp.TransportStreamableHTTP {
			discovery.Endpoints = append(discovery.Endpoints, server)
		}
	}
	return discovery, nil
}

func validateInstallGraphify(flags installFlags) error {
	if flags.graphifyKind == "" && flags.graphifyManaged {
		flags.graphifyKind = "managed"
	}
	if flags.graphifyKind == "" {
		if flags.graphifyWorkspace != "" || flags.graphifyIndex != "" || flags.graphifyFingerprint != "" {
			return fmt.Errorf("Graphify binding flags require explicit --graphify-managed selection")
		}
		return nil
	}
	if flags.graphifyKind != "local" && flags.graphifyKind != "self-hosted" && flags.graphifyKind != "managed" {
		return fmt.Errorf("invalid Graphify endpoint kind %q", flags.graphifyKind)
	}
	if strings.TrimSpace(flags.graphifyIndex) == "" || strings.TrimSpace(flags.graphifyFingerprint) == "" {
		return fmt.Errorf("Graphify setup requires --graphify-index and --graphify-fingerprint")
	}
	if strings.TrimSpace(flags.graphifyServer) == "" {
		return fmt.Errorf("--graphify-server cannot be empty")
	}
	return nil
}

func runInstallGraphify(cmd *cobra.Command, flags installFlags, runtimeStateDir string, dryRun bool) error {
	if flags.graphifyKind == "" && flags.graphifyManaged {
		flags.graphifyKind = "managed"
	}
	if flags.graphifyKind == "" {
		return nil
	}
	if err := validateInstallGraphify(flags); err != nil {
		return err
	}
	workspace := flags.graphifyWorkspace
	if workspace == "" {
		var err error
		workspace, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	originalStateDir := gf.stateDir
	gf.stateDir = runtimeStateDir
	defer func() { gf.stateDir = originalStateDir }()
	args := []string{
		"--workspace", workspace,
		"--index", flags.graphifyIndex,
		"--fingerprint", flags.graphifyFingerprint,
		"--server", flags.graphifyServer,
		"--endpoint-kind", flags.graphifyKind,
		"--approve",
	}
	if flags.graphifyExecutable != "" {
		args = append(args, "--executable", flags.graphifyExecutable)
	}
	if flags.graphifyEnvironment != "" {
		args = append(args, "--environment", flags.graphifyEnvironment)
	}
	if flags.graphifyEnvironmentVersion != "" {
		args = append(args, "--environment-version", flags.graphifyEnvironmentVersion)
	}
	if flags.graphifyManaged {
		args = append(args, "--install-managed-environment", "--uv", flags.graphifyUV)
	}
	if dryRun {
		args = append(args, "--dry-run")
	}
	setup := newGraphifySetupCmd()
	setup.SetArgs(args)
	setup.SetOut(cmd.OutOrStdout())
	setup.SetErr(cmd.ErrOrStderr())
	setup.SetIn(cmd.InOrStdin())
	return setup.ExecuteContext(cmd.Context())
}

func runtimeSkillPlan(flags installFlags) ([]extensions.DiscoveredSkill, error) {
	if flags.runtimeSkillSource == "" {
		return nil, nil
	}
	if internalgithub.IsURL(flags.runtimeSkillSource) {
		return discoverResolvedRuntimeSkills(contextOrBackground(), flags)
	}
	skills, err := extensions.DiscoverLocalSkills(flags.runtimeSkillSource, extensions.DiscoverSkillsOptions{
		All: flags.runtimeSkillAll, Names: flags.runtimeSkillNames,
	})
	if err != nil {
		return nil, fmt.Errorf("discover runtime skill source: %w", err)
	}

	return skills, nil
}

func contextOrBackground() context.Context {
	return context.Background()
}

func discoverResolvedRuntimeSkills(ctx context.Context, flags installFlags) ([]extensions.DiscoveredSkill, error) {
	result, err := resolver.Resolve(ctx, flags.runtimeSkillSource, cfg.GitHubToken, resolver.Bounds{})
	if err != nil {
		return nil, fmt.Errorf("resolve runtime skill source: %w", err)
	}
	defer result.Cleanup()
	skills, err := extensions.DiscoverSkillsFS(result.FS, "imported-skill", extensions.DiscoverSkillsOptions{
		All: flags.runtimeSkillAll, Names: flags.runtimeSkillNames,
	})
	if err != nil {
		return nil, fmt.Errorf("discover runtime skill source: %w", err)
	}
	return skills, nil
}

func printRuntimePlan(cmd *cobra.Command, flags installFlags, runtimeStateDir string) {
	skills, err := runtimeSkillPlan(flags)
	if err != nil {
		return // Validation reports the actionable error before application.
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Runtime scope: %s\nRuntime state: %s\n", flags.runtimeScope, runtimeStateDir)
	if len(skills) > 0 {
		names := make([]string, 0, len(skills))
		for _, skill := range skills {
			names = append(names, skill.Name)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Runtime skills: %s\n", strings.Join(names, ", "))
	}
	if agent, err := runtimeAgentPlan(flags); err == nil && agent != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Runtime agent: %s (translation=%s, model=%s)\n", agent.identity, agent.report.Adapter, agent.model)
		if agent.existing && flags.runtimeAgentSkillsSet {
			fmt.Fprintf(cmd.OutOrStdout(), "Runtime agent skill bindings: %s\n", strings.Join(flags.runtimeAgentSkills, ", "))
		} else if spec, parseErr := agentpkg.ParseManaged(agent.content, ""); parseErr == nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Runtime agent skill bindings: %s\n", strings.Join(spec.AllowedSkills, ", "))
		}
		if flags.runtimeMCPSet {
			fmt.Fprintf(cmd.OutOrStdout(), "Runtime agent MCP access: %s %s\n", flags.runtimeMCPMode, strings.Join(flags.runtimeMCPServers, ", "))
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "Runtime agent MCP access: unchanged")
		}
		for _, finding := range agent.report.Findings {
			fmt.Fprintf(cmd.OutOrStdout(), "Translation %s: %s\n", finding.Severity, finding.Message)
		}
	}
}

type plannedRuntimeAgent struct {
	identity           string
	content            []byte
	source             []byte
	sourcePath         string
	sourceURI          string
	revision           string
	subpath            string
	supportFiles       map[string][]byte
	importConfig       []byte
	importConfigDigest string
	model              string
	report             importer.Report
	existing           bool
}

var agentModelLine = regexp.MustCompile(`(?m)^model:\s*.*$`)

func replaceAgentModel(content []byte, model string) ([]byte, bool) {
	content = bytes.TrimSpace(content)
	if !bytes.HasPrefix(content, []byte("---")) {
		return content, false
	}
	end := bytes.Index(content[3:], []byte("\n---"))
	if end < 0 {
		return content, false
	}
	end += 3
	frontmatter := content[:end]
	if !agentModelLine.Match(frontmatter) {
		return content, false
	}
	return append(agentModelLine.ReplaceAll(frontmatter, []byte("model: "+strconv.Quote(model))), content[end:]...), true
}

func runtimeAgentPlan(flags installFlags) (*plannedRuntimeAgent, error) {
	modes := 0
	for _, value := range []string{flags.runtimeAgentSource, flags.runtimeAgentCopy, flags.runtimeAgentExisting} {
		if strings.TrimSpace(value) != "" {
			modes++
		}
	}
	if modes > 1 {
		return nil, fmt.Errorf("select exactly one of --runtime-agent-source, --runtime-agent-copy, or --runtime-agent-existing")
	}
	if flags.runtimeAgentAs != "" && flags.runtimeAgentSource == "" && flags.runtimeAgentCopy == "" {
		return nil, fmt.Errorf("--runtime-agent-as requires --runtime-agent-source or --runtime-agent-copy")
	}
	if flags.runtimeAgentModel != "" && flags.runtimeAgentSource == "" {
		return nil, fmt.Errorf("--runtime-agent-model requires --runtime-agent-source")
	}
	if modes == 0 {
		if flags.runtimeAgentSkillsSet || flags.runtimeMCPSet || flags.runtimeAgentName != "" {
			return nil, fmt.Errorf("agent skill, MCP, and display-name choices require a runtime agent selection")
		}
		return nil, nil
	}
	if flags.runtimeAgentExisting != "" {
		if err := extensions.ValidateIdentity(flags.runtimeAgentExisting); err != nil {
			return nil, err
		}
		return &plannedRuntimeAgent{identity: flags.runtimeAgentExisting, existing: true, report: importer.Report{Adapter: "managed-update"}}, nil
	}
	if flags.runtimeAgentCopy != "" {
		if flags.runtimeAgentAs == "" {
			return nil, fmt.Errorf("--runtime-agent-copy requires --runtime-agent-as so the copied agent has a unique identity")
		}
		if err := extensions.ValidateIdentity(flags.runtimeAgentAs); err != nil {
			return nil, err
		}
		content, err := fs.ReadFile(prismbundle.BundleFS(), filepath.ToSlash(filepath.Join("agents", flags.runtimeAgentCopy+".md")))
		if err != nil {
			return nil, fmt.Errorf("read bundled agent %q: %w", flags.runtimeAgentCopy, err)
		}
		return &plannedRuntimeAgent{identity: flags.runtimeAgentAs, content: content, model: "bundled", report: importer.Report{Adapter: "bundled-copy"}}, nil
	}
	candidates, sourceInfo, cleanup, err := discoverAgentCandidates(context.Background(), flags.runtimeAgentSource, flags.runtimeAgentRef, flags.runtimeAgentPath, flags.runtimeAgentFormat)
	if err != nil {
		return nil, fmt.Errorf("discover runtime agent source: %w", err)
	}
	defer cleanup()
	selected := []string{}
	if flags.runtimeAgentSelect != "" {
		selected = []string{flags.runtimeAgentSelect}
	}
	candidates, err = selectAgentCandidates(candidates, selected, false)
	if err != nil {
		return nil, err
	}
	candidate := candidates[0]
	preview, previewReport, err := importer.TranslateWithFormat(flags.runtimeAgentFormat, candidate.Path, candidate.Data, importer.Config{DefaultModel: "prism-import-preview"})
	if err != nil {
		return nil, err
	}
	_ = preview
	model := strings.TrimSpace(flags.runtimeAgentModel)
	var decision *importconfig.AgentDecision
	var configData []byte
	if flags.runtimeImportConfig != "" {
		configData, err = os.ReadFile(flags.runtimeImportConfig)
		if err != nil {
			return nil, err
		}
		parsed, parseErr := importconfig.Parse(configData)
		if parseErr != nil {
			return nil, fmt.Errorf("parse import config: %w", parseErr)
		}
		selectedDecision, selectErr := parsed.For(candidate.ID, previewReport.SourceDigest, previewReport.Adapter, previewReport.AdapterDigest)
		if selectErr != nil {
			return nil, selectErr
		}
		decision, model = &selectedDecision, strings.TrimSpace(selectedDecision.Model)
	} else if flags.runtimeDecision != nil {
		decision = flags.runtimeDecision
		model = strings.TrimSpace(decision.Model)
		configData, err = yaml.Marshal(importconfig.Config{Version: importconfig.Version, Agents: []importconfig.AgentDecision{*decision}})
		if err != nil {
			return nil, err
		}
	}
	if model == "" {
		return nil, fmt.Errorf("--runtime-agent-source requires --runtime-agent-model or --runtime-agent-import-config")
	}
	translationConfig, err := importerConfigFromDecision(model, previewReport, decision)
	if err != nil {
		return nil, err
	}
	content, report, err := importer.TranslateWithFormat(flags.runtimeAgentFormat, candidate.Path, candidate.Data, translationConfig)
	if err != nil {
		return nil, fmt.Errorf("translate runtime agent source: %w", err)
	}
	spec, err := agentpkg.ParseManaged(content, "")
	if err != nil {
		return nil, err
	}
	if flags.runtimeAgentName != "" {
		spec.Name = strings.TrimSpace(flags.runtimeAgentName)
	}
	if flags.runtimeAgentSkillsSet {
		spec.AllowedSkills = append([]string{}, flags.runtimeAgentSkills...)
	}
	content, err = agentpkg.Render(spec)
	if err != nil {
		return nil, err
	}
	identity := flags.runtimeAgentAs
	if identity == "" {
		identity = spec.ID
	}
	if err := extensions.ValidateIdentity(identity); err != nil {
		return nil, err
	}

	if configured := strings.TrimSpace(cfg.ModelRuntime.Primary.Model); configured != "" && configured != model {
		return nil, fmt.Errorf("runtime agent model %q conflicts with global runtime model override %q", model, configured)
	}
	support := map[string][]byte{}
	for name, data := range candidate.SupportFiles {
		support[name] = append([]byte(nil), data...)
	}
	configDigest := ""
	if len(configData) > 0 {
		configDigest = importconfig.Digest(configData)
	}
	return &plannedRuntimeAgent{identity: identity, content: content, source: append([]byte(nil), candidate.Data...), sourcePath: candidate.Path, sourceURI: sourceInfo.CanonicalSource, revision: sourceInfo.ResolvedRevision, subpath: candidate.Path, supportFiles: support, importConfig: configData, importConfigDigest: configDigest, model: model, report: report}, nil
}

func activateRuntime(cmd *cobra.Command, flags installFlags, runtimeStateDir string) error {
	objects := []extensions.RuntimeBatchObject{}
	skillCount := 0
	if flags.runtimeSkillSource != "" {
		if internalgithub.IsURL(flags.runtimeSkillSource) {
			result, resolveErr := resolver.Resolve(cmd.Context(), flags.runtimeSkillSource, cfg.GitHubToken, resolver.Bounds{})
			if resolveErr != nil {
				return fmt.Errorf("resolve runtime skill source: %w", resolveErr)
			}

			defer result.Cleanup()
			discovered, err := extensions.DiscoverSkillsFS(result.FS, "imported-skill", extensions.DiscoverSkillsOptions{
				All: flags.runtimeSkillAll, Names: flags.runtimeSkillNames,
			})
			if err != nil {
				return fmt.Errorf("activate runtime skill source: %w", err)
			}
			for _, selected := range discovered {
				object, err := extensions.PlanSkillObject(result.FS, selected.Path, selected.Name, result.CanonicalSource, result.ResolvedRevision, selected.Path, result.Digest, flags.runtimeReplace)
				if err != nil {
					return fmt.Errorf("plan runtime skill %q: %w", selected.Name, err)
				}
				objects = append(objects, object)
				skillCount++
			}
		} else {
			discovered, err := extensions.DiscoverLocalSkills(flags.runtimeSkillSource, extensions.DiscoverSkillsOptions{All: flags.runtimeSkillAll, Names: flags.runtimeSkillNames})
			if err != nil {
				return fmt.Errorf("activate runtime skill source: %w", err)
			}
			for _, selected := range discovered {
				object, err := extensions.PlanSkillObject(os.DirFS(selected.Path), ".", selected.Name, "local", "", filepath.Base(selected.Path), "", flags.runtimeReplace)
				if err != nil {
					return fmt.Errorf("plan runtime skill %q: %w", selected.Name, err)
				}
				objects = append(objects, object)
				skillCount++
			}
		}
	}
	planned, err := runtimeAgentPlan(flags)
	if err != nil {
		return err
	}
	if planned != nil {
		var object extensions.RuntimeBatchObject
		proposed := map[string]bool{}
		for _, candidate := range objects {
			if candidate.Entry.Kind == "skill" {
				proposed[strings.ToLower(candidate.Entry.Identity)] = true
			}
		}
		if planned.existing {
			bindings := []extensions.SkillBinding(nil)
			if flags.runtimeAgentSkillsSet {
				bindings, err = resolveSkillBindingOrigins(cmd.Context(), flags.runtimeAgentSkills, proposed)
				if err != nil {
					return err
				}
			}
			object, err = extensions.PlanExistingAgentUpdate(runtimeStateDir, planned.identity, flags.runtimeAgentName, flags.runtimeAgentSkills, flags.runtimeAgentSkillsSet, bindings)
		} else if planned.report.Adapter == "bundled-copy" {
			object, err = extensions.PlanBundledAgentCopyWithOptions(prismbundle.BundleFS(), prismbundle.BundleDigest(), flags.runtimeAgentCopy, planned.identity, flags.runtimeAgentName, flags.runtimeAgentSkills, flags.runtimeAgentSkillsSet)
			if err == nil && flags.runtimeAgentSkillsSet {
				object.Entry.SkillBindings, err = resolveSkillBindingOrigins(cmd.Context(), flags.runtimeAgentSkills, proposed)
			}
		} else {
			target := effectiveRuntimeTarget(planned.model)
			spec, parseErr := agentpkg.ParseManaged(planned.content, "")
			if parseErr != nil {
				return parseErr
			}
			bindings, bindingErr := resolveSkillBindingOrigins(cmd.Context(), spec.AllowedSkills, proposed)
			if bindingErr != nil {
				return bindingErr
			}
			object, err = extensions.PlanTranslatedAgentObject(extensions.TranslatedAgentInstall{Name: planned.identity + ".md", Normalized: planned.content, Source: planned.source, SourceName: planned.sourcePath, SupportFiles: planned.supportFiles, ImportConfig: planned.importConfig, Request: extensions.InstallLocalAgentRequest{As: planned.identity, Replace: flags.runtimeReplace, SourceURI: planned.sourceURI, Revision: planned.revision, Subpath: planned.subpath, Runtime: &target, Provenance: extensions.Provenance{SourceDigest: planned.report.SourceDigest, SourceModel: planned.report.SourceModel, Adapter: planned.report.Adapter, AdapterDigest: planned.report.AdapterDigest, ImportConfig: planned.importConfigDigest}, SkillBindings: bindings}})
		}
		if err != nil {
			return fmt.Errorf("plan runtime agent: %w", err)
		}
		objects = append(objects, object)
	}
	var access *extensions.MCPAccessState
	if planned != nil && flags.runtimeMCPSet {
		rule := extensions.MCPAccessRule{Mode: flags.runtimeMCPMode, Servers: flags.runtimeMCPServers}
		if err := extensions.ValidateMCPAccessRule(rule); err != nil {
			return err
		}
		if rule.Mode == extensions.MCPAccessModeCustom {
			configured, loadErr := downstreammcp.Load(filepath.Join(runtimeStateDir, "mcp-servers.yaml"))
			if loadErr != nil {
				return loadErr
			}
			for _, name := range rule.Servers {
				if _, ok := configured.Get(name); !ok {
					return fmt.Errorf("downstream MCP server %q is not configured", name)
				}
			}
		}
		state, _, loadErr := extensions.LoadMCPAccess(runtimeStateDir)
		if loadErr != nil {
			return loadErr
		}
		if state.Agents == nil {
			state.Agents = map[string]extensions.MCPAccessRule{}
		}
		state.Agents[strings.ToLower(planned.identity)] = rule
		access = &state
	}
	entries, err := extensions.ActivateRuntimeBatch(cmd.Context(), runtimeStateDir, extensions.RuntimeBatchRequest{Objects: objects, MCPAccess: access, DryRun: flags.dryRun})
	if err != nil {
		return fmt.Errorf("activate runtime batch: %w", err)
	}
	if skillCount > 0 {
		verb := "Activated"
		if flags.dryRun {
			verb = "Would activate"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %d managed runtime skill(s).\n", verb, skillCount)
	}
	if planned != nil {
		for _, entry := range entries {
			if entry.Kind == "agent" {
				verb := "Activated"
				if flags.dryRun {
					verb = "Would activate"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s managed runtime agent %s (%s).\n", verb, entry.Identity, shortDigest(entry.Digest))
			}
		}
	}
	return nil
}

func activateRuntimeTransaction(cmd *cobra.Command, flags installFlags, runtimeStateDir string, initialize bool) error {
	hasChanges := flags.runtimeSkillSource != "" || flags.runtimeAgentSource != "" || flags.runtimeAgentCopy != "" || flags.runtimeAgentExisting != "" || flags.runtimeMCPSet
	store := extensions.NewStore(runtimeStateDir)
	if !hasChanges {
		if !initialize {
			return nil
		}
		if err := store.EnsureInitialized(cmd.Context()); err != nil {
			return fmt.Errorf("initialize runtime extension state: %w", err)
		}
		return nil
	}
	previous, _, err := store.RecoverAndLoadManifest(cmd.Context())
	if err != nil {
		return fmt.Errorf("load runtime extension state: %w", err)
	}
	if err := activateRuntime(cmd, flags, runtimeStateDir); err == nil {
		return nil
	} else {
		rollbackErr := restoreRuntimeManifest(cmd.Context(), store, previous)
		if rollbackErr != nil {
			return fmt.Errorf("%w (runtime rollback failed: %v)", err, rollbackErr)
		}
		return err
	}
}

func restoreRuntimeManifest(ctx context.Context, store *extensions.Store, manifest extensions.Manifest) error {
	tx, err := store.BeginTransaction(ctx)
	if err != nil {
		return err
	}
	tx.ReplaceManifest(manifest)
	return tx.Commit()
}

func finishInstallInput(err error) error {
	if errors.Is(err, errInstallCancelled) {
		return nil
	}
	return err
}

func readInstallAnswer(reader *bufio.Reader, cmd *cobra.Command, prompt string) (string, error) {
	return (&guidedInput{reader: reader, cmd: cmd}).answer(prompt)
}

func newUninstallCmd() *cobra.Command {
	var global, project, dryRun bool
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove files recorded in Prism's install manifest",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if global && project {
				return fmt.Errorf("--project and --global are mutually exclusive")
			}
			scope := installer.Project
			if global {
				scope = installer.Global
			}
			manifest, err := installer.Uninstall(scope, "", dryRun)
			if err != nil {
				return err
			}
			verb := "Removed"
			if dryRun {
				verb = "Would remove"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %d Prism-managed paths from %s scope\n", verb, len(manifest.Entries), scope)
			return nil
		},
	}
	cmd.Flags().BoolVar(&project, "project", false, "Use the current project manifest")
	cmd.Flags().BoolVar(&global, "global", false, "Use the global manifest")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview removals")
	return cmd
}

func newInstallStatusCmd() *cobra.Command {
	var global bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the Prism-owned install manifest",
		RunE: func(cmd *cobra.Command, _ []string) error {
			scope := installer.Project
			root, err := os.Getwd()
			if global {
				scope = installer.Global
				root, err = os.UserHomeDir()
			}
			if err != nil {
				return err
			}
			path := filepath.Join(root, ".prism", "install.json")
			manifest, err := installer.LoadManifest(path)
			if errors.Is(err, os.ErrNotExist) {
				fmt.Fprintf(cmd.OutOrStdout(), "No Prism %s installation found.\n", scope)
				return nil
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Prism %s\nBundle: %s\nScope: %s\nManaged paths: %d\nManifest: %s\n", manifest.PrismVersion, manifest.BundleDigest, manifest.Scope, len(manifest.Entries), path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&global, "global", false, "Show the global installation")
	return cmd
}

func promptSelect(input *guidedInput, title string, values []string) ([]string, error) {
	fmt.Fprintf(input.cmd.OutOrStdout(), "%s:\n", title)
	for index, value := range values {
		fmt.Fprintf(input.cmd.OutOrStdout(), "  %d. %s\n", index+1, value)
	}
	fmt.Fprint(input.cmd.OutOrStdout(), "Select comma-separated numbers or 'all': ")
	for {
		answer, err := input.answer("")
		if err != nil {
			return nil, err
		}
		if strings.EqualFold(answer, "all") {
			return append([]string{}, values...), nil
		}
		var selected []string
		valid := true
		for _, raw := range strings.Split(answer, ",") {
			if strings.TrimSpace(raw) == "" {
				continue
			}
			index, err := strconv.Atoi(strings.TrimSpace(raw))
			if err != nil || index < 1 || index > len(values) {
				fmt.Fprintf(input.cmd.OutOrStdout(), "Invalid selection %q. Try again, or enter 'cancel': ", raw)
				valid = false
				break
			}
			selected = append(selected, values[index-1])
		}
		if valid {
			return selected, nil
		}
	}
}

func promptChoice(input *guidedInput, prompt, defaultValue string, choices []string) (string, error) {
	for {
		answer, err := input.answer(prompt)
		if err != nil {
			return "", err
		}
		if answer == "" {
			return defaultValue, nil
		}
		for _, choice := range choices {
			if strings.EqualFold(answer, choice) {
				return strings.ToLower(choice), nil
			}
		}
		fmt.Fprintf(input.cmd.OutOrStdout(), "Invalid choice %q. Try again, or enter 'cancel'.\n", answer)
	}
}

func printInstallPlan(cmd *cobra.Command, plan installer.Plan, copyMode bool) {
	mode := "symlink (copy fallback)"
	if copyMode {
		mode = "copy"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nPrism %s\nBundle SHA-256: %s\nScope: %s\nMode: %s\nSkills: %s\nSpecialists: %s\nHosts: %s\n\nPaths:\n", plan.Version, plan.BundleDigest, plan.Scope, mode, strings.Join(plan.Skills, ", "), strings.Join(plan.Specialists, ", "), strings.Join(plan.Targets, ", "))
	for _, path := range plan.Paths {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", path)
	}
}

func detectedTargets(scope installer.Scope) []string {
	var found []string
	home, _ := os.UserHomeDir()
	checks := map[string][]string{
		"codex":       {filepath.Join(home, ".codex")},
		"copilot":     {filepath.Join(home, ".vscode")},
		"antigravity": {filepath.Join(home, ".gemini")},
		"claude":      {filepath.Join(home, ".claude")},
		"opencode":    {filepath.Join(home, ".config", "opencode")},
	}

	if scope == installer.Project {
		cwd, _ := os.Getwd()
		checks["codex"] = append(checks["codex"], filepath.Join(cwd, ".codex"))
		checks["copilot"] = append(checks["copilot"], filepath.Join(cwd, ".github"), filepath.Join(cwd, ".vscode"))
		checks["antigravity"] = append(checks["antigravity"], filepath.Join(cwd, ".agents"))
		checks["claude"] = append(checks["claude"], filepath.Join(cwd, ".claude"))
		checks["opencode"] = append(checks["opencode"], filepath.Join(cwd, ".opencode"))
	}
	for _, target := range installer.Targets {
		for _, path := range checks[target] {
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				found = append(found, target)
				break
			}
		}
	}
	return found
}

func shortDigest(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}

func validateRuntimeScope(scope string) error {
	scope = strings.ToLower(strings.TrimSpace(scope))
	if scope != "project" && scope != "user" {
		return fmt.Errorf("runtime scope must be 'project' or 'user'")
	}
	return nil
}

func resolveInstallRuntimeStateDir(cmd *cobra.Command, scope string) (string, error) {
	explicitScope := strings.EqualFold(strings.TrimSpace(scope), "project") || flagChanged(cmd, "runtime-scope")
	explicitStateDir := flagChanged(cmd, "state-dir")
	if gf.stateDir == "" {
		if explicitStateDir {
			return "", fmt.Errorf("--state-dir cannot be empty")
		}
		return resolveRuntimeStateDir(scope)
	}
	selectedDir, err := filepath.Abs(gf.stateDir)
	if err != nil {
		return "", err
	}
	if explicitStateDir && explicitScope {
		scopedDir, err := resolveRuntimeStateDir(scope)
		if err != nil {
			return "", err
		}
		if filepath.Clean(selectedDir) != filepath.Clean(scopedDir) {
			return "", fmt.Errorf("--state-dir %q conflicts with --runtime-scope %q (expected %q)", gf.stateDir, scope, scopedDir)
		}
	}
	if explicitStateDir || !explicitScope {
		return selectedDir, nil
	}
	return resolveRuntimeStateDir(scope)
}

func resolveRuntimeStateDir(scope string) (string, error) {
	scope = strings.ToLower(strings.TrimSpace(scope))
	switch scope {
	case "project":
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Abs(filepath.Join(cwd, ".prism"))
	case "user":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Abs(filepath.Join(home, ".prism"))
	default:
		return "", fmt.Errorf("runtime scope must be 'project' or 'user'")
	}
}
