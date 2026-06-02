package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/app"
	"github.com/bryanbarton525/prism/internal/rootresolver"
)

func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Inspect registered agent specifications",
	}
	cmd.AddCommand(newAgentListCmd())
	cmd.AddCommand(newAgentShowCmd())
	cmd.AddCommand(newAgentConstitutionCmd())
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
	fmt.Printf("Temperature:     %.2f\n", spec.Temperature)
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
// GITHUB_TOKEN is read from the environment and passed to rootresolver so the
// GitHub Contents API is used when available; git clone is the fallback.
func newRunner(ctx context.Context) (*app.Runner, func(), error) {
	token := os.Getenv("GITHUB_TOKEN")
	rootFS, cleanup, err := rootresolver.Resolve(ctx, gf.rootDir, token)
	if err != nil {
		return nil, func() {}, fmt.Errorf("resolving root %q: %w", gf.rootDir, err)
	}
	runner, err := app.New(app.Config{
		RootFS:     rootFS,
		RootLabel:  gf.rootDir,
		AgentDir:   gf.agentDir,
		SkillsDir:  gf.skillsDir,
		OllamaHost: gf.ollamaHost,
	})
	if err != nil {
		cleanup()
		return nil, func() {}, err
	}
	return runner, cleanup, nil
}


func resolvedAgentDir() string {
	if gf.agentDir != "" {
		return gf.agentDir
	}
	return gf.rootDir + "/agents"
}
