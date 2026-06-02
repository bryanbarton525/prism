// Package cli provides Cobra command handlers for the Prism CLI.
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// globalFlags are the persistent flags shared by all subcommands.
type globalFlags struct {
	rootDir    string
	agentDir   string
	skillsDir  string
	ollamaHost string
	verbose    bool
	jsonOut    bool
}

var gf globalFlags

// rootCmd is the Cobra root command.
var rootCmd = &cobra.Command{
	Use:   "prism",
	Short: "Prism — local specialist agent runner",
	Long: `Prism delegates focused subtasks to local Ollama specialist agents.

Each agent is defined by a Markdown+frontmatter spec and a constitution.
Skills are attached per invocation to control scope.`,
	SilenceUsage: true,
}

// Execute runs the Cobra command tree.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	home, _ := os.UserHomeDir()
	defaultRoot := "."
	if cwd, err := os.Getwd(); err == nil {
		defaultRoot = cwd
	}

	rootCmd.PersistentFlags().StringVar(&gf.rootDir, "root", defaultRoot,
		"Project root — local path or github.com URL. URLs are read via the GitHub Contents API (set GITHUB_TOKEN) with git clone fallback.")
	rootCmd.PersistentFlags().StringVar(&gf.agentDir, "agent-dir", "",
		"Agent spec directory (default: <root>/agents)")
	rootCmd.PersistentFlags().StringVar(&gf.skillsDir, "skills-dir", "",
		"Skills directory (default: <root>/skills)")
	rootCmd.PersistentFlags().StringVar(&gf.ollamaHost, "ollama-host",
		envOrDefault("PRISM_OLLAMA_HOST", "http://127.0.0.1:11434"),
		"Ollama server URL [$PRISM_OLLAMA_HOST]")
	rootCmd.PersistentFlags().BoolVarP(&gf.verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVar(&gf.jsonOut, "json", false, "Force JSON output")

	_ = home
	_ = filepath.Join // keep import

	rootCmd.AddCommand(newAgentCmd())
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newMCPCmd())
	rootCmd.AddCommand(newBenchmarkCmd())
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func verboseLog(format string, args ...interface{}) {
	if gf.verbose {
		fmt.Fprintf(os.Stderr, "[prism] "+format+"\n", args...)
	}
}
