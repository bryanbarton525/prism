package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	prismbundle "github.com/bryanbarton525/prism"
	agentpkg "github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/agent/importer"
	"github.com/bryanbarton525/prism/internal/extensions"
	"github.com/bryanbarton525/prism/internal/extensions/resolver"
	internalgithub "github.com/bryanbarton525/prism/internal/github"
	"github.com/bryanbarton525/prism/internal/installer"
)

type installFlags struct {
	project            bool
	global             bool
	runtimeOnly        bool
	runtimeScope       string
	runtimeSkillSource string
	runtimeSkillAll    bool
	runtimeSkillNames  []string
	runtimeAgentSource string
	runtimeAgentCopy   string
	runtimeAgentAs     string
	runtimeAgentModel  string
	runtimeReplace     bool
	targets            []string
	skills             []string
	specialists        []string
	copyMode           bool
	yes                bool
	all                bool
	dryRun             bool
	force              bool
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
	cmd.Flags().StringVar(&flags.runtimeAgentCopy, "runtime-agent-copy", "", "Bundled agent ID to copy into independently managed runtime state")
	cmd.Flags().StringVar(&flags.runtimeAgentAs, "runtime-agent-as", "", "Identity for an imported or copied runtime agent")
	cmd.Flags().StringVar(&flags.runtimeAgentModel, "runtime-agent-model", "", "Explicit local or self-hosted model used when translating an imported agent")
	cmd.Flags().BoolVar(&flags.runtimeReplace, "runtime-replace", false, "Replace existing managed runtime skills or agents selected by this command")
	cmd.Flags().StringSliceVar(&flags.targets, "target", nil, "Host target: codex, copilot, antigravity, claude, or opencode")
	cmd.Flags().StringSliceVar(&flags.skills, "skill", nil, "Bundled skill to install")
	cmd.Flags().StringSliceVar(&flags.specialists, "specialist", nil, "Bundled specialist wrapper to install")
	cmd.Flags().BoolVar(&flags.copyMode, "copy", false, "Copy into host directories instead of symlinking the universal installation")
	cmd.Flags().BoolVarP(&flags.yes, "yes", "y", false, "Accept the preview without prompting")
	cmd.Flags().BoolVar(&flags.all, "all", false, "Install all skills, specialists, and hosts")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "Preview changes without writing files")
	cmd.Flags().BoolVar(&flags.force, "force", false, "Replace unmanaged collisions")
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
	runtimeStateDir, err := resolveRuntimeStateDir(flags.runtimeScope)
	if err != nil {
		return err
	}
	if cmd.Flags().Changed("state-dir") {
		explicitStateDir, err := filepath.Abs(gf.stateDir)
		if err != nil {
			return err
		}
		if filepath.Clean(explicitStateDir) != filepath.Clean(runtimeStateDir) {
			return fmt.Errorf("--state-dir %q conflicts with --runtime-scope %q (expected %q)", gf.stateDir, flags.runtimeScope, runtimeStateDir)
		}
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
		if flags.dryRun {
			printRuntimePlan(cmd, flags, runtimeStateDir)
			fmt.Fprintln(cmd.OutOrStdout(), "Host installer changes are skipped. Graphify executables, endpoints, and indexes are never installed or built by this command.")
			return nil
		}
		if err := activateRuntimeTransaction(cmd, flags, runtimeStateDir, true); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Initialized runtime extension state (scope=%s, state-dir=%s). Host installer changes are skipped. Graphify executables, endpoints, bindings, and indexes were not changed.\n", flags.runtimeScope, runtimeStateDir)
		return nil
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
		if len(flags.skills) == 0 {
			flags.skills = skills
		}
		if len(flags.specialists) == 0 {
			for _, spec := range specialists {
				flags.specialists = append(flags.specialists, spec.ID)
			}
		}
		if len(flags.targets) == 0 {
			flags.targets = detectedTargets(scope)
			if len(flags.targets) == 0 {
				flags.targets = []string{"codex"}
			}
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
	printInstallPlan(cmd, plan, flags.copyMode)
	printRuntimePlan(cmd, flags, runtimeStateDir)
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
		return nil
	}
	if _, err := installer.Install(opts); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Installed Prism %s (%s)\nManifest: %s\n", plan.Version, shortDigest(plan.BundleDigest), plan.ManifestPath)
	if err := activateRuntimeTransaction(cmd, flags, runtimeStateDir, false); err != nil {
		return fmt.Errorf("host installation succeeded but runtime activation failed: %w; retry runtime setup with `prism install --runtime-only --runtime-scope %s`", err, flags.runtimeScope)
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
		skills, err := runtimeSkillPlan(*flags)
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
		}
	}
	mode, err := promptChoice(input, "Runtime agent [none/import/copy] (none): ", "none", []string{"none", "import", "copy"})
	if err != nil {
		return "", err
	}
	switch mode {
	case "import":
		if flags.runtimeAgentSource, err = input.answer("Agent source file: "); err != nil {
			return "", err
		}
		if flags.runtimeAgentSource == "" {
			return "", fmt.Errorf("agent import requires a source file")
		}
		if flags.runtimeAgentModel, err = input.answer("Execution model (local or self-hosted): "); err != nil {
			return "", err
		}
		if flags.runtimeAgentModel == "" {
			return "", fmt.Errorf("agent import requires an execution model")
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
	}
	return resolveRuntimeStateDir(flags.runtimeScope)
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
		if spec, parseErr := agentpkg.Parse(agent.content, ""); parseErr == nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Runtime agent skill bindings: %s\n", strings.Join(spec.AllowedSkills, ", "))
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Runtime agent MCP access: unchanged (configure explicitly with `prism --state-dir %s mcp access agent set %s`)\n", runtimeStateDir, agent.identity)
		for _, finding := range agent.report.Findings {
			fmt.Fprintf(cmd.OutOrStdout(), "Translation %s: %s\n", finding.Severity, finding.Message)
		}
	}
}

