// Package cli provides Cobra command handlers for the Prism CLI.
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/config"
)

// globalFlags are the persistent flags shared by all subcommands.
type globalFlags struct {
	rootDir    string
	agentDir   string
	skillsDir  string
	ollamaHost string
	eventStore string
	policyFile string
	stateDir   string
	verbose    bool
	jsonOut    bool
}

var gf globalFlags
var cfg config.Settings

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
	loaded, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[prism] config load warning: %v\n", err)
		loaded = config.Settings{
			RootDir:    ".",
			OllamaHost: config.DefaultOllamaHost,
			StateDir:   ".prism",
		}
	}
	cfg = loaded

	rootCmd.PersistentFlags().StringVar(&gf.rootDir, "root", cfg.RootDir,
		"Project root — local path or github.com URL. URLs are read via the GitHub Contents API (set GITHUB_TOKEN or GH_TOKEN) with git clone fallback.")
	rootCmd.PersistentFlags().StringVar(&gf.agentDir, "agent-dir", cfg.AgentDir,
		"Agent spec directory (default: <root>/agents)")
	rootCmd.PersistentFlags().StringVar(&gf.skillsDir, "skills-dir", cfg.SkillsDir,
		"Skills directory (default: <root>/skills)")
	rootCmd.PersistentFlags().StringVar(&gf.ollamaHost, "ollama-host", cfg.OllamaHost,
		"Ollama server URL [$PRISM_OLLAMA_HOST]")
	rootCmd.PersistentFlags().StringVar(&gf.stateDir, "state-dir", cfg.StateDir,
		"Prism local state directory [$PRISM_STATE_DIR]")
	rootCmd.PersistentFlags().StringVar(&gf.eventStore, "event-store", cfg.EventStore,
		"SQLite event store path; enables durable run history [$PRISM_EVENT_STORE]")
	rootCmd.PersistentFlags().StringVar(&gf.policyFile, "policy-file", cfg.PolicyFile,
		"YAML policy file; enables policy enforcement [$PRISM_POLICY_FILE]")
	rootCmd.PersistentFlags().BoolVarP(&gf.verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVar(&gf.jsonOut, "json", false, "Force JSON output")

	rootCmd.AddCommand(newAgentCmd())
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newMCPCmd())
	rootCmd.AddCommand(newBenchmarkCmd())
	rootCmd.AddCommand(newEventsCmd())
	rootCmd.AddCommand(newPolicyCmd())
	rootCmd.AddCommand(newRouteCmd())
	rootCmd.AddCommand(newSkillCmd())
	rootCmd.AddCommand(newBundleCmd())
	rootCmd.AddCommand(newRegistryCmd())
	rootCmd.AddCommand(newGraphCmd())
	rootCmd.AddCommand(newDashboardCmd())
	rootCmd.AddCommand(newReportCmd())
}

func verboseLog(format string, args ...interface{}) {
	if gf.verbose {
		fmt.Fprintf(os.Stderr, "[prism] "+format+"\n", args...)
	}
}

func eventStorePath() string {
	if gf.eventStore != "" {
		return gf.eventStore
	}
	return filepath.Join(gf.stateDir, "events.db")
}

func installedBundlesPath() string {
	return filepath.Join(gf.stateDir, "bundles.yaml")
}

func registrySourcesPath() string {
	return filepath.Join(gf.stateDir, "registry-sources.yaml")
}

func mcpServersPath() string {
	return filepath.Join(gf.stateDir, "mcp-servers.yaml")
}
