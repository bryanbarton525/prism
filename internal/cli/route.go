package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/router"
)

func newRouteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route",
		Short: "Suggest deterministic Prism offload routes",
	}
	cmd.AddCommand(newRouteSuggestCmd())
	cmd.AddCommand(newRouteExplainCmd())
	return cmd
}

func newRouteSuggestCmd() *cobra.Command {
	var task string
	cmd := &cobra.Command{
		Use:   "suggest",
		Short: "Suggest an agent and skills for a task",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if task == "" {
				var err error
				task, err = resolveTask(runFlags{stdin: true})
				if err != nil {
					return err
				}
			}
			runner, cleanup, err := newRunner(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()
			policyEngine, err := configuredPolicyEngine()
			if err != nil {
				return err
			}
			res, err := router.New(runner, policyEngine).Suggest(cmd.Context(), router.Request{Task: task, Source: "cli"})
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(res)
		},
	}
	cmd.Flags().StringVar(&task, "task", "", "Task text to route")
	return cmd
}

func newRouteExplainCmd() *cobra.Command {
	cmd := newRouteSuggestCmd()
	cmd.Use = "explain"
	cmd.Short = "Explain the deterministic route for a task"
	return cmd
}

func printRouteHuman(res router.Result) {
	fmt.Printf("Agent: %s\nSkills: %v\nReason: %s\nRisk: %s\nPolicy: %s\n",
		res.AgentID, res.SkillNames, res.Reason, res.Risk, res.PolicyDecision.Decision)
}
