package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/app"
)

func newRunCmd() *cobra.Command {
	var (
		skillNames []string
		inputFile  string
		fromStdin  bool
		format     string
	)

	cmd := &cobra.Command{
		Use:   "run <agent-id>",
		Short: "Run an agent with a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentID := args[0]

			var task string
			switch {
			case fromStdin:
				data, err := os.ReadFile("/dev/stdin")
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				task = string(data)
			case inputFile != "":
				data, err := os.ReadFile(inputFile)
				if err != nil {
					return fmt.Errorf("reading input file %s: %w", inputFile, err)
				}
				task = string(data)
			default:
				return fmt.Errorf("provide a task via --input <file> or --stdin")
			}

			cfg := buildConfig()
			runner, err := app.New(cfg)
			if err != nil {
				return err
			}

			req := app.RunRequest{
				AgentID:    agentID,
				Task:       task,
				SkillNames: skillNames,
				Format:     format,
			}

			res, err := runner.Run(context.Background(), req)
			if err != nil {
				return err
			}

			if global.jsonOutput || format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			}

			if res.Status != "ok" {
				fmt.Fprintf(os.Stderr, "error: %s\n", res.Error)
				return fmt.Errorf("agent run failed")
			}

			fmt.Printf("# Result: %s\n\n", res.AgentID)
			fmt.Printf("**Model:** %s  **Confidence:** %s  **Duration:** %dms\n\n",
				res.Model, res.Confidence, res.Usage.DurationMs)
			if res.Summary != "" {
				fmt.Printf("## Summary\n\n%s\n\n", res.Summary)
			}
			if res.RawOutput != "" {
				fmt.Printf("## Output\n\n%s\n", res.RawOutput)
			}
			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&skillNames, "skills", "s", nil, "comma-separated skill names to attach (required)")
	cmd.Flags().StringVarP(&inputFile, "input", "i", "", "path to task input file")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "read task from stdin")
	cmd.Flags().StringVar(&format, "format", "markdown", "output format: json or markdown")

	return cmd
}
