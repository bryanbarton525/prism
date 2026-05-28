package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/skill"
)

func newSkillCmd(cfg *Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage and inspect Agent Skills",
	}
	cmd.AddCommand(newSkillListCmd(cfg))
	cmd.AddCommand(newSkillShowCmd(cfg))
	cmd.AddCommand(newSkillValidateCmd(cfg))
	return cmd
}

func newSkillListCmd(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all available Agent Skills",
		RunE: func(cmd *cobra.Command, args []string) error {
			skills, err := skill.LoadAll(cfg.SkillsRoot)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			}
			if cfg.JSONOutput {
				type summary struct {
					Name        string `json:"name"`
					Description string `json:"description"`
				}
				var out []summary
				for _, s := range skills {
					out = append(out, summary{
						Name:        s.Frontmatter.Name,
						Description: s.Frontmatter.Description,
					})
				}
				return json.NewEncoder(os.Stdout).Encode(out)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tDESCRIPTION")
			for _, s := range skills {
				desc := s.Frontmatter.Description
				if len(desc) > 80 {
					desc = desc[:77] + "..."
				}
				fmt.Fprintf(w, "%s\t%s\n", s.Frontmatter.Name, desc)
			}
			return w.Flush()
		},
	}
}

func newSkillShowCmd(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "show <skill-name>",
		Short: "Show details for a specific Agent Skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			skillName := args[0]
			skills, err := skill.LoadAll(cfg.SkillsRoot)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			}

			var found *skill.Skill
			for _, s := range skills {
				if s.DirName == skillName {
					found = s
					break
				}
			}
			if found == nil {
				return fmt.Errorf("skill %q not found", skillName)
			}

			if cfg.JSONOutput {
				return json.NewEncoder(os.Stdout).Encode(found.Frontmatter)
			}

			fmt.Printf("Name:          %s\n", found.Frontmatter.Name)
			fmt.Printf("Description:   %s\n", found.Frontmatter.Description)
			if found.Frontmatter.Compatibility != "" {
				fmt.Printf("Compatibility: %s\n", found.Frontmatter.Compatibility)
			}
			if len(found.Frontmatter.Metadata) > 0 {
				for k, v := range found.Frontmatter.Metadata {
					fmt.Printf("Metadata[%s]: %s\n", k, v)
				}
			}
			return nil
		},
	}
}

func newSkillValidateCmd(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate all Agent Skills in the skills directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			skills, err := skill.LoadAll(cfg.SkillsRoot)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "validation errors:\n  %v\n", err)
				return fmt.Errorf("validation failed")
			}
			fmt.Printf("OK: %d skill(s) validated successfully\n", len(skills))
			return nil
		},
	}
}
