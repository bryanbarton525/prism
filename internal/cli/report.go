package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/bundles"
	"github.com/bryanbarton525/prism/internal/events"
	"github.com/bryanbarton525/prism/internal/reports"
)

func newReportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate Prism usage reports",
	}
	cmd.AddCommand(newReportEventsCmd("usage"))
	cmd.AddCommand(newReportEventsCmd("savings"))
	cmd.AddCommand(newReportEventsCmd("adoption"))
	cmd.AddCommand(newReportBundlesCmd())
	return cmd
}

func newReportEventsCmd(kind string) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   kind,
		Short: "Generate " + kind + " report",
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := events.Open(eventStorePath())
			if err != nil {
				return err
			}
			defer store.Close()
			sum, err := store.Summary(cmd.Context())
			if err != nil {
				return err
			}
			if format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(sum)
			}
			if format == "csv" {
				return writeSummaryCSV(os.Stdout, kind, sum)
			}
			if format != "markdown" {
				return fmt.Errorf("--format must be markdown, json, or csv")
			}
			switch kind {
			case "usage":
				fmt.Print(reports.UsageMarkdown(sum))
			case "savings":
				fmt.Print(reports.SavingsMarkdown(sum))
			case "adoption":
				fmt.Print(reports.AdoptionMarkdown(sum))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "markdown", "Format: markdown, json, or csv")
	return cmd
}

func newReportBundlesCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "bundles",
		Short: "Generate bundle report",
		RunE: func(_ *cobra.Command, _ []string) error {
			state, err := bundles.Load(installedBundlesPath())
			if err != nil {
				return err
			}
			if format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(state)
			}
			if format == "csv" {
				cw := csv.NewWriter(os.Stdout)
				_ = cw.Write([]string{"id", "version", "channel", "owner", "risk_level", "installed_at"})
				for _, b := range state.Bundles {
					_ = cw.Write([]string{b.ID, b.Version, b.Channel, b.Owner, b.RiskLevel, b.InstalledAt})
				}
				cw.Flush()
				return cw.Error()
			}
			if format != "markdown" {
				return fmt.Errorf("--format must be markdown, json, or csv")
			}
			fmt.Print(reports.BundlesMarkdown(len(state.Bundles)))
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "markdown", "Format: markdown, json, or csv")
	return cmd
}

func writeSummaryCSV(w *os.File, kind string, sum events.Summary) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"report", "total", "context_budget_warnings", "policy_denials", "validation_failures", "timeouts", "graph_executions", "prompt_tokens_estimate", "completion_tokens_estimate"}); err != nil {
		return err
	}
	if err := cw.Write([]string{
		kind,
		fmt.Sprint(sum.Total),
		fmt.Sprint(sum.ContextBudgetWarnings),
		fmt.Sprint(sum.PolicyDenials),
		fmt.Sprint(sum.ValidationFailures),
		fmt.Sprint(sum.Timeouts),
		fmt.Sprint(sum.GraphExecutions),
		fmt.Sprint(sum.PromptTokensEstimate),
		fmt.Sprint(sum.CompletionTokensEstimate),
	}); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}
