package cli

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/mcp"
)

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server",
	}
	cmd.AddCommand(newMCPServeCmd())
	return cmd
}

func newMCPServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the Prism MCP server (stdio)",
		Long: `Expose list_agents, run_agent, get_constitution, and doctor over MCP stdio.

Example Cursor mcp.json:

  {
    "mcpServers": {
      "prism": {
        "command": "prism",
        "args": ["mcp", "serve"]
      }
    }
  }`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			runner, cleanup, err := newRunner(cmd.Context())
			if err != nil {
				return fmt.Errorf("initializing runtime: %w", err)
			}
			defer cleanup()

			logger := log.New(os.Stderr, "[prism-mcp] ", log.LstdFlags)
			logger.Println(mcp.StatusSummary(runner))
			logger.Printf("ollama: %s", gf.ollamaHost)
			logger.Printf("root: %s", gf.rootDir)
			logger.Printf("agents: %s", resolvedAgentDir())
			logger.Println("tools: list_agents, run_agent, get_constitution, doctor")

			if err := mcp.Serve(context.Background(), runner); err != nil {
				return fmt.Errorf("mcp server: %w", err)
			}
			return nil
		},
	}
}
