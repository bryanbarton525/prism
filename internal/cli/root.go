// Package cli contains all Cobra command definitions for the prism binary.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/app"
	"github.com/bryanbarton525/prism/internal/config"
)

// globalFlags are bound to persistent flags on the root command so every
// subcommand inherits them.
type globalFlags struct {
	configPath   string
	ollamaHost   string
	defaultModel string
	agentDir     string
	verbose      bool
	jsonOutput   bool
}

var gf globalFlags

// NewRootCmd constructs the root cobra.Command. The returned command owns the
// full command tree; call Execute() on it from main.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "prism",
		Short: "Delegate focused tasks to local Ollama specialist agents",
		Long: `Prism routes narrow subtasks to local specialist agents running on Ollama,
keeping the primary LLM as orchestrator while reducing paid-model token usage.

Configuration is resolved in priority order:
  1. command flags
  2. environment variables (PRISM_OLLAMA_HOST, PRISM_DEFAULT_MODEL, PRISM_AGENT_DIR, PRISM_CONFIG)
  3. config file (~/.config/prism/config.json or PRISM_CONFIG)
  4. built-in defaults`,
		SilenceUsage: true,
		// PersistentPreRunE wires flags into config before any subcommand runs.
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			_, err := buildApp(cmd)
			return err
		},
	}

	root.PersistentFlags().StringVar(&gf.configPath, "config", "", "config file path (default: ~/.config/prism/config.json)")
	root.PersistentFlags().StringVar(&gf.ollamaHost, "ollama-host", "", "Ollama API base URL (env: PRISM_OLLAMA_HOST)")
	root.PersistentFlags().StringVar(&gf.defaultModel, "model", "", "default Ollama model tag (env: PRISM_DEFAULT_MODEL)")
	root.PersistentFlags().StringVar(&gf.agentDir, "agent-dir", "", "directory containing agent specs (env: PRISM_AGENT_DIR)")
	root.PersistentFlags().BoolVarP(&gf.verbose, "verbose", "v", false, "verbose output")
	root.PersistentFlags().BoolVar(&gf.jsonOutput, "json", false, "emit JSON output where supported")

	root.AddCommand(newAgentCmd())
	root.AddCommand(newRunCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newMCPCmd())

	return root
}

// buildApp resolves config and constructs the App value for a command invocation.
func buildApp(_ *cobra.Command) (*app.App, error) {
	flags := config.Flags{
		ConfigPath:   gf.configPath,
		OllamaHost:   gf.ollamaHost,
		DefaultModel: gf.defaultModel,
		AgentDir:     gf.agentDir,
	}
	cfg, err := config.Load(flags)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	return app.New(cfg), nil
}

// Execute runs the root command and exits on error.
func Execute() {
	root := NewRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
