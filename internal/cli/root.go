// Package cli implements the Cobra command tree for the prism CLI.
package cli

import (
	"github.com/spf13/cobra"
)

// Config holds resolved configuration values shared across commands.
type Config struct {
	AgentsDir  string
	SkillsRoot string
	JSONOutput bool
}

// NewRootCmd builds and returns the root cobra.Command for prism.
func NewRootCmd(cfg *Config) *cobra.Command {
	root := &cobra.Command{
		Use:   "prism",
		Short: "Prism — local specialist agent runner",
		Long: `Prism delegates focused tasks to small, specialized agents running on local
Ollama models while keeping the main LLM as the orchestrator.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&cfg.AgentsDir, "agents-dir", "agents", "directory containing agent spec files")
	root.PersistentFlags().StringVar(&cfg.SkillsRoot, "skills-dir", "skills", "directory containing skill subdirectories")
	root.PersistentFlags().BoolVar(&cfg.JSONOutput, "json", false, "emit JSON output instead of human-readable text")

	root.AddCommand(newAgentCmd(cfg))
	root.AddCommand(newSkillCmd(cfg))

	return root
}
