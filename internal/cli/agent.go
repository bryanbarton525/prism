package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/app"
)

func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Inspect registered agent specifications",
	}
	cmd.AddCommand(newAgentListCmd())
	cmd.AddCommand(newAgentShowCmd())
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
	runner, err := app.New(app.Config{
		AgentDir:   gf.agentDir,
		SkillsDir:  gf.skillsDir,
		OllamaHost: gf.ollamaHost,
	})
	if err != nil {
		return err
	}

	summaries, err := runner.ListAgents(ctx)
	if err != nil {
		return err
	}

	if len(summaries) == 0 {
		fmt.Println("No agents found in", gf.agentDir)
		return nil
	}

	if jsonOut {
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
	runner, err := app.New(app.Config{
		AgentDir:   gf.agentDir,
		SkillsDir:  gf.skillsDir,
		OllamaHost: gf.ollamaHost,
	})
	if err != nil {
		return err
	}

	spec, err := runner.GetSpec(ctx, agentID)
	if err != nil {
		return err
	}

	if jsonOut {
		type specView struct {
			agent.Spec
		}
		data, _ := json.MarshalIndent(spec, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("ID:              %s\n", spec.ID)
	fmt.Printf("Name:            %s\n", spec.Name)
	fmt.Printf("Description:     %s\n", spec.Description)
	fmt.Printf("Model:           %s\n", spec.Model)
	fmt.Printf("Context budget:  %d tokens\n", spec.ContextBudget)
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
	fmt.Println()
	fmt.Println("--- Constitution body ---")
	fmt.Println(spec.Body)
	return nil
}
