package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/events"
)

func newEventsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "events",
		Short: "Inspect local Prism run history",
	}
	cmd.AddCommand(newEventsListCmd())
	cmd.AddCommand(newEventsExportCmd())
	cmd.AddCommand(newEventsSummarizeCmd())
	return cmd
}

func newEventsListCmd() *cobra.Command {
	var limit int
	var status, agent, source string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List stored run events",
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := events.Open(eventStorePath())
			if err != nil {
				return err
			}
			defer store.Close()
			items, err := store.List(cmd.Context(), events.ListOptions{Limit: limit, Status: status, Agent: agent, Source: source})
			if err != nil {
				return err
			}
			if gf.jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(items)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "TIME\tSTATUS\tSOURCE\tAGENT\tSKILLS\tDURATION")
			for _, event := range items {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%dms\n",
					event.Timestamp.Format("2006-01-02 15:04:05"), event.Status, event.Source,
					event.AgentID, len(event.Skills), event.DurationMS)
			}
			return w.Flush()
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum events to list")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status")
	cmd.Flags().StringVar(&agent, "agent", "", "Filter by agent id")
	cmd.Flags().StringVar(&source, "source", "", "Filter by source")
	return cmd
}

func newEventsExportCmd() *cobra.Command {
	var format string
	var limit int
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export stored run events",
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := events.Open(eventStorePath())
			if err != nil {
				return err
			}
			defer store.Close()
			items, err := store.List(cmd.Context(), events.ListOptions{Limit: limit})
			if err != nil {
				return err
			}
			switch format {
			case "json":
				return events.WriteJSON(os.Stdout, items)
			case "csv":
				return events.WriteCSV(os.Stdout, items)
			default:
				return fmt.Errorf("--format must be json or csv")
			}
		},
	}
	cmd.Flags().StringVar(&format, "format", "json", "Export format: json or csv")
	cmd.Flags().IntVar(&limit, "limit", 10000, "Maximum events to export")
	return cmd
}

func newEventsSummarizeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "summarize",
		Short: "Summarize stored run events",
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
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(sum)
		},
	}
}
