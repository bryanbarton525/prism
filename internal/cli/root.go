// Package cli contains Cobra command handlers for the Prism CLI.
package cli

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/config"
)

// NewRootCmd builds the root `prism` Cobra command with all subcommands wired in.
func NewRootCmd() *cobra.Command {
	var (
		agentDir   string
		ollamaHost string
		verbose    bool
	)

	// Resolve config before command execution so flag defaults reflect env vars.
	cfg := config.Load(repoRoot())

	root := &cobra.Command{
		Use:   "prism",
		Short: "Prism – delegate focused tasks to local specialist agents",
		Long: `Prism routes narrow subtasks to local Ollama-backed specialist agents,
reducing orchestrator token usage while keeping the main LLM in control.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Apply flag overrides after cobra parses them.
			if agentDir != "" {
				cfg.AgentDir = agentDir
			}
			if ollamaHost != "" {
				cfg.OllamaHost = ollamaHost
			}
			_ = verbose
		},
	}

	root.PersistentFlags().StringVar(&agentDir, "agent-dir", cfg.AgentDir,
		"Directory containing agent spec Markdown files (overrides PRISM_AGENT_DIR)")
	root.PersistentFlags().StringVar(&ollamaHost, "ollama-host", "",
		"Ollama base URL (overrides PRISM_OLLAMA_HOST)")
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false,
		"Enable verbose output")

	root.AddCommand(NewAgentCmd(cfg))
	root.AddCommand(NewConfigCmd(cfg))

	return root
}

// repoRoot returns the directory of the running binary, falling back to the
// working directory. For development this typically resolves to the repo root.
func repoRoot() string {
	exe, err := os.Executable()
	if err != nil {
		wd, _ := os.Getwd()
		return wd
	}
	// Walk up until we find a go.mod or fall back after 5 levels.
	dir := filepath.Dir(exe)
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	wd, _ := os.Getwd()
	return wd
}
