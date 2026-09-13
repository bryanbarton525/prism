package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

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
	cmd.AddCommand(newMCPAddCmd())
	cmd.AddCommand(newMCPListCmd())
	cmd.AddCommand(newMCPShowCmd())
	cmd.AddCommand(newMCPRemoveCmd())
	cmd.AddCommand(newMCPToolsCmd())
	cmd.AddCommand(newMCPCallCmd())
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
			runner, cleanup, err := newRunnerWithControls(cmd.Context(), eventSink, policyEngine, false)
			if err != nil {
				return fmt.Errorf("initializing runtime: %w", err)
			}
			defer cleanup()

			logger := log.New(os.Stderr, "[prism-mcp] ", log.LstdFlags)
			logger.Println(mcp.StatusSummary(runner))
			if cfg.ModelRuntime.Primary.Engine != "" || cfg.ModelRuntime.Primary.BaseURL != "" {
				logger.Printf("model_runtime: engine=%s base_url=%s model=%s", cfg.ModelRuntime.Primary.Engine, cfg.ModelRuntime.Primary.BaseURL, cfg.ModelRuntime.Primary.Model)
			} else {
				logger.Printf("ollama: %s", gf.ollamaHost)
			}
			if gf.rootDir != "" {
				logger.Printf("workspace fallback: %s", gf.rootDir)
			} else {
				logger.Printf("workspace fallback: none")
			}
			logger.Printf("agents: %s", resolvedAgentDir())
			logger.Println("tools: list_agents, run_agent, get_constitution, doctor, suggest_route, run_graph, explain_policy, list_policies, get_usage_summary, get_skill_health, list_mcp_servers, list_mcp_server_tools, call_mcp_tool")

			downstreamState, err := configuredDownstreamMCPState()
			if err != nil {
				return fmt.Errorf("loading downstream MCP servers: %w", err)
			}
			if err := mcp.ServeWithConfig(context.Background(), runner, mcp.Config{
				Policy:         policyEngine,
				EventSink:      eventSink,
				DownstreamMCP:  downstreammcp.New(downstreamState),
				EventStorePath: eventStorePath(),
				RootDir:        gf.rootDir,
				SkillsDir:      gf.skillsDir,
			}); err != nil {
				return fmt.Errorf("mcp server: %w", err)
			}
			return nil
		},
	}
}

func newMCPAddCmd() *cobra.Command {
	var transport string
	var command string
	var commandArgs []string
	var url string
	var timeoutMS int
	var maxBytes int
	var replace bool
	var dryRun bool
	var description string
	var envRefs []string
	var headerRefs []string

	cmd := &cobra.Command{
		Use:   "add [flags] <name>",
		Short: "Add a downstream MCP server using command, sse, or streamable-http transport",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			server := downstreammcp.Server{
				Name:        name,
				Transport:   transport,
				Command:     strings.TrimSpace(command),
				Args:        append([]string{}, commandArgs...),
				EnvRefs:     parseReferenceAssignments(envRefs),
				URL:         strings.TrimSpace(url),
				HeaderRefs:  parseReferenceAssignments(headerRefs),
				TimeoutMS:   timeoutMS,
				MaxBytes:    maxBytes,
				Description: strings.TrimSpace(description),
			}
			if err := server.Validate(); err != nil {
				return err
			}
			return mutateDownstreamMCPServer(cmd.Context(), server, replace, dryRun)
		},
	}
	cmd.Flags().StringVar(&transport, "transport", downstreammcp.TransportCommand, "Transport: command, sse, streamable-http")
	cmd.Flags().StringVar(&command, "command", "", "Executable for command transport")
	cmd.Flags().StringArrayVar(&commandArgs, "arg", nil, "Command argument (repeatable)")
	cmd.Flags().StringVar(&url, "url", "", "Endpoint URL for sse or streamable-http transport")
	cmd.Flags().IntVar(&timeoutMS, "timeout-ms", downstreammcp.DefaultTimeoutMS, "Per-call timeout in milliseconds")
	cmd.Flags().IntVar(&maxBytes, "max-bytes", downstreammcp.DefaultMaxBytes, "Maximum returned content bytes")
	cmd.Flags().StringVar(&description, "description", "", "Human-readable server description")
	cmd.Flags().BoolVar(&replace, "replace", false, "Replace existing server with same name")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show outcome without writing state")
	cmd.Flags().StringArrayVar(&envRefs, "env-ref", nil, "Command env reference assignment KEY=ENV_VAR (repeatable)")
	cmd.Flags().StringArrayVar(&headerRefs, "header-ref", nil, "HTTP header reference assignment HEADER=ENV_VAR (repeatable)")
	return cmd
}

