package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage and inspect local agent specs",
	}
	cmd.AddCommand(newAgentListCmd())
	cmd.AddCommand(newAgentShowCmd())
	return cmd
}

func newAgentListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available agents",
		Long:  "List all agent specs found in the configured agent directory.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := buildApp(cmd)
			if err != nil {
				return err
			}
			// TODO(milestone-2): load agent registry from a.Config.AgentDir
			fmt.Fprintf(cmd.OutOrStdout(), "agent dir: %s\n", a.Config.AgentDir)
			fmt.Fprintln(cmd.OutOrStdout(), "(no agents loaded yet — registry not implemented)")
			return nil
		},
	}
}

func newAgentShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <agent-id>",
		Short: "Show details for a specific agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO(milestone-2): load and display agent spec from registry
			fmt.Fprintf(cmd.OutOrStdout(), "agent: %s\n", args[0])
			fmt.Fprintln(cmd.OutOrStdout(), "(agent registry not implemented yet)")
			return nil
		},
	}
}
