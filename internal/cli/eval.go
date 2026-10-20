package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bryanbarton525/prism/internal/eval"
	"github.com/bryanbarton525/prism/internal/llm"
	"github.com/spf13/cobra"
)

func init() { rootCmd.AddCommand(newEvalCmd()) }
func newEvalCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "eval", Short: "Compare model behavior on repeatable MCP workflows"}
	cmd.AddCommand(newEvalRunCmd(false), newEvalRunCmd(true))
	baseline := &cobra.Command{Use: "baseline", Short: "Manage explicitly approved evaluation baselines"}
	var model, output string
	create := &cobra.Command{Use: "create RUN.json", Short: "Approve a fully passing, consistent run as a baseline", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		var report eval.Report
		if err := eval.ReadJSON(args[0], &report); err != nil {
			return err
		}
		b, err := eval.CreateBaseline(report, model)
		if err != nil {
			return err
		}
		if err := eval.WriteJSON(output, b); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Baseline saved:", output)
		return nil
	}}
	create.Flags().StringVar(&model, "model", "", "Model ID from the run to approve")
	create.Flags().StringVar(&output, "output", "", "New baseline JSON path")
	_ = create.MarkFlagRequired("model")
	_ = create.MarkFlagRequired("output")
	baseline.AddCommand(create)
	cmd.AddCommand(baseline)
	var compareModel string
	compare := &cobra.Command{Use: "compare BASELINE.json RUN.json", Short: "Check an existing run for functional or consistency regressions", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		var b eval.Baseline
		var r eval.Report
		if err := eval.ReadJSON(args[0], &b); err != nil {
			return err
		}
		if err := eval.ReadJSON(args[1], &r); err != nil {
			return err
		}
		return printComparison(cmd, b, r, compareModel)
	}}
	compare.Flags().StringVar(&compareModel, "model", "", "Candidate model ID from the run")
	_ = compare.MarkFlagRequired("model")
	cmd.AddCommand(compare)
	var dir string
	export := &cobra.Command{Use: "export RUN.json", Short: "Export traces, scores and metadata as a local comparison dataset", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		var r eval.Report
		if err := eval.ReadJSON(args[0], &r); err != nil {
			return err
		}
		if err := eval.Export(r, dir); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Dataset saved:", dir)
		return nil
	}}
	export.Flags().StringVar(&dir, "output", "", "New dataset directory")
	_ = export.MarkFlagRequired("output")
	cmd.AddCommand(export)
	return cmd
}
func newEvalRunCmd(test bool) *cobra.Command {
	var suitePath, output, baselinePath, layaURL string
	var models []string
	var repeat, maxTokens int
	var timeout time.Duration
	var temperature float64
	use, short := "run", "Run repeated model comparisons against fixed MCP responses"
	if test {
		use, short = "test", "Run a candidate model and fail on baseline regressions"
	}
	cmd := &cobra.Command{Use: use, Short: short, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		suite, err := eval.LoadSuite(suitePath)
		if err != nil {
			return err
		}
		runtimeConfig := effectivePrimary()
		if len(models) == 0 {
			models = []string{runtimeConfig.Model}
		}
		for _, model := range models {
			if strings.TrimSpace(model) == "" {
				return fmt.Errorf("no model selected; pass --model with an exact served model ID")
			}
		}
		if test && len(models) != 1 {
			return fmt.Errorf("eval test requires exactly one candidate --model")
		}
		var b eval.Baseline
		if test {
			if err := eval.ReadJSON(baselinePath, &b); err != nil {
				return err
			}
			if b.SuiteHash != eval.Hash(suite) || b.Grader != eval.GraderVersion {
				return fmt.Errorf("baseline does not match suite or grader")
			}
			if !cmd.Flags().Changed("repeat") {
				for _, score := range b.Cases {
					if repeat < score.Trials {
						repeat = score.Trials
					}
				}
			}
			if !cmd.Flags().Changed("temperature") {
				temperature = b.Generation.Temperature
			}
			if !cmd.Flags().Changed("max-tokens") {
				maxTokens = b.Generation.MaxTokens
			}
		}
		if test {
			if b.LayaEnabled != (layaURL != "") {
				return fmt.Errorf("baseline and candidate Laya modes differ; select --laya-url for a Laya baseline")
			}
			if err := eval.CheckBaseline(b, suite, eval.Generation{Temperature: temperature, MaxTokens: maxTokens}, repeat); err != nil {
				return err
			}
		}
		if _, err := os.Stat(output); err == nil {
			return fmt.Errorf("output already exists: %s", output)
		} else if !os.IsNotExist(err) {
			return err
		}
		// Evaluation supplies a different model on each request. Provider-level
		// configured model overrides must not pin the entire comparison matrix.
		runtimeConfig.Model = ""
		rt, err := llm.NewRuntime(runtimeConfig)
		if err != nil {
			return err
		}
		journal, err := os.OpenFile(output+".trials.jsonl", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer journal.Close()
		encoder := json.NewEncoder(journal)
		report, runErr := eval.Run(cmd.Context(), suite, rt, eval.Options{LayaURL: layaURL, Models: models, Repeat: repeat, Timeout: timeout, Generation: eval.Generation{Temperature: temperature, MaxTokens: maxTokens}, OnTrial: func(t eval.Trial) error {
			fmt.Fprintf(cmd.ErrOrStderr(), "%s %s trial %d: passed=%t (%dms)\n", t.Model, t.CaseID, t.Repetition, t.Grade.Passed, t.DurationMS)
			if err := encoder.Encode(t); err != nil {
				return err
			}
			return journal.Sync()
		}})
		if err := eval.WriteJSON(output, report); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Evaluation saved:", output)
		if runErr != nil {
			return runErr
		}
		if test {
			return printComparison(cmd, b, report, models[0])
		}
		for _, model := range models {
			scores := eval.Scores(report, model)
			for _, c := range suite.Cases {
				score := scores[c.ID]
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %d/%d passed, %.1f%% semantic consistency\n", model, c.ID, score.Passed, score.Trials, 100*score.Consistency)
			}
		}
		return nil
	}}
	cmd.Flags().StringVar(&layaURL, "laya-url", "", "Enable Laya English recommendations (lexical shortlist, 500ms budget); record and grade application")
	cmd.Flags().StringVar(&suitePath, "suite", "", "Versioned YAML replay suite (default: bundled mcp-response-v1)")
	cmd.Flags().StringSliceVar(&models, "model", nil, "Model ID(s); repeat flag or use comma-separated IDs (default: configured model)")
	cmd.Flags().IntVar(&repeat, "repeat", 3, "Trials per case and model")
	cmd.Flags().DurationVar(&timeout, "timeout", 90*time.Second, "Timeout per complete agent trial")
	cmd.Flags().Float64Var(&temperature, "temperature", 0.1, "Temperature for every model call")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 1024, "Maximum output tokens per model call")
	cmd.Flags().StringVar(&output, "output", "", "New report JSON path (plus incremental .trials.jsonl)")
	_ = cmd.MarkFlagRequired("output")
	if test {
		cmd.Flags().StringVar(&baselinePath, "baseline", "", "Approved baseline JSON path")
		_ = cmd.MarkFlagRequired("baseline")
	}
	return cmd
}
func printComparison(cmd *cobra.Command, b eval.Baseline, r eval.Report, model string) error {
	regressions, err := eval.Compare(b, r, model)
	if err != nil {
		return err
	}
	if len(regressions) > 0 {
		return fmt.Errorf("evaluation regressions:\n%s", strings.Join(regressions, "\n"))
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Baseline checks passed for", model)
	return nil
}
