// Package cli contains the Cobra command handlers for the prism CLI.
package cli

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/app"
)

// globalFlags holds values shared across all subcommands.
type globalFlags struct {
	ollamaHost string
	agentDir   string
	skillsDir  string
	verbose    bool
	jsonOutput bool
}

var global globalFlags

// NewRootCmd builds and returns the root cobra command.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "prism",
		Short: "Prism – delegate focused tasks to local specialist agents",
		Long: `Prism runs specialized local agents on Ollama models, reducing paid LLM
token usage by offloading narrow subtasks to small, tool-focused agents.`,
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVar(&global.ollamaHost, "ollama-host", envOrDefault("PRISM_OLLAMA_HOST", "http://127.0.0.1:11434"), "Ollama host URL ($PRISM_OLLAMA_HOST)")
	root.PersistentFlags().StringVar(&global.agentDir, "agent-dir", envOrDefault("PRISM_AGENT_DIR", "agents"), "directory containing agent specs ($PRISM_AGENT_DIR)")
	root.PersistentFlags().StringVar(&global.skillsDir, "skills-dir", envOrDefault("PRISM_SKILLS_DIR", "skills"), "directory containing skills ($PRISM_SKILLS_DIR)")
	root.PersistentFlags().BoolVarP(&global.verbose, "verbose", "v", false, "enable verbose output")
	root.PersistentFlags().BoolVar(&global.jsonOutput, "json", false, "emit JSON output")

	root.AddCommand(newAgentCmd())
	root.AddCommand(newRunCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newMCPCmd())

	return root
}

// buildConfig resolves configuration and returns an app.Config. repoRoot is
// used for constitution_path resolution when it is relative.
func buildConfig() app.Config {
	repoRoot, _ := os.Getwd()
	agentDir := global.agentDir
	if !filepath.IsAbs(agentDir) {
		agentDir = filepath.Join(repoRoot, agentDir)
	}
	skillsDir := global.skillsDir
	if !filepath.IsAbs(skillsDir) {
		skillsDir = filepath.Join(repoRoot, skillsDir)
	}
	return app.Config{
		OllamaHost: global.ollamaHost,
		AgentDir:   agentDir,
		SkillsDir:  skillsDir,
		RepoRoot:   repoRoot,
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