func newMCPListCmd() *cobra.Command {
	return newMCPServerListCmd()
}

func newMCPShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show one configured downstream MCP server",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			state, err := configuredDownstreamMCPState()
			if err != nil {
				return err
			}
			server, ok := state.Get(args[0])
			if !ok {
				return fmt.Errorf("downstream MCP server %s not found", args[0])
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(server)
		},
	}
}

func newMCPRemoveCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "remove [flags] <name>",
		Short: "Remove one configured downstream MCP server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if dryRun {
				state, err := downstreammcp.Load(mcpServersPath())
				if err != nil {
					return err
				}
				if _, ok := state.Get(name); !ok {
					fmt.Printf("downstream MCP server %s unchanged\n", name)
					return nil
				}
				fmt.Printf("downstream MCP server %s removed\n", name)
				return nil
			}
			result, err := downstreammcp.NewService(mcpServersPath()).Remove(cmd.Context(), name)
			if err != nil {
				return err
			}
			switch result.Outcome {
			case downstreammcp.OutcomeRemoved:
				fmt.Printf("downstream MCP server %s removed\n", name)
			case downstreammcp.OutcomeUnchanged:
				fmt.Printf("downstream MCP server %s unchanged\n", name)
			case downstreammcp.OutcomeConflict:
				return fmt.Errorf("downstream MCP server %s changed concurrently; retry remove", name)
			default:
				return fmt.Errorf("unexpected downstream MCP remove outcome: %s", result.Outcome)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show outcome without writing state")
	return cmd
}

func newMCPToolsCmd() *cobra.Command {
	return newMCPServerToolsCmd()
}

func newMCPCallCmd() *cobra.Command {
	return newMCPServerCallCmd()
}

func newMCPServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Compatibility wrappers for downstream MCP commands",
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
	var replace bool
	var description string
	var envRefs []string
	cmd := &cobra.Command{
		Use:   "add-command [flags] <name> <command> [args...]",
		Short: "Add a downstream MCP server launched as a command",
		Long: `Add a downstream MCP server launched as a command.

Everything after <command> is passed to the downstream server verbatim
(so wrappers like "npx -y mcp-remote ..." work). Because of that, prism
flags such as --timeout-ms and --max-bytes must come BEFORE <name>:

  prism mcp server add-command --timeout-ms 1500 linear npx -y mcp-remote https://mcp.linear.app/mcp`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := rejectMisplacedPrismFlags(args); err != nil {
				return err
			}
			server := downstreammcp.Server{
				Name:        args[0],
				Transport:   downstreammcp.TransportCommand,
				Command:     args[1],
				Args:        append([]string{}, args[2:]...),
				EnvRefs:     parseReferenceAssignments(envRefs),
				TimeoutMS:   timeoutMS,
				MaxBytes:    maxBytes,
				Description: strings.TrimSpace(description),
			}
			if err := server.Validate(); err != nil {
				return err
			}
			return mutateDownstreamMCPServer(context.Background(), server, replace, false)
		},
	}
	cmd.Flags().IntVar(&timeoutMS, "timeout-ms", downstreammcp.DefaultTimeoutMS, "Per-call timeout in milliseconds")
	cmd.Flags().IntVar(&maxBytes, "max-bytes", downstreammcp.DefaultMaxBytes, "Maximum returned content bytes")
	cmd.Flags().BoolVar(&replace, "replace", false, "Replace existing server with same name")
	cmd.Flags().StringVar(&description, "description", "", "Human-readable server description")
	cmd.Flags().StringArrayVar(&envRefs, "env-ref", nil, "Command env reference assignment KEY=ENV_VAR (repeatable)")
	cmd.Flags().SetInterspersed(false)
	return cmd
}

