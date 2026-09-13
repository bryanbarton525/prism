package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/graphify"
)

func newGraphifyCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "graphify", Short: "Inspect Graphify runtime bindings"}
	cmd.AddCommand(newGraphifyDoctorCmd())
	return cmd
}

func newGraphifyDoctorCmd() *cobra.Command {
	var workspace, fingerprint string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check the configured Graphify index without changing it",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if workspace == "" {
				var err error
				workspace, err = os.Getwd()
				if err != nil {
					return err
				}
			}
			cfg, err := graphify.Load(filepath.Join(gf.stateDir, "graphify.yaml"))
			if err != nil {
				return err
			}
			ready := graphify.CheckReadiness(cfg, workspace, fingerprint)
			if gf.jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(ready)
			}
			fmt.Fprintln(cmd.OutOrStdout(), ready.Message)
			if !ready.Ready {
				return fmt.Errorf("Graphify is not ready")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace the Graphify index must match")
	cmd.Flags().StringVar(&fingerprint, "fingerprint", "", "Current deterministic workspace generation fingerprint")
	return cmd
}
