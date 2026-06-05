package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	internalgraph "github.com/bryanbarton525/prism/internal/graph"
)

func newGraphCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Validate and run bounded Prism graphs",
	}
	cmd.AddCommand(newGraphValidateCmd())
	cmd.AddCommand(newGraphRunCmd())
	return cmd
}

func newGraphValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <graph.yaml>",
		Short: "Validate a graph definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			def, err := internalgraph.Load(args[0])
			if err != nil {
				return err
			}
			res := internalgraph.Validate(def)
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(res); err != nil {
				return err
			}
			if !res.Valid {
				return fmt.Errorf("graph validation failed")
			}
			return nil
		},
	}
}

func newGraphRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <graph.yaml>",
		Short: "Run a bounded graph definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			def, err := internalgraph.Load(args[0])
			if err != nil {
				return err
			}
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
				return err
			}
			defer cleanup()
			res, err := internalgraph.RunWithOptions(cmd.Context(), runner, def, internalgraph.RunOptions{
				Source:    "cli",
				Policy:    policyEngine,
				EventSink: eventSink,
			})
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(res)
		},
	}
}
