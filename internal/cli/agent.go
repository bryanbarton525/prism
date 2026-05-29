package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/app"
)

func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Inspect registered agents",
	}
	cmd.AddCommand(newAgentListCmd())
	cmd.AddCommand(newAgentShowCmd())
	return cmd
}

func newAgentListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all registered agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := buildConfig()
			runner, err := app.New(cfg)
			if err != nil {
				return err
			}
			summaries, err := runner.ListAgents(context.Background())
			if err != nil {
				return err
			}
			if global.jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(summaries)
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tNAME\tMODEL\tLATENCY_BUDGET\tSKILLS")
			for _, s := range summaries {
				skills := ""
				for i, sk := range s.AllowedSkills {
					if i > 0 {
						skills += ", "
					}
					skills += sk
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%dms\t%s\n",
					s.ID, s.Name, s.Model, s.LatencyBudgetMs, skills)
			}
			return tw.Flush()
		},
	}
}

func newAgentShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <agent-id>",
		Short: "Show metadata and constitution for an agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentID := args[0]
			cfg := buildConfig()
			runner, err := app.New(cfg)
			if err != nil {
				return err
			}

			summaries, err := runner.ListAgents(context.Background())
			if err != nil {
				return err
			}
			var found *struct {
				ID              string
				Name            string
				Description     string
				Model           string
				AllowedSkills   []string
				LatencyBudgetMs int
			}
			for _, s := range summaries {
				if s.ID == agentID {
					found = &struct {
						ID              string
						Name            string
						Description     string
						Model           string
						AllowedSkills   []string
						LatencyBudgetMs int
					}{
						ID:              s.ID,
						Name:            s.Name,
						Description:     s.Description,
						Model:           s.Model,
						AllowedSkills:   s.AllowedSkills,
						LatencyBudgetMs: s.LatencyBudgetMs,
					}
					break
				}
			}
			if found == nil {
				return fmt.Errorf("agent %q not found", agentID)
			}

			constitution, err := runner.GetConstitution(context.Background(), agentID)
			if err != nil {
				return err
			}

			if global.jsonOutput {
				out := map[string]any{
					"id":                found.ID,
					"name":              found.Name,
					"description":       found.Description,
					"model":             found.Model,
					"allowed_skills":    found.AllowedSkills,
					"latency_budget_ms": found.LatencyBudgetMs,
					"constitution":      constitution.Text,
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			fmt.Printf("ID:               %s\n", found.ID)
			fmt.Printf("Name:             %s\n", found.Name)
			fmt.Printf("Description:      %s\n", found.Description)
			fmt.Printf("Model:            %s\n", found.Model)
			fmt.Printf("Latency budget:   %dms\n", found.LatencyBudgetMs)
			fmt.Printf("Allowed skills:   ")
			for i, sk := range found.AllowedSkills {
				if i > 0 {
					fmt.Print(", ")
				}
				fmt.Print(sk)
			}
			fmt.Println()
			fmt.Printf("\n--- Constitution ---\n%s\n", constitution.Text)
			return nil
		},
	}
}
