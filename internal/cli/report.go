package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/bundles"
	"github.com/bryanbarton525/prism/internal/events"
	"github.com/bryanbarton525/prism/internal/reports"
	"github.com/bryanbarton525/prism/internal/skill"
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
	cmd.AddCommand(newReportSkillsCmd())
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
				_ = cw.Write([]string{"id", "version", "channel", "owner", "risk_level", "deprecation_status", "installed_at"})
				for _, b := range state.Bundles {
					_ = cw.Write([]string{b.ID, b.Version, b.Channel, b.Owner, b.RiskLevel, b.DeprecationStatus, b.InstalledAt})
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

func newReportSkillsCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Generate skill health report",
		RunE: func(_ *cobra.Command, _ []string) error {
			items := reportSkillHealth(gf.skillsDirOrDefault())
			switch format {
			case "json":
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(items)
			case "csv":
				cw := csv.NewWriter(os.Stdout)
				_ = cw.Write([]string{"name", "ok", "chars", "evals", "errors", "warnings"})
				for _, item := range items {
					_ = cw.Write([]string{item.Name, fmt.Sprint(item.OK), fmt.Sprint(item.Chars), fmt.Sprint(item.Evals), strings.Join(item.Errors, "; "), strings.Join(item.Warnings, "; ")})
				}
				cw.Flush()
				return cw.Error()
			case "markdown":
				fmt.Println("# Prism Skill Health Report")
				fmt.Println()
				fmt.Println("| Skill | Status | Chars | Evals | Notes |")
				fmt.Println("| --- | --- | ---: | ---: | --- |")
				for _, item := range items {
					status := "ok"
					if !item.OK {
						status = "fail"
					}
					notes := strings.Join(append(item.Errors, item.Warnings...), "; ")
					fmt.Printf("| %s | %s | %d | %d | %s |\n", item.Name, status, item.Chars, item.Evals, notes)
				}
				return nil
			default:
				return fmt.Errorf("--format must be markdown, json, or csv")
			}
		},
	}
	cmd.Flags().StringVar(&format, "format", "markdown", "Format: markdown, json, or csv")
	return cmd
}

type reportSkill struct {
	Name     string   `json:"name"`
	OK       bool     `json:"ok"`
	Chars    int      `json:"chars"`
	Evals    int      `json:"evals,omitempty"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

func reportSkillHealth(root string) []reportSkill {
	entries, err := os.ReadDir(root)
	if err != nil {
		return []reportSkill{{Name: root, OK: false, Errors: []string{err.Error()}}}
	}
	fsys := os.DirFS(root)
	out := make([]reportSkill, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		item := reportSkill{Name: name, OK: true}
		data, err := fs.ReadFile(fsys, filepath.ToSlash(filepath.Join(name, "SKILL.md")))
		if err != nil {
			item.OK = false
			item.Errors = append(item.Errors, err.Error())
			out = append(out, item)
			continue
		}
		item.Chars = len(data)
		if _, err := skill.LoadDir(fsys, name); err != nil {
			item.OK = false
			item.Errors = append(item.Errors, err.Error())
		}
		if err := skill.ValidateStructure(fsys, name); err != nil {
			item.OK = false
			item.Errors = append(item.Errors, err.Error())
		}
		count, err := skill.ValidateEvals(fsys, name)
		if err != nil {
			item.OK = false
			item.Errors = append(item.Errors, err.Error())
		} else {
			item.Evals = count
		}
		if !strings.Contains(string(data), "##") {
			item.Warnings = append(item.Warnings, "no markdown section headings")
		}
		out = append(out, item)
	}
	return out
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
