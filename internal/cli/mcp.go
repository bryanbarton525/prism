package cli

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/app"
	"github.com/bryanbarton525/prism/internal/mcp"
)

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server management",
	}
	cmd.AddCommand(newMCPServeCmd())
	return cmd
}

func newMCPServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the Prism MCP server (stdio transport)",
		Long: `Start the Prism MCP server over stdio. Register this command as an MCP
server in your editor or agent client configuration.

Example ~/.cursor/mcp.json entry:

  {
    "mcpServers": {
      "prism": {
        "command": "prism",
        "args": ["mcp", "serve"]
      }
    }
  }`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := buildConfig()
			runner, err := app.New(cfg)
			if err != nil {
				return fmt.Errorf("initializing prism runtime: %w", err)
			}

			logger := log.New(os.Stderr, "[prism-mcp] ", log.LstdFlags)
			logger.Println(mcp.StatusSummary(runner))
			logger.Printf("ollama host: %s", cfg.OllamaHost)
			logger.Printf("agent dir:   %s", cfg.AgentDir)
			logger.Printf("skills dir:  %s", cfg.SkillsDir)
			logger.Println("tools: list_agents, run_agent, get_constitution, doctor")
			logger.Println("waiting for MCP client connection on stdio…")

			if err := mcp.Serve(context.Background(), runner); err != nil {
				return fmt.Errorf("mcp server error: %w", err)
			}
			return nil
		},
	}
}
