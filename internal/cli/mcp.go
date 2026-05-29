package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newMCPCmd() *cobra.Command {
	var addr string

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server adapter commands",
	}

	serve := &cobra.Command{
		Use:   "serve",
		Short: "Start the Prism MCP server",
		Long: `Start an MCP server that exposes Prism agents as tools for editor and
agent integrations (Cursor, Copilot, etc.).

Available tools (milestone-4):
  list_agents      — return agent IDs, summaries, and model hints
  run_agent        — invoke a specialist with a bounded task request
  get_constitution — return the constitution text for an agent
  doctor           — report Ollama connectivity and model availability`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// TODO(milestone-4): initialize Go MCP SDK server and register tools
			fmt.Fprintf(cmd.OutOrStdout(), "MCP server not implemented yet (planned listen addr: %s)\n", addr)
			return nil
		},
	}
	serve.Flags().StringVar(&addr, "addr", "stdio", "transport address: 'stdio' or 'host:port'")

	cmd.AddCommand(serve)
	return cmd
}
