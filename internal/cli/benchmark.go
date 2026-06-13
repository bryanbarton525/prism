package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/benchmark"
)

func newBenchmarkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "Run Prism benchmark scenarios",
	}
	cmd.AddCommand(newBenchmarkRunCmd())
	cmd.AddCommand(newBenchmarkProjectCmd())
	return cmd
}

func newBenchmarkRunCmd() *cobra.Command {
	var (
		mock       bool
		jsonOut    bool
		outputPath string
		mockDelay  int
	)

	cmd := &cobra.Command{
		Use:   "run [scenario-id]",
		Short: "Compare orchestrator-only vs Prism-delegated (real Ollama by default)",
		Long: `Runs a golden benchmark scenario twice against a real Ollama server:

  1. orchestrator_only — one large prompt (no MCP): all evidence + skills + constitutions
  2. prism_delegated — eight Prism agent runs + short orchestrator synthesis

Default scenario: homelab-release-incident (all 8 skills).

Requires Ollama running with llama3.1:8b (or pass --model). Use --mock only for offline CI simulation.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scenarioID := "homelab-release-incident"
			if len(args) > 0 {
				scenarioID = args[0]
			}

			root := gf.rootDir
			if root == "" || root == "." {
				var err error
				root, err = benchmark.FindRepoRoot()
				if err != nil {
					return err
				}
			}

			opts := benchmark.RunOptions{
				Mock:          mock,
				MockPerCallMS: mockDelay,
				OllamaHost:    gf.ollamaHost,
			}

			if !mock {
				if err := benchmark.OllamaReachable(cmd.Context(), gf.ollamaHost); err != nil {
					return fmt.Errorf("Ollama required for live benchmark: %w (use --mock for simulation)", err)
				}
			}

			report, err := benchmark.Compare(cmd.Context(), root, scenarioID, opts)
			if err != nil {
				return err
			}

			if jsonOut || gf.jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				if err := enc.Encode(report); err != nil {
					return err
				}
			} else {
				fmt.Println(report.Markdown)
			}

			if outputPath != "" {
				if err := os.WriteFile(outputPath, []byte(report.Markdown), 0o644); err != nil {
					return fmt.Errorf("writing report: %w", err)
				}
				jsonPath := outputPath + ".json"
				data, _ := json.MarshalIndent(report, "", "  ")
				if err := os.WriteFile(jsonPath, data, 0o644); err != nil {
					return fmt.Errorf("writing json: %w", err)
				}
				fmt.Fprintf(os.Stderr, "Wrote %s and %s\n", outputPath, jsonPath)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&mock, "mock", false, "Simulate with canned responses (offline CI only)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON comparison report")
	cmd.Flags().StringVar(&outputPath, "output", "", "Write markdown report (and .json) to this path")
	cmd.Flags().IntVar(&mockDelay, "mock-delay-ms", 750, "Per-delegation delay when --mock is set")
	return cmd
}
