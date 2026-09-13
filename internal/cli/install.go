package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/installer"
)

type installFlags struct {
	project      bool
	global       bool
	runtimeOnly  bool
	runtimeScope string
	targets      []string
	skills       []string
	specialists  []string
	copyMode     bool
	yes          bool
	all          bool
	dryRun       bool
	force        bool
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
	cmd.Flags().StringVar(&flags.runtimeScope, "runtime-scope", "project", "Runtime extension scope: user|project")
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
		fmt.Fprintf(cmd.OutOrStdout(), "Runtime-only setup selected (scope=%s, state-dir=%s). Host installer changes are skipped in this mode.\n", flags.runtimeScope, runtimeStateDir)
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
		reader := bufio.NewReader(cmd.InOrStdin())
		identity, _ := installer.BuildPlan(installer.Options{Scope: scope})
		fmt.Fprintf(cmd.OutOrStdout(), "Prism %s\nBundle SHA-256: %s\n\n", identity.Version, identity.BundleDigest)
		flags.skills, err = promptSelect(reader, cmd, "Bundled skills", skills)
		if err != nil {
			return err
		}
		agentIDs := make([]string, 0, len(specialists))
		for _, spec := range specialists {
			agentIDs = append(agentIDs, spec.ID)
		}
		flags.specialists, err = promptSelect(reader, cmd, "Prism specialists", agentIDs)
		if err != nil {
			return err
		}
		detected := detectedTargets(scope)
		if len(detected) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Detected hosts: %s\n", strings.Join(detected, ", "))
		}
		flags.targets, err = promptSelect(reader, cmd, "Additional agent hosts (universal .agents/skills is always covered)", installer.Targets)
		if err != nil {
			return err
		}
		if !flags.project && !flags.global {
			fmt.Fprint(cmd.OutOrStdout(), "Scope [project/global] (project): ")
			answer, _ := reader.ReadString('\n')
			if strings.EqualFold(strings.TrimSpace(answer), "global") {
				scope = installer.Global
			}
		}
		fmt.Fprint(cmd.OutOrStdout(), "Mode [symlink/copy] (symlink): ")
		answer, _ := reader.ReadString('\n')
		flags.copyMode = strings.EqualFold(strings.TrimSpace(answer), "copy")
	}
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
	opts := installer.Options{Scope: scope, Targets: flags.targets, Skills: flags.skills, Specialists: flags.specialists, Copy: flags.copyMode, Force: flags.force, DryRun: flags.dryRun, Binary: binary}
	plan, err := installer.BuildPlan(opts)
	if err != nil {
		return err
	}
	printInstallPlan(cmd, plan, flags.copyMode)
	if !flags.yes && !flags.dryRun {
		fmt.Fprint(cmd.OutOrStdout(), "Apply these changes? [y/N]: ")
		answer, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
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
	return nil
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

func promptSelect(reader *bufio.Reader, cmd *cobra.Command, title string, values []string) ([]string, error) {
	fmt.Fprintf(cmd.OutOrStdout(), "%s:\n", title)
	for index, value := range values {
		fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s\n", index+1, value)
	}
	fmt.Fprint(cmd.OutOrStdout(), "Select comma-separated numbers or 'all': ")
	answer, err := reader.ReadString('\n')
	if err != nil && len(answer) == 0 {
		return nil, err
	}
	answer = strings.TrimSpace(answer)
	if strings.EqualFold(answer, "all") {
		return append([]string{}, values...), nil
	}
	var selected []string
	for _, raw := range strings.Split(answer, ",") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		index, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || index < 1 || index > len(values) {
			return nil, fmt.Errorf("invalid selection %q", raw)
		}
		selected = append(selected, values[index-1])
	}
	return selected, nil
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
