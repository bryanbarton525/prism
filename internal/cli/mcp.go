package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
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
	var envFrom []string
	var headerFrom []string
	var legacyEnvRef []string
	var legacyHeaderRef []string

	cmd := &cobra.Command{
		Use:   "add [flags] <name> [-- <command> [args...]]",
		Short: "Add a downstream MCP server using command, sse, or streamable-http transport",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			finalTransport, err := resolveAddTransport(transport, strings.TrimSpace(url))
			if err != nil {
				return err
			}
			envAssignments := append([]string{}, envFrom...)
			envAssignments = append(envAssignments, legacyEnvRef...)
			headerAssignments := append([]string{}, headerFrom...)
			headerAssignments = append(headerAssignments, legacyHeaderRef...)
			envRefs, err := parseReferenceAssignments(envAssignments, "--env-from")
			if err != nil {
				return err
			}
			headerRefs, err := parseReferenceAssignments(headerAssignments, "--header-from")
			if err != nil {
				return err
			}
			resolvedCommand := strings.TrimSpace(command)
			resolvedArgs := append([]string{}, commandArgs...)
			if len(args) > 1 {
				if resolvedCommand != "" {
					return fmt.Errorf("command was provided both by --command and positional argv; use one form")
				}
				resolvedCommand = args[1]
				resolvedArgs = append([]string{}, args[2:]...)
			}
			server := downstreammcp.Server{
				Name:        name,
				Transport:   finalTransport,
				Command:     resolvedCommand,
				Args:        resolvedArgs,
				EnvRefs:     envRefs,
				URL:         strings.TrimSpace(url),
				HeaderRefs:  headerRefs,
				TimeoutMS:   timeoutMS,
				MaxBytes:    maxBytes,
				Description: strings.TrimSpace(description),
			}
			if err := validateAddCommandFlags(cmd, server); err != nil {
				return err
			}
			if err := server.Validate(); err != nil {
				return err
			}
			return mutateDownstreamMCPServer(cmd.Context(), server, replace, dryRun)
		},
	}
	cmd.Flags().StringVar(&transport, "transport", "", "Transport: command, sse, http, streamable-http")
	cmd.Flags().StringVar(&command, "command", "", "Executable for command transport")
	cmd.Flags().StringArrayVar(&commandArgs, "arg", nil, "Command argument (repeatable)")
	cmd.Flags().StringVar(&url, "url", "", "Endpoint URL for sse or streamable-http transport")
	cmd.Flags().IntVar(&timeoutMS, "timeout-ms", downstreammcp.DefaultTimeoutMS, "Per-call timeout in milliseconds")
	cmd.Flags().IntVar(&maxBytes, "max-bytes", downstreammcp.DefaultMaxBytes, "Maximum returned content bytes")
	cmd.Flags().StringVar(&description, "description", "", "Human-readable server description")
	cmd.Flags().BoolVar(&replace, "replace", false, "Replace existing server with same name")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show outcome without writing state")
	cmd.Flags().StringArrayVar(&envFrom, "env-from", nil, "Command env reference assignment KEY=ENV_VAR (repeatable)")
	cmd.Flags().StringArrayVar(&headerFrom, "header-from", nil, "HTTP header reference assignment HEADER=ENV_VAR (repeatable)")
	cmd.Flags().StringArrayVar(&legacyEnvRef, "env-ref", nil, "Deprecated alias for --env-from")
	cmd.Flags().StringArrayVar(&legacyHeaderRef, "header-ref", nil, "Deprecated alias for --header-from")
	_ = cmd.Flags().MarkHidden("env-ref")
	_ = cmd.Flags().MarkHidden("header-ref")
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
	var description string
	var envFrom []string
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
			envRefs, err := parseReferenceAssignments(envFrom, "--env-from")
			if err != nil {
				return err
			}
			server := downstreammcp.Server{
				Name:        args[0],
				Transport:   downstreammcp.TransportCommand,
				Command:     args[1],
				Args:        append([]string{}, args[2:]...),
				EnvRefs:     envRefs,
				TimeoutMS:   timeoutMS,
				MaxBytes:    maxBytes,
				Description: strings.TrimSpace(description),
			}
			if err := server.Validate(); err != nil {
				return err
			}
			return mutateDownstreamMCPServer(context.Background(), server, true, false)
		},
	}
	cmd.Flags().IntVar(&timeoutMS, "timeout-ms", downstreammcp.DefaultTimeoutMS, "Per-call timeout in milliseconds")
	cmd.Flags().IntVar(&maxBytes, "max-bytes", downstreammcp.DefaultMaxBytes, "Maximum returned content bytes")
	cmd.Flags().StringVar(&description, "description", "", "Human-readable server description")
	cmd.Flags().StringArrayVar(&envFrom, "env-from", nil, "Command env reference assignment KEY=ENV_VAR (repeatable)")
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
	var description string
	var headerFrom []string
	cmd := &cobra.Command{
		Use:   "add-sse <name> <url>",
		Short: "Add a downstream MCP server using SSE transport",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			headerRefs, err := parseReferenceAssignments(headerFrom, "--header-from")
			if err != nil {
				return err
			}
			server := downstreammcp.Server{
				Name:        args[0],
				Transport:   downstreammcp.TransportSSE,
				URL:         args[1],
				HeaderRefs:  headerRefs,
				TimeoutMS:   timeoutMS,
				MaxBytes:    maxBytes,
				Description: strings.TrimSpace(description),
			}
			if err := server.Validate(); err != nil {
				return err
			}
			return mutateDownstreamMCPServer(context.Background(), server, true, false)
		},
	}
	cmd.Flags().IntVar(&timeoutMS, "timeout-ms", downstreammcp.DefaultTimeoutMS, "Per-call timeout in milliseconds")
	cmd.Flags().IntVar(&maxBytes, "max-bytes", downstreammcp.DefaultMaxBytes, "Maximum returned content bytes")
	cmd.Flags().StringVar(&description, "description", "", "Human-readable server description")
	cmd.Flags().StringArrayVar(&headerFrom, "header-from", nil, "HTTP header reference assignment HEADER=ENV_VAR (repeatable)")
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

