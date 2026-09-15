package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	prismbundle "github.com/bryanbarton525/prism"
	"github.com/bryanbarton525/prism/internal/extensions"
	extensionresolver "github.com/bryanbarton525/prism/internal/extensions/resolver"
	"github.com/bryanbarton525/prism/internal/skill"
)

func newSkillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "skill",
		Aliases: []string{"skills"},
		Short:   "Lint, test, and benchmark Prism skills",
	}
	cmd.AddCommand(newSkillLintCmd())
	cmd.AddCommand(newSkillTestCmd())
	cmd.AddCommand(newSkillBenchmarkCmd())
	cmd.AddCommand(newSkillResourcesCmd())
	cmd.AddCommand(newSkillAddCmd(), newSkillManagedCmd())
	cmd.AddCommand(newSkillListCmd(), newSkillShowCmd(), newSkillRemoveCmd(), newSkillRenameCmd(), newSkillReadCmd())
	return cmd
}

func newSkillAddCmd() *cobra.Command {
	var all bool
	var names []string
	var as string
	var replace bool
	var dryRun bool
	var listOnly bool
	var ref string
	var subpath string
	cmd := &cobra.Command{
		Use:   "add <source-dir>",
		Short: "Install local skills into managed runtime state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			source, inferredSkill, err := normalizeSkillSource(args[0], ref)
			if err != nil {
				return err
			}
			if inferredSkill != "" && len(names) == 0 && !all {
				names = []string{inferredSkill}
			}
			remote := source != args[0] || strings.Contains(source, "github.com/") || subpath != ""
			if remote {
				resolved, err := extensionresolver.Resolve(cmd.Context(), source, cfg.GitHubToken, extensionresolver.Bounds{})
				if err != nil {
					return err
				}
				defer resolved.Cleanup()
				fsys := resolved.FS
				if subpath != "" {
					fsys, err = fs.Sub(fsys, path.Clean(strings.TrimSpace(subpath)))
					if err != nil {
						return fmt.Errorf("resolve --path: %w", err)
					}
				}
				discovered, err := extensions.DiscoverSkillsFS(fsys, path.Base(strings.TrimSuffix(source, "/")), extensions.DiscoverSkillsOptions{All: all || listOnly, Names: names})
				if err != nil {
					return err
				}
				if listOnly {
					if gf.jsonOut {
						return json.NewEncoder(os.Stdout).Encode(discovered)
					}
					for _, item := range discovered {
						fmt.Printf("%s\t%s\n", item.Name, item.Path)
					}
					return nil
				}
				if as != "" {
					return fmt.Errorf("--as is supported only for a single local skill source")
				}
				entries, err := extensions.NewLocalSkillService(gf.stateDir).InstallResolvedSkillsWithProvenance(cmd.Context(), fsys, path.Base(strings.TrimSuffix(source, "/")), resolved.CanonicalSource, resolved.ResolvedRevision, subpath, resolved.Digest, extensions.DiscoverSkillsOptions{All: all, Names: names}, replace, dryRun)
				if err != nil {
					return err
				}
				for _, entry := range entries {
					action := "installed"
					if dryRun {
						action = "would install"
					}
					fmt.Printf("%s skill %s (%s)\n", action, entry.Identity, entry.Digest)
				}
				return nil
			}
			if listOnly {
				discovered, err := extensions.DiscoverLocalSkills(args[0], extensions.DiscoverSkillsOptions{All: true})
				if err != nil {
					return err
				}
				if gf.jsonOut {
					return json.NewEncoder(os.Stdout).Encode(discovered)
				}
				for _, item := range discovered {
					fmt.Printf("%s\t%s\n", item.Name, item.Path)
				}
				return nil
			}
			svc := extensions.NewLocalSkillService(gf.stateDir)
			entries, err := svc.InstallLocalSkills(cmd.Context(), extensions.InstallLocalSkillsRequest{
				Source:   args[0],
				Discover: extensions.DiscoverSkillsOptions{All: all, Names: names},
				As:       as,
				Replace:  replace,
				DryRun:   dryRun,
			})
			if err != nil {
				return err
			}
			if gf.jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}
			for _, entry := range entries {
				action := "installed"
				if dryRun {
					action = "would install"
				}
				fmt.Printf("%s skill %s (%s)\n", action, entry.Identity, entry.Digest)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Install all skills discovered in source")
	cmd.Flags().StringSliceVar(&names, "name", nil, "Install only named discovered skills (repeatable)")
	cmd.Flags().StringSliceVar(&names, "skill", nil, "Install only named discovered skills (repeatable)")
	cmd.Flags().BoolVar(&listOnly, "list", false, "List discovered skills without installing")
	cmd.Flags().StringVar(&ref, "ref", "", "Git branch, tag, or commit to resolve")
	cmd.Flags().StringVar(&subpath, "path", "", "Repository subdirectory to search")
	cmd.Flags().StringVar(&as, "as", "", "Rename single installed skill identity")
	cmd.Flags().BoolVar(&replace, "replace", false, "Replace existing managed skill entry")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview installation without mutating state")
	return cmd
}

func normalizeGitHubSource(source, ref string) string {
	source = strings.TrimSpace(source)
	_, localErr := os.Stat(source)
	if localErr != nil && !strings.Contains(source, "://") && !strings.HasPrefix(source, "git@") && strings.Count(source, "/") == 1 {
		source = "https://github.com/" + source
	}
	if strings.TrimSpace(ref) == "" || !strings.Contains(source, "github.com/") {
		return source
	}
	if marker := strings.Index(source, "/tree/"); marker >= 0 {
		source = source[:marker]
	}
	return strings.TrimSuffix(source, "/") + "/tree/" + strings.TrimSpace(ref)
}

func normalizeSkillSource(source, ref string) (string, string, error) {
	trimmed := strings.TrimSpace(source)
	if strings.HasPrefix(trimmed, "https://skills.sh/") || strings.HasPrefix(trimmed, "http://skills.sh/") {
		u, err := url.Parse(trimmed)
		if err != nil {
			return "", "", err
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) > 0 && parts[0] == "p" {
			return "", "", fmt.Errorf("skills.sh packs are discovery metadata; use the GitHub source for each selected skill")
		}
		if len(parts) != 3 {
			return "", "", fmt.Errorf("skills.sh skill URLs must use https://skills.sh/<owner>/<repo>/<skill>")
		}
		return normalizeGitHubSource("https://github.com/"+parts[0]+"/"+parts[1], ref), parts[2], nil
	}
	if strings.Contains(trimmed, "github.com/") && strings.Contains(trimmed, "/tree/") && ref == "" {
		after := strings.SplitN(trimmed, "/tree/", 2)[1]
		if strings.Count(strings.Trim(after, "/"), "/") > 0 {
			return "", "", fmt.Errorf("GitHub tree URLs with a subpath are ambiguous; pass --ref and --path explicitly")
		}
	}
	return normalizeGitHubSource(trimmed, ref), "", nil
}

func newSkillListCmd() *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List active bundled and managed skills", RunE: func(cmd *cobra.Command, _ []string) error {
		skills, err := skill.DiscoverAll(configuredSkillsFSContext(cmd.Context()))
		if err != nil {
			return err
		}
		snapshot, err := catalogSnapshotForCLI(cmd.Context())
		if err != nil {
			return err
		}
		managed := map[string]bool{}
		for _, entry := range snapshot.Skills {
			if entry.Active && entry.Origin == "managed" {
				managed[strings.ToLower(entry.ID)] = true
			}
		}
		type item struct {
			Name        string                            `json:"name"`
			Description string                            `json:"description,omitempty"`
			Origin      string                            `json:"origin"`
			Active      bool                              `json:"active"`
			Reason      string                            `json:"reason,omitempty"`
			Diagnostics []extensions.ActivationDiagnostic `json:"diagnostics,omitempty"`
		}
		items := make([]item, 0, len(skills))
		for _, sk := range skills {
			origin := "bundled"
			if gf.skillsDir != "" {
				origin = "development"
			}
			if managed[strings.ToLower(sk.Name)] {
				origin = "managed"
			}
			items = append(items, item{Name: sk.Name, Description: sk.Description, Origin: origin, Active: true})
		}
		for _, catalogItem := range snapshot.Skills {
			if !catalogItem.Active {
				items = append(items, item{Name: catalogItem.ID, Origin: catalogItem.Origin, Active: false, Reason: catalogItem.Reason, Diagnostics: catalogItem.Diagnostics})
			}
		}
		if gf.jsonOut {
			return json.NewEncoder(os.Stdout).Encode(items)
		}
		for _, sk := range items {
			state := "active"
			description := sk.Description
			if !sk.Active {
				state = "inactive"
				if len(sk.Diagnostics) > 0 {
					description = sk.Diagnostics[0].Message
				} else {
					description = sk.Reason
				}
			}
			fmt.Printf("%s\t%s\t%s\t%s\n", sk.Name, sk.Origin, state, description)
		}
		return nil
	}}
}

