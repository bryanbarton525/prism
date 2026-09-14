package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	prismbundle "github.com/bryanbarton525/prism"
	"github.com/bryanbarton525/prism/internal/app"
	"github.com/bryanbarton525/prism/internal/downstreammcp"
	"github.com/bryanbarton525/prism/internal/events"
	"github.com/bryanbarton525/prism/internal/extensions"
	"github.com/bryanbarton525/prism/internal/graphify"
	internalpolicy "github.com/bryanbarton525/prism/internal/policy"
	"github.com/bryanbarton525/prism/internal/rootresolver"
	"github.com/bryanbarton525/prism/pkg/observe"
)

func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Inspect registered agent specifications",
	}
	cmd.AddCommand(newAgentListCmd())
	cmd.AddCommand(newAgentShowCmd())
	cmd.AddCommand(newAgentConstitutionCmd())
	cmd.AddCommand(newAgentAddCmd(), newAgentManagedCmd())
	return cmd
}

func newAgentAddCmd() *cobra.Command {
	var as string
	var replace bool
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "add <source-file-or-dir>",
		Short: "Install a local managed agent into runtime extensions",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			svc := extensions.NewLocalAgentService(gf.stateDir)
			entry, err := svc.InstallLocalAgent(context.Background(), extensions.InstallLocalAgentRequest{
				Source:  args[0],
				As:      as,
				Replace: replace,
				DryRun:  dryRun,
			})
			if err != nil {
				return err
			}
			if gf.jsonOut {
				data, _ := json.MarshalIndent(entry, "", "  ")
				fmt.Println(string(data))
				return nil
			}
			action := "installed"
			if dryRun {
				action = "would install"
			}
			fmt.Printf("%s agent %s (%s)\n", action, entry.Identity, entry.Digest)
			return nil
		},
	}
	cmd.Flags().StringVar(&as, "as", "", "Rename installed agent identity")
	cmd.Flags().BoolVar(&replace, "replace", false, "Replace existing managed agent")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without mutation")
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
		RunE: func(_ *cobra.Command, _ []string) error {
			svc := extensions.NewLocalAgentService(gf.stateDir)
			entries, err := svc.ListManagedAgents(context.Background())
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
		RunE: func(_ *cobra.Command, args []string) error {
			svc := extensions.NewLocalAgentService(gf.stateDir)
			removed, err := svc.RemoveManagedAgent(context.Background(), args[0], dryRun)
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
		RunE: func(_ *cobra.Command, args []string) error {
			svc := extensions.NewLocalAgentService(gf.stateDir)
			ok, err := svc.CopyManagedAgent(context.Background(), args[0], args[1], dryRun)
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
		RunE: func(_ *cobra.Command, args []string) error {
			svc := extensions.NewLocalAgentService(gf.stateDir)
			ok, err := svc.RenameManagedAgent(context.Background(), args[0], args[1], dryRun)
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
	if len(summaries) == 0 {
		fmt.Fprintln(os.Stderr, "No agents found in", resolvedAgentDir())
		return nil
	}
	if jsonOut || gf.jsonOut {
		data, _ := json.MarshalIndent(summaries, "", "  ")
		fmt.Println(string(data))
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tMODEL\tDESCRIPTION")
	for _, s := range summaries {
		desc := s.Description
		if len(desc) > 72 {
			desc = desc[:69] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.ID, s.Name, s.Model, desc)
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
		return err
	}
	if jsonOut || gf.jsonOut {
		data, _ := json.MarshalIndent(spec, "", "  ")
		fmt.Println(string(data))
		return nil
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
	mcpAccess, mcpAccessConfigured, err := extensions.LoadMCPAccess(gf.stateDir)
	if err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("loading MCP access policy: %w", err)
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
		}
		if gf.skillsDir != "" {
			skillsFS = os.DirFS(gf.skillsDir)
		}
		bundleDigest = prismbundle.DigestParts(map[string]fs.FS{"agents": agentsFS, "skills": skillsFS, "constitutions": constitutionsFS})
	}
	runner, err := app.New(app.Config{
		BundleFS:            prismbundle.BundleFS(),
		BundleDigest:        bundleDigest,
		BundleMode:          bundleMode,
		WorkspaceFS:         workspaceFS,
		WorkspaceLabel:      workspaceRoot,
		GitHubToken:         cfg.GitHubToken,
		AgentDir:            gf.agentDir,
		SkillsDir:           gf.skillsDir,
		OllamaHost:          gf.ollamaHost,
		EventSink:           sink,
		PolicyEngine:        policyEngine,
		DownstreamMCP:       downstreammcp.New(mcpState),
		ModelRuntime:        modelRuntime,
		ExtensionsStateDir:  gf.stateDir,
		MCPAccess:           mcpAccess,
		MCPAccessConfigured: mcpAccessConfigured,
		Graphify:            graphifyConfig,
	})
	if err != nil {
		cleanup()
		return nil, func() {}, err
	}
	return runner, cleanup, nil
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