func parseReferenceAssignments(assignments []string, flagName string) (map[string]string, error) {
	out := map[string]string{}
	for _, assignment := range assignments {
		parts := strings.SplitN(assignment, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid %s assignment %q (expected KEY=ENV_VAR)", flagName, assignment)
		}
		key := strings.TrimSpace(parts[0])
		ref := strings.TrimSpace(parts[1])
		if key == "" || ref == "" {
			return nil, fmt.Errorf("invalid %s assignment %q (expected KEY=ENV_VAR)", flagName, assignment)
		}
		out[key] = ref
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func resolveAddTransport(rawTransport, rawURL string) (string, error) {
	transport := strings.TrimSpace(strings.ToLower(rawTransport))
	if transport == "http" {
		transport = downstreammcp.TransportStreamableHTTP
	}
	if transport == "" {
		if rawURL != "" {
			return downstreammcp.TransportStreamableHTTP, nil
		}
		return downstreammcp.TransportCommand, nil
	}
	switch transport {
	case downstreammcp.TransportCommand, downstreammcp.TransportSSE, downstreammcp.TransportStreamableHTTP:
		return transport, nil
	default:
		return "", fmt.Errorf("unsupported transport %q", rawTransport)
	}
}

func validateAddCommandFlags(cmd *cobra.Command, server downstreammcp.Server) error {
	if cmd.Flags().Changed("timeout-ms") && server.TimeoutMS <= 0 {
		return fmt.Errorf("--timeout-ms must be > 0")
	}
	if cmd.Flags().Changed("max-bytes") && server.MaxBytes <= 0 {
		return fmt.Errorf("--max-bytes must be > 0")
	}
	switch server.Transport {
	case downstreammcp.TransportCommand:
		if strings.TrimSpace(server.URL) != "" {
			return fmt.Errorf("--url is not valid with command transport")
		}
		if len(server.HeaderRefs) > 0 {
			return fmt.Errorf("--header-from is not valid with command transport")
		}
		if strings.TrimSpace(server.Command) == "" {
			return fmt.Errorf("command transport requires executable command (use --command or argv after --)")
		}
	case downstreammcp.TransportSSE, downstreammcp.TransportStreamableHTTP:
		if strings.TrimSpace(server.Command) != "" || len(server.Args) > 0 {
			return fmt.Errorf("URL-based transports do not accept command argv")
		}
		if len(server.EnvRefs) > 0 {
			return fmt.Errorf("--env-from is not valid with URL-based transports")
		}
		if err := validateAbsoluteHTTPURL(server.URL); err != nil {
			return err
		}
	}
	return nil
}

func validateAbsoluteHTTPURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid --url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("--url must use http or https scheme")
	}
	if parsed.Host == "" {
		return fmt.Errorf("--url must be absolute")
	}
	if filepath.IsAbs(raw) {
		return fmt.Errorf("--url must be an absolute URL, not a path")
	}
	return nil
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