func newSkillShowCmd() *cobra.Command {
	return &cobra.Command{Use: "show <name>", Short: "Show one active skill", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		snapshot, snapshotErr := catalogSnapshotForCLI(cmd.Context())
		if snapshotErr != nil {
			return snapshotErr
		}
		sk, err := skill.LoadDir(configuredSkillsFSContext(cmd.Context()), args[0])
		if err != nil {
			for _, item := range snapshot.Skills {
				if strings.EqualFold(item.ID, args[0]) && !item.Active {
					if gf.jsonOut {
						return json.NewEncoder(os.Stdout).Encode(item)
					}
					fmt.Printf("Name: %s\nOrigin: %s\nState: inactive\nReason: %s\n", item.ID, item.Origin, item.Reason)
					for _, diagnostic := range item.Diagnostics {
						fmt.Printf("Diagnostic: %s\n", diagnostic.Message)
					}
					return nil
				}
			}
			return err
		}
		origin := "bundled"
		if gf.skillsDir != "" {
			origin = "development"
		}
		var managed *extensions.ManifestEntry
		managedActive := false
		for _, item := range snapshot.Skills {
			if strings.EqualFold(item.ID, args[0]) && item.Active && item.Origin == "managed" {
				managedActive = true
				origin = "managed"
			}
		}
		if managedActive {
			entries, listErr := extensions.NewLocalSkillService(gf.stateDir).ListManagedSkills(cmd.Context())
			if listErr == nil {
				for i := range entries {
					if strings.EqualFold(entries[i].Identity, args[0]) {
						managed = &entries[i]
						break
					}
				}
			}
		}
		if gf.jsonOut {
			return json.NewEncoder(os.Stdout).Encode(map[string]any{"skill": sk, "origin": origin, "active": true, "managed_entry": managed})
		}
		fmt.Printf("Origin: %s\n", origin)
		fmt.Printf("State: active\n")
		fmt.Printf("Name: %s\nDescription: %s\n", sk.Name, sk.Description)
		if sk.Compatibility != "" {
			fmt.Printf("Compatibility: %s\n", sk.Compatibility)
		}
		fmt.Printf("\n%s\n", sk.Body)
		return nil
	}}
}

