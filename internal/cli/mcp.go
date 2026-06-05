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
			policyEngine, err := configuredPolicyEngine()
			if err != nil {
				return err
			}
			eventSink, closeEventSink, err := configuredEventSink()
			if err != nil {
				return err
			}
			defer closeEventSink()
			runner, cleanup, err := newRunnerWithControls(cmd.Context(), eventSink, policyEngine)
			if err != nil {
				return fmt.Errorf("initializing runtime: %w", err)
			}
			defer cleanup()

			logger := log.New(os.Stderr, "[prism-mcp] ", log.LstdFlags)
			logger.Println(mcp.StatusSummary(runner))
			logger.Printf("ollama: %s", gf.ollamaHost)
			logger.Printf("root: %s", gf.rootDir)
			logger.Printf("agents: %s", resolvedAgentDir())
			logger.Println("tools: list_agents, run_agent, get_constitution, doctor, suggest_route, run_graph, explain_policy, list_policies")

			if err := mcp.ServeWithConfig(context.Background(), runner, mcp.Config{Policy: policyEngine, EventSink: eventSink}); err != nil {
				return fmt.Errorf("mcp server: %w", err)
			}
			return nil
		},
	}
}