type plannedRuntimeAgent struct {
	identity string
	content  []byte
	model    string
	report   importer.Report
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
	if flags.runtimeAgentSource != "" && flags.runtimeAgentCopy != "" {
		return nil, fmt.Errorf("--runtime-agent-source and --runtime-agent-copy are mutually exclusive")
	}
	if flags.runtimeAgentAs != "" && flags.runtimeAgentSource == "" && flags.runtimeAgentCopy == "" {
		return nil, fmt.Errorf("--runtime-agent-as requires --runtime-agent-source or --runtime-agent-copy")
	}
	if flags.runtimeAgentModel != "" && flags.runtimeAgentSource == "" {
		return nil, fmt.Errorf("--runtime-agent-model requires --runtime-agent-source")
	}
	if flags.runtimeAgentSource == "" && flags.runtimeAgentCopy == "" {
		return nil, nil
	}
	if flags.runtimeAgentCopy != "" {
		if flags.runtimeAgentAs == "" {
			return nil, fmt.Errorf("--runtime-agent-copy requires --runtime-agent-as so the copied agent has a unique identity")
		}
		content, err := fs.ReadFile(prismbundle.BundleFS(), filepath.ToSlash(filepath.Join("agents", flags.runtimeAgentCopy+".md")))
		if err != nil {
			return nil, fmt.Errorf("read bundled agent %q: %w", flags.runtimeAgentCopy, err)
		}
		return &plannedRuntimeAgent{identity: flags.runtimeAgentAs, content: content, model: "bundled", report: importer.Report{Adapter: "bundled-copy"}}, nil
	}
	if strings.TrimSpace(flags.runtimeAgentModel) == "" {
		return nil, fmt.Errorf("--runtime-agent-source requires --runtime-agent-model; source-host model settings are not execution targets")
	}
	data, err := os.ReadFile(flags.runtimeAgentSource)
	if err != nil {
		return nil, fmt.Errorf("read runtime agent source: %w", err)
	}
	content, report, err := importer.Translate(filepath.Base(flags.runtimeAgentSource), data, importer.Config{DefaultModel: flags.runtimeAgentModel})
	if err != nil {
		return nil, fmt.Errorf("translate runtime agent source: %w", err)
	}
	content, replaced := replaceAgentModel(content, flags.runtimeAgentModel)
	if !replaced {
		return nil, fmt.Errorf("translated runtime agent has no model field")
	}
	identity := flags.runtimeAgentAs
	if identity == "" {
		identity = strings.TrimSuffix(filepath.Base(flags.runtimeAgentSource), filepath.Ext(flags.runtimeAgentSource))
	}

	return &plannedRuntimeAgent{identity: identity, content: content, model: flags.runtimeAgentModel, report: report}, nil
}

func activateRuntime(cmd *cobra.Command, flags installFlags, runtimeStateDir string) error {
	if flags.runtimeSkillSource != "" {
		var entries []extensions.ManifestEntry
		var err error
		if internalgithub.IsURL(flags.runtimeSkillSource) {
			result, resolveErr := resolver.Resolve(cmd.Context(), flags.runtimeSkillSource, cfg.GitHubToken, resolver.Bounds{})
			if resolveErr != nil {
				return fmt.Errorf("resolve runtime skill source: %w", resolveErr)
			}

			defer result.Cleanup()
			entries, err = extensions.NewLocalSkillService(runtimeStateDir).InstallResolvedSkills(cmd.Context(), result.FS, "imported-skill", result.CanonicalSource, extensions.DiscoverSkillsOptions{
				All: flags.runtimeSkillAll, Names: flags.runtimeSkillNames,
			}, flags.runtimeReplace, false)
		} else {
			entries, err = extensions.NewLocalSkillService(runtimeStateDir).InstallLocalSkills(cmd.Context(), extensions.InstallLocalSkillsRequest{
				Source: flags.runtimeSkillSource,
				Discover: extensions.DiscoverSkillsOptions{
					All: flags.runtimeSkillAll, Names: flags.runtimeSkillNames,
				},
				Replace: flags.runtimeReplace,
			})
		}
		if err != nil {
			return fmt.Errorf("activate runtime skill source: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Activated %d managed runtime skill(s).\n", len(entries))
	}
	planned, err := runtimeAgentPlan(flags)
	if err != nil {
		return err
	}
	if planned == nil {
		return nil
	}
	entry, err := extensions.NewLocalAgentService(runtimeStateDir).InstallAgentContent(cmd.Context(), planned.identity+".md", planned.content, extensions.InstallLocalAgentRequest{As: planned.identity, Replace: flags.runtimeReplace})
	if err != nil {
		return fmt.Errorf("activate runtime agent: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Activated managed runtime agent %s (%s).\n", entry.Identity, shortDigest(entry.Digest))
	return nil
}

func activateRuntimeTransaction(cmd *cobra.Command, flags installFlags, runtimeStateDir string, initialize bool) error {
	hasChanges := flags.runtimeSkillSource != "" || flags.runtimeAgentSource != "" || flags.runtimeAgentCopy != ""
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
