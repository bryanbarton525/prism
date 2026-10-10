package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bryanbarton525/prism/internal/buildinfo"
	"github.com/bryanbarton525/prism/internal/selfupdate"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	var check bool
	var version string
	cmd := &cobra.Command{
		Use: "update", Short: "Update this executable from an official Prism release",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			executable, err := os.Executable()
			if err != nil {
				return err
			}
			result, err := selfupdate.New().Run(cmd.Context(), executable, buildinfo.Current().Version, version, check)
			if err != nil {
				return err
			}
			if gf.jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Current: %s\nRelease: %s\nExecutable: %s\n", result.Current, result.Release, result.Path)
			switch {
			case result.Updated:
				fmt.Fprintln(cmd.OutOrStdout(), "Updated. Reconnect running Prism MCP sessions to load the new version.")
			case result.Available:
				fmt.Fprintln(cmd.OutOrStdout(), "Update available. Run prism update to install it.")
			default:
				fmt.Fprintln(cmd.OutOrStdout(), "Already up to date.")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "Check the release without changing the executable")
	cmd.Flags().StringVar(&version, "version", "", "Install a specific release tag (also permits rollback)")
	return cmd
}
