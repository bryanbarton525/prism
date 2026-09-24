package cli

import (
	"encoding/json"
	"fmt"

	prismbundle "github.com/bryanbarton525/prism"
	"github.com/bryanbarton525/prism/internal/buildinfo"
	"github.com/spf13/cobra"
)

type versionOutput struct {
	PrismVersion  string `json:"prism_version"`
	BundleVersion string `json:"bundle_version"`
	BundleDigest  string `json:"bundle_digest"`
	Revision      string `json:"revision,omitempty"`
	Dirty         bool   `json:"dirty"`
}

func currentVersionOutput() versionOutput {
	info := buildinfo.Current()
	return versionOutput{
		PrismVersion:  info.Version,
		BundleVersion: info.Version,
		BundleDigest:  prismbundle.BundleDigest(),
		Revision:      info.Revision,
		Dirty:         info.Dirty,
	}
}

func newVersionCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show Prism and embedded bundle versions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := currentVersionOutput()
			if jsonOut || gf.jsonOut {
				data, err := json.MarshalIndent(out, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Prism:  %s\nBundle: %s\nDigest: %s\n", out.PrismVersion, out.BundleVersion, out.BundleDigest)
			if out.Revision != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Commit: %s\n", out.Revision)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	return cmd
}