func catalogSnapshotForCLI(ctx context.Context) (extensions.CatalogSnapshot, error) {
	store := extensions.NewStore(gf.stateDir)
	manifest, _, err := store.RecoverAndLoadManifest(ctx)
	if err != nil {
		return extensions.CatalogSnapshot{}, err
	}
	return extensions.ComposeCatalog(extensions.ComposeInput{BundleFS: prismbundle.BundleFS(), Manifest: manifest, ObjectStoreRoot: store.ObjectRoot(), AgentOverride: gf.agentDir != "", SkillOverride: gf.skillsDir != ""})
}

func newSkillRemoveCmd() *cobra.Command { return newSkillManagedRemoveCmd() }

func newSkillRenameCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{Use: "rename <old-name> <new-name>", Short: "Rename a managed skill and its managed-agent bindings", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		ok, err := extensions.NewLocalSkillService(gf.stateDir).RenameManagedSkill(cmd.Context(), args[0], args[1], dryRun)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("managed skill %q not found", args[0])
		}
		if gf.jsonOut {
			return json.NewEncoder(os.Stdout).Encode(map[string]any{"renamed": !dryRun, "from": args[0], "to": args[1], "dry_run": dryRun})
		}
		action := "renamed"
		if dryRun {
			action = "would rename"
		}
		fmt.Printf("%s managed skill %q -> %q\n", action, args[0], args[1])
		return nil
	}}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without mutation")
	return cmd
}

func newSkillReadCmd() *cobra.Command {
	cmd := newSkillResourcesReadCmd()
	cmd.Use = "read <skill-name> <resource-path>"
	cmd.Short = "Read a bounded skill resource"
	return cmd
}

func newSkillManagedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "managed",
		Short: "Manage installed runtime skills",
	}
	cmd.AddCommand(newSkillManagedListCmd(), newSkillManagedRemoveCmd())
	return cmd
}

func newSkillManagedListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List managed skills from extension manifest",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc := extensions.NewLocalSkillService(gf.stateDir)
			entries, err := svc.ListManagedSkills(cmd.Context())
			if err != nil {
				return err
			}
			if gf.jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}
			for _, entry := range entries {
				fmt.Printf("%s\t%s\t%s\n", entry.Identity, entry.Source, entry.Digest)
			}
			return nil
		},
	}
}

func newSkillManagedRemoveCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "remove <skill-name>",
		Short: "Remove a managed skill entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := extensions.NewLocalSkillService(gf.stateDir)
			removed, err := svc.RemoveManagedSkill(cmd.Context(), args[0], dryRun)
			if err != nil {
				return err
			}
			if !removed {
				fmt.Printf("managed skill %q not found\n", args[0])
				return nil
			}
			if dryRun {
				fmt.Printf("would remove managed skill %q\n", args[0])
			} else {
				fmt.Printf("removed managed skill %q\n", args[0])
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview removal without mutating state")
	return cmd
}

func newSkillResourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resources",
		Short: "Inspect skill resources",
	}
	cmd.AddCommand(newSkillResourcesListCmd(), newSkillResourcesReadCmd())
	return cmd
}

func newSkillResourcesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <skill-name>",
		Short: "List bounded resources for a skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			entries, err := skill.ListResources(configuredSkillsFS(), args[0])
			if err != nil {
				return err
			}
			if gf.jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}
			for _, e := range entries {
				binary := "text"
				if e.Binary {
					binary = "binary"
				}
				fmt.Printf("%s\t%d\t%s\t%s\n", e.Path, e.Size, e.MediaType, binary)
			}
			return nil
		},
	}
}

func newSkillResourcesReadCmd() *cobra.Command {
	var offset int64
	var limit int64
	var maxBytes int64
	cmd := &cobra.Command{
		Use:   "read <skill-name> <resource-path>",
		Short: "Read bounded UTF-8 resource content",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			result, err := skill.ReadResource(configuredSkillsFS(), args[0], args[1], skill.ReadResourceOptions{
				Offset:       offset,
				Limit:        limit,
				MaxReadBytes: maxBytes,
			})
			if err != nil {
				return err
			}
			if gf.jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			fmt.Printf("path: %s\n", result.Path)
			fmt.Printf("media_type: %s\n", result.MediaType)
			fmt.Printf("size: %d\n", result.Size)
			fmt.Printf("offset: %d\n", result.Offset)
			fmt.Printf("truncated: %s\n", strconv.FormatBool(result.Truncated))
			fmt.Println("---")
			fmt.Println(result.Content)
			return nil
		},
	}
	cmd.Flags().Int64Var(&offset, "offset", 0, "Byte offset to start reading")
	cmd.Flags().Int64Var(&limit, "limit", 0, "Optional max bytes to return for this read")
	cmd.Flags().Int64Var(&maxBytes, "max-bytes", 32*1024, "Hard max bytes per read")
	return cmd
}

func newSkillLintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint [skill-name]",
		Short: "Validate skill structure and metadata",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			results := lintSkills(args, strictSkillAuthoringMode())
			return printSkillResults(results)
		},
	}
}

func newSkillTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test [skill-name]",
		Short: "Run structural skill tests",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			strict := strictSkillAuthoringMode()
			results := lintSkills(args, strict)
			for i := range results {
				if !results[i].OK {
					continue
				}
				fsys := configuredSkillsFS()
				results[i].Warnings = append(results[i].Warnings, skill.ExecutionLimitations(fsys, results[i].Name)...)
				count, err := skill.ValidateEvals(fsys, results[i].Name)
				if err != nil {
					if strict {
						results[i].OK = false
						results[i].Errors = append(results[i].Errors, err.Error())
					} else {
						results[i].Warnings = append(results[i].Warnings, "eval validation skipped for portable mode: "+err.Error())
					}
					continue
				}
				results[i].Evals = count
			}
			return printSkillResults(results)
		},
	}
}

