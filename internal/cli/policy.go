package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	internalpolicy "github.com/bryanbarton525/prism/internal/policy"
	policypkg "github.com/bryanbarton525/prism/pkg/policy"
)

func newPolicyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "Validate and explain Prism policy",
	}
	cmd.AddCommand(newPolicyValidateCmd())
	cmd.AddCommand(newPolicyExplainCmd())
	cmd.AddCommand(newPolicyTestCmd())
	return cmd
}

func newPolicyValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <policy.yaml>",
		Short: "Validate a Prism policy file",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			_, err := internalpolicy.Load(args[0])
			if err != nil {
				return err
			}
			fmt.Println("policy: ok")
			return nil
		},
	}
}

func newPolicyExplainCmd() *cobra.Command {
	var skills []string
	var plugins []string
	var source string
	cmd := &cobra.Command{
		Use:   "explain <policy.yaml> <agent-id>",
		Short: "Explain the policy decision for an agent request",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			engine, err := internalpolicy.Load(args[0])
			if err != nil {
				return err
			}
			decision := engine.Explain(policypkg.Request{
				AgentID: args[1],
				Skills:  skills,
				Plugins: plugins,
				Source:  source,
			})
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(decision)
		},
	}
	cmd.Flags().StringSliceVar(&skills, "skills", nil, "Skill names requested")
	cmd.Flags().StringSliceVar(&plugins, "plugins", nil, "Runtime plugins requested")
	cmd.Flags().StringVar(&source, "source", "cli", "Request source")
	return cmd
}

func newPolicyTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test <policy.yaml> <cases.yaml>",
		Short: "Run fixture-driven policy decision tests",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			engine, err := internalpolicy.Load(args[0])
			if err != nil {
				return err
			}
			suite, err := internalpolicy.LoadTestSuite(args[1])
			if err != nil {
				return err
			}
			results := engine.Test(suite)
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(results); err != nil {
				return err
			}
			for _, res := range results {
				if !res.Passed {
					return fmt.Errorf("policy test %q failed: %s", res.Name, res.Error)
				}
			}
			return nil
		},
	}
}
