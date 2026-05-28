package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/skill"
)

func newAgentCmd(cfg *Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage and inspect agent specifications",
	}
	cmd.AddCommand(newAgentListCmd(cfg))
	cmd.AddCommand(newAgentShowCmd(cfg))
	cmd.AddCommand(newAgentValidateCmd(cfg))
	return cmd
}

func newAgentListCmd(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all available agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			agents, err := agent.LoadAll(cfg.AgentsDir)
			if err != nil {
				// Print partial results even when some agents failed.
				_, _ = fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			}
			if cfg.JSONOutput {
				type summary struct {
					ID            string   `json:"id"`
					Name          string   `json:"name"`
					Description   string   `json:"description"`
					Model         string   `json:"model"`
					AllowedSkills []string `json:"allowed_skills"`
				}
				var out []summary
				for _, a := range agents {
					out = append(out, summary{
						ID:            a.Frontmatter.ID,
						Name:          a.Frontmatter.Name,
						Description:   a.Frontmatter.Description,
						Model:         a.Frontmatter.Model,
						AllowedSkills: a.Frontmatter.AllowedSkills,
					})
				}
				return json.NewEncoder(os.Stdout).Encode(out)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tMODEL\tSKILLS")
			for _, a := range agents {
				fmt.Fprintf(w, "%s\t%s\t%s\t%v\n",
					a.Frontmatter.ID,
					a.Frontmatter.Name,
					a.Frontmatter.Model,
					a.Frontmatter.AllowedSkills,
				)
			}
			return w.Flush()
		},
	}
}

func newAgentShowCmd(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "show <agent-id>",
		Short: "Show details for a specific agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentID := args[0]
			agents, err := agent.LoadAll(cfg.AgentsDir)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			}

			var found *agent.Agent
			for _, a := range agents {
				if a.Frontmatter.ID == agentID {
					found = a
					break
				}
			}
			if found == nil {
				return fmt.Errorf("agent %q not found", agentID)
			}

			if cfg.JSONOutput {
				return json.NewEncoder(os.Stdout).Encode(found.Frontmatter)
			}

			fmt.Printf("ID:             %s\n", found.Frontmatter.ID)
			fmt.Printf("Name:           %s\n", found.Frontmatter.Name)
			fmt.Printf("Description:    %s\n", found.Frontmatter.Description)
			fmt.Printf("Model:          %s\n", found.Frontmatter.Model)
			fmt.Printf("Context Budget: %d\n", found.Frontmatter.ContextBudget)
			fmt.Printf("Latency Budget: %d ms\n", found.Frontmatter.LatencyBudgetMS)
			fmt.Printf("Allowed Skills: %v\n", found.Frontmatter.AllowedSkills)
			if len(found.Frontmatter.Tools) > 0 {
				fmt.Printf("Tools:          %v\n", found.Frontmatter.Tools)
			}
			if found.Frontmatter.ConstitutionPath != "" {
				fmt.Printf("Constitution:   %s\n", found.Frontmatter.ConstitutionPath)
			}
			return nil
		},
	}
}

func newAgentValidateCmd(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate all agent specs and skill allowlists",
		RunE: func(cmd *cobra.Command, args []string) error {
			skills, skillErr := skill.LoadAll(cfg.SkillsRoot)
			if skillErr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "skill validation errors:\n  %v\n", skillErr)
			}
			skillIndex := agent.BuildSkillIndex(skills)

			agents, agentErr := agent.LoadAll(cfg.AgentsDir)
			if agentErr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "agent validation errors:\n  %v\n", agentErr)
			}

			allowlistErr := agent.ValidateSkillAllowlists(agents, skillIndex)
			if allowlistErr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "allowlist validation errors:\n  %v\n", allowlistErr)
			}

			if skillErr != nil || agentErr != nil || allowlistErr != nil {
				return fmt.Errorf("validation failed")
			}

			fmt.Printf("OK: %d agent(s) and %d skill(s) validated successfully\n", len(agents), len(skills))
			return nil
		},
	}
}