func newSkillBenchmarkCmd() *cobra.Command {
	var maxChars int
	cmd := &cobra.Command{
		Use:   "benchmark [skill-name]",
		Short: "Check skill context size budgets",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			results := lintSkills(args, strictSkillAuthoringMode())
			for i := range results {
				if results[i].Chars > maxChars {
					results[i].OK = false
					results[i].Errors = append(results[i].Errors, fmt.Sprintf("skill size %d exceeds max chars %d", results[i].Chars, maxChars))
				}
			}
			return printSkillResults(results)
		},
	}
	cmd.Flags().IntVar(&maxChars, "max-chars", 24000, "Maximum SKILL.md character count")
	return cmd
}

type skillResult struct {
	Name     string   `json:"name"`
	OK       bool     `json:"ok"`
	Chars    int      `json:"chars"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
	Evals    int      `json:"evals,omitempty"`
}

func lintSkills(args []string, strict bool) []skillResult {
	fsys := configuredSkillsFS()
	var names []string
	if len(args) == 1 {
		names = []string{args[0]}
	} else {
		entries, err := fs.ReadDir(fsys, ".")
		if err != nil {
			return []skillResult{{Name: "skills", OK: false, Errors: []string{err.Error()}}}
		}
		for _, entry := range entries {
			if entry.IsDir() {
				names = append(names, entry.Name())
			}
		}
	}
	results := make([]skillResult, 0, len(names))
	for _, name := range names {
		res := skillResult{Name: name, OK: true}
		data, err := fs.ReadFile(fsys, filepath.ToSlash(filepath.Join(name, "SKILL.md")))
		if err != nil {
			res.OK = false
			res.Errors = append(res.Errors, err.Error())
			results = append(results, res)
			continue
		}
		res.Chars = len(data)
		if _, err := skill.LoadDir(fsys, name); err != nil {
			res.OK = false
			res.Errors = append(res.Errors, err.Error())
		}
		if strict {
			if err := skill.ValidateAuthoringStructure(fsys, name); err != nil {
				res.OK = false
				res.Errors = append(res.Errors, err.Error())
			}
		} else if err := skill.ValidatePortableStructure(fsys, name); err != nil {
			res.OK = false
			res.Errors = append(res.Errors, err.Error())
		}
		res.Warnings = append(res.Warnings, skill.ExecutionLimitations(fsys, name)...)
		if !strings.Contains(string(data), "##") {
			res.Warnings = append(res.Warnings, "no markdown section headings")
		}
		results = append(results, res)
	}
	return results
}

func printSkillResults(results []skillResult) error {
	if gf.jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}
	ok := true
	for _, res := range results {
		status := "ok"
		if !res.OK {
			status = "fail"
			ok = false
		}
		if res.Evals > 0 {
			fmt.Printf("%s\t%s\t%d chars\t%d eval(s)\n", status, res.Name, res.Chars, res.Evals)
		} else {
			fmt.Printf("%s\t%s\t%d chars\n", status, res.Name, res.Chars)
		}
		for _, err := range res.Errors {
			fmt.Printf("  error: %s\n", err)
		}
		for _, warn := range res.Warnings {
			fmt.Printf("  warn: %s\n", warn)
		}
	}
	if !ok {
		return fmt.Errorf("one or more skills failed")
	}
	return nil
}

func configuredSkillsFS() fs.FS {
	return configuredSkillsFSContext(context.Background())
}

func configuredSkillsFSContext(ctx context.Context) fs.FS {
	if gf.skillsDir != "" {
		return os.DirFS(gf.skillsDir)
	}
	base := prismbundle.BundleFS()
	store := extensions.NewStore(gf.stateDir)
	manifest, _, err := store.RecoverAndLoadManifest(ctx)
	if err == nil {
		snapshot, composeErr := extensions.ComposeCatalog(extensions.ComposeInput{
			BundleFS:        base,
			Manifest:        manifest,
			ObjectStoreRoot: store.ObjectRoot(),
		})
		if composeErr == nil {
			if runtimeFS, materializeErr := extensions.MaterializeRuntimeBundle(base, snapshot); materializeErr == nil {
				base = runtimeFS
			}
		}
	}
	skillsFS, _ := fs.Sub(base, "skills")
	return skillsFS
}

func strictSkillAuthoringMode() bool {
	// Embedded bundle validation retains strict authoring checks for CI.
	return gf.skillsDir == ""
}
