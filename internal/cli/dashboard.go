package cli

import (
	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/dashboard"
	"github.com/bryanbarton525/prism/internal/events"
)

func newDashboardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Serve the local Prism dashboard",
	}
	cmd.AddCommand(newDashboardServeCmd())
	return cmd
}

func newDashboardServeCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the local dashboard",
		RunE: func(_ *cobra.Command, _ []string) error {
			store, err := events.Open(eventStorePath())
			if err != nil {
				return err
			}
			defer store.Close()
			return dashboard.Serve(addr, store)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8765", "Address to serve")
	return cmd
}