// rejectMisplacedPrismFlags catches prism's own flags appearing after the
// positional args. Flag parsing is non-interspersed here so downstream
// command flags (e.g. "npx -y") pass through verbatim — which means a
// trailing --timeout-ms would silently become an argument to the downstream
// command while the default timeout stayed in effect.
func rejectMisplacedPrismFlags(args []string) error {
	for _, a := range args {
		for _, flag := range []string{"--timeout-ms", "--max-bytes"} {
			if a == flag || strings.HasPrefix(a, flag+"=") {
				return fmt.Errorf(
					"%s appears after the command arguments and would be passed to the downstream server instead of prism; place it before <name>: prism mcp server add-command %s ... <name> <command> [args...]",
					flag, a)
			}
		}
	}
	return nil
}

func newMCPServerAddSSECmd() *cobra.Command {
	var timeoutMS int
	var maxBytes int
	var replace bool
	var description string
	var headerRefs []string
	cmd := &cobra.Command{
		Use:   "add-sse <name> <url>",
		Short: "Add a downstream MCP server using SSE transport",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			server := downstreammcp.Server{
				Name:        args[0],
				Transport:   downstreammcp.TransportSSE,
				URL:         args[1],
				HeaderRefs:  parseReferenceAssignments(headerRefs),
				TimeoutMS:   timeoutMS,
				MaxBytes:    maxBytes,
				Description: strings.TrimSpace(description),
			}
			if err := server.Validate(); err != nil {
				return err
			}
			return mutateDownstreamMCPServer(context.Background(), server, replace, false)
		},
	}
	cmd.Flags().IntVar(&timeoutMS, "timeout-ms", downstreammcp.DefaultTimeoutMS, "Per-call timeout in milliseconds")
	cmd.Flags().IntVar(&maxBytes, "max-bytes", downstreammcp.DefaultMaxBytes, "Maximum returned content bytes")
	cmd.Flags().BoolVar(&replace, "replace", false, "Replace existing server with same name")
	cmd.Flags().StringVar(&description, "description", "", "Human-readable server description")
	cmd.Flags().StringArrayVar(&headerRefs, "header-ref", nil, "HTTP header reference assignment HEADER=ENV_VAR (repeatable)")
	return cmd
}

func printDownstreamMCPMutation(name, outcome string) error {
	switch outcome {
	case downstreammcp.OutcomeCreated:
		fmt.Printf("downstream MCP server %s created\n", name)
		return nil
	case downstreammcp.OutcomeUnchanged:
		fmt.Printf("downstream MCP server %s unchanged\n", name)
		return nil
	case downstreammcp.OutcomeReplaced:
		fmt.Printf("downstream MCP server %s replaced\n", name)
		return nil
	case downstreammcp.OutcomeConflict:
		return fmt.Errorf("downstream MCP server %s already exists with different settings; rerun with --replace", name)
	default:
		return fmt.Errorf("unexpected downstream MCP mutation outcome: %s", outcome)
	}
}

func mutateDownstreamMCPServer(ctx context.Context, server downstreammcp.Server, replace, dryRun bool) error {
	if dryRun {
		state, err := downstreammcp.Load(mcpServersPath())
		if err != nil {
			return err
		}
		current, ok := state.Get(server.Name)
		if !ok {
			return printDownstreamMCPMutation(server.Name, downstreammcp.OutcomeCreated)
		}
		if current.Equals(server) {
			return printDownstreamMCPMutation(server.Name, downstreammcp.OutcomeUnchanged)
		}
		if !replace {
			return printDownstreamMCPMutation(server.Name, downstreammcp.OutcomeConflict)
		}
		return printDownstreamMCPMutation(server.Name, downstreammcp.OutcomeReplaced)
	}
	result, err := downstreammcp.NewService(mcpServersPath()).AddOrUpdate(ctx, server, replace)
	if err != nil {
		return err
	}
	return printDownstreamMCPMutation(server.Name, result.Outcome)
}

func parseReferenceAssignments(assignments []string) map[string]string {
	out := map[string]string{}
	for _, assignment := range assignments {
		parts := strings.SplitN(assignment, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		ref := strings.TrimSpace(parts[1])
		if key == "" || ref == "" {
			continue
		}
		out[key] = ref
	}
	if len(out) == 0 {
		return nil
	}
	return out
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
