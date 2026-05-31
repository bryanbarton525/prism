package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/benchmark"
)

func newBenchmarkProjectCmd() *cobra.Command {
	var jsonOut bool
	var writeDocs bool

	cmd := &cobra.Command{
		Use:   "project",
		Short: "Project monthly orchestrator cost savings from committed benchmark results",
		Long: `Uses testdata/benchmarks/results.yaml and scale-profiles.yaml to estimate
monthly and annual net savings for solo, team, and enterprise usage profiles.

The headline orchestrator showcase uses the live todo-spa-build benchmark
(6,191 -> 363 orchestrator input tokens) at 1 task/day across GPT/Claude rates.

Re-run live benchmarks and update results.yaml to refresh projections:
  prism benchmark run todo-spa-build
  prism benchmark project --write
  prism benchmark run homelab-release-incident
  prism benchmark run homelab-release-incident-at-scale`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root := gf.rootDir
			if root == "" || root == "." {
				var err error
				root, err = benchmark.FindRepoRoot()
				if err != nil {
					return err
				}
			}

			if writeDocs {
				report, err := benchmark.WriteShowcaseDocs(root)
				if err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "Updated README.md, docs/benchmark-scale.md, docs/benchmark-showcase.md, and testdata/benchmarks/scenarios/todo-spa-build/live-results.yaml from live %s data.\n",
					report.Showcase.ScenarioID)
				if jsonOut || gf.jsonOut {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(report)
				}
				fmt.Println(report.Markdown)
				return nil
			}

			report, err := benchmark.ProjectMonthly(root)
			if err != nil {
				return err
			}

			if jsonOut || gf.jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}
			fmt.Println(report.Markdown)
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON projection report")
	cmd.Flags().BoolVar(&writeDocs, "write", false, "Regenerate README/docs showcase tables from results.yaml")
	return cmd
}
