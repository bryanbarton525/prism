// Package cli implements all Cobra command handlers for Prism.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// globalFlags holds values shared across all commands.
type globalFlags struct {
	agentDir   string
	skillsDir  string
	ollamaHost string
	verbose    bool
}

var gf globalFlags

// NewRootCmd builds and returns the root Cobra command.
func NewRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "prism",
		Short: "Prism – delegate focused tasks to local Ollama specialist agents",
		Long: `Prism separates orchestration from execution by routing narrow subtasks
to local Ollama agents with explicit constitutions, skills, and tool allowlists.

The calling LLM (Cursor, Copilot, etc.) remains the orchestrator; Prism provides
the local execution layer with auditable prompts and structured results.`,
		SilenceUsage: true,
	}

	// Persistent flags available to every subcommand.
	root.PersistentFlags().StringVar(&gf.agentDir, "agent-dir", agentDirDefault(),
		"Directory containing agent spec files (*.md)")
	root.PersistentFlags().StringVar(&gf.skillsDir, "skills-dir", skillsDirDefault(),
		"Directory containing Agent Skills subdirectories")
	root.PersistentFlags().StringVar(&gf.ollamaHost, "ollama-host", ollamaHostDefault(),
		"Ollama server base URL")
	root.PersistentFlags().BoolVarP(&gf.verbose, "verbose", "v", false, "Enable verbose output")

	root.AddCommand(newAgentCmd())
	root.AddCommand(newRunCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newVersionCmd(version))

	return root
}

// Execute runs the root command and exits on error.
func Execute(version string) {
	if err := NewRootCmd(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// Default resolution helpers (flag > env > hardcoded default)
// ---------------------------------------------------------------------------

func agentDirDefault() string {
	if v := os.Getenv("PRISM_AGENT_DIR"); v != "" {
		return v
	}
	return "agents"
}

func skillsDirDefault() string {
	if v := os.Getenv("PRISM_SKILLS_DIR"); v != "" {
		return v
	}
	return "skills"
}

func ollamaHostDefault() string {
	if v := os.Getenv("PRISM_OLLAMA_HOST"); v != "" {
		return v
	}
	return "http://127.0.0.1:11434"
}
