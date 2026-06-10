package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/downstreammcp"
	"github.com/bryanbarton525/prism/internal/mcp"
)

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server and downstream MCP clients",
	}
	cmd.AddCommand(newMCPServeCmd())
	cmd.AddCommand(newMCPServerCmd())
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
			logger.Println("tools: list_agents, run_agent, get_constitution, doctor, suggest_route, run_graph, explain_policy, list_policies, list_mcp_servers, list_mcp_server_tools, call_mcp_tool")

			downstreamState, err := configuredDownstreamMCPState()
			if err != nil {
				return fmt.Errorf("loading downstream MCP servers: %w", err)
			}
			if err := mcp.ServeWithConfig(context.Background(), runner, mcp.Config{Policy: policyEngine, EventSink: eventSink, DownstreamMCP: downstreammcp.New(downstreamState)}); err != nil {
				return fmt.Errorf("mcp server: %w", err)
			}
			return nil
		},
	}
}

func newMCPServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage downstream MCP servers Prism can call",
	}
	cmd.AddCommand(newMCPServerAddCommandCmd())
	cmd.AddCommand(newMCPServerAddSSECmd())
	cmd.AddCommand(newMCPServerListCmd())
	cmd.AddCommand(newMCPServerToolsCmd())
	cmd.AddCommand(newMCPServerCallCmd())
	return cmd
}

func newMCPServerAddCommandCmd() *cobra.Command {
	var timeoutMS int
	var maxBytes int
	cmd := &cobra.Command{
		Use:   "add-command <name> <command> [args...]",
		Short: "Add a downstream MCP server launched as a command",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			state, err := downstreammcp.Load(mcpServersPath())
			if err != nil {
				return err
			}
			server := downstreammcp.Server{
				Name:      args[0],
				Transport: downstreammcp.TransportCommand,
				Command:   args[1],
				Args:      append([]string{}, args[2:]...),
				TimeoutMS: timeoutMS,
				MaxBytes:  maxBytes,
			}
			if err := server.Validate(); err != nil {
				return err
			}
			state.Upsert(server)
			if err := downstreammcp.Save(mcpServersPath(), state); err != nil {
				return err
			}
			fmt.Printf("downstream MCP server %s saved\n", server.Name)
			return nil
		},
	}
	cmd.Flags().IntVar(&timeoutMS, "timeout-ms", downstreammcp.DefaultTimeoutMS, "Per-call timeout in milliseconds")
	cmd.Flags().IntVar(&maxBytes, "max-bytes", downstreammcp.DefaultMaxBytes, "Maximum returned content bytes")
	cmd.Flags().SetInterspersed(false)
	return cmd
}

func newMCPServerAddSSECmd() *cobra.Command {
	var timeoutMS int
	var maxBytes int
	cmd := &cobra.Command{
		Use:   "add-sse <name> <url>",
		Short: "Add a downstream MCP server using SSE transport",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			state, err := downstreammcp.Load(mcpServersPath())
			if err != nil {
				return err
			}
			server := downstreammcp.Server{
				Name:      args[0],
				Transport: downstreammcp.TransportSSE,
				URL:       args[1],
				TimeoutMS: timeoutMS,
				MaxBytes:  maxBytes,
			}
			if err := server.Validate(); err != nil {
				return err
			}
			state.Upsert(server)
			if err := downstreammcp.Save(mcpServersPath(), state); err != nil {
				return err
			}
			fmt.Printf("downstream MCP server %s saved\n", server.Name)
			return nil
		},
	}
	cmd.Flags().IntVar(&timeoutMS, "timeout-ms", downstreammcp.DefaultTimeoutMS, "Per-call timeout in milliseconds")
	cmd.Flags().IntVar(&maxBytes, "max-bytes", downstreammcp.DefaultMaxBytes, "Maximum returned content bytes")
	return cmd
}

func newMCPServerListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured downstream MCP servers",
		RunE: func(_ *cobra.Command, _ []string) error {
			state, err := configuredDownstreamMCPState()
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(state.PublicServers())
		},
	}
}

func newMCPServerToolsCmd() *cobra.Command {
	var includeSchema bool
	cmd := &cobra.Command{
		Use:   "tools <name>",
		Short: "List tools exposed by a downstream MCP server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			state, err := configuredDownstreamMCPState()
			if err != nil {
				return err
			}
			tools, err := downstreammcp.New(state).ListTools(cmd.Context(), args[0], downstreammcp.ListToolsOptions{IncludeSchema: includeSchema})
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(tools)
		},
	}
	cmd.Flags().BoolVar(&includeSchema, "schema", false, "Include tool input schemas")
	return cmd
}

func newMCPServerCallCmd() *cobra.Command {
	var rawArgs string
	cmd := &cobra.Command{
		Use:   "call <name> <tool>",
		Short: "Call one tool on a downstream MCP server",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			state, err := configuredDownstreamMCPState()
			if err != nil {
				return err
			}
			toolArgs, err := downstreammcp.ParseArguments(rawArgs)
			if err != nil {
				return fmt.Errorf("parsing --args-json: %w", err)
			}
			res, err := downstreammcp.New(state).CallTool(cmd.Context(), args[0], args[1], toolArgs)
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(res)
		},
	}
	cmd.Flags().StringVar(&rawArgs, "args-json", "{}", "JSON object of tool arguments")
	return cmd
}
