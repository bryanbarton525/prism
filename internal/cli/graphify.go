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
	cmd.AddCommand(newGraphifyDoctorCmd(), newGraphifyBindCmd())
	return cmd
}

func newGraphifyBindCmd() *cobra.Command {
	var workspace, indexPath, upstreamVersion, schemaVersion, fingerprint string
	cmd := &cobra.Command{
		Use:   "bind",
		Short: "Explicitly bind a Graphify index to one workspace generation",
		RunE: func(cmd *cobra.Command, _ []string) error {
			workspace, err := filepath.Abs(workspace)
			if err != nil {
				return err
			}
			indexPath, err = filepath.Abs(indexPath)
			if err != nil {
				return err
			}
			cfg, err := graphify.Load(filepath.Join(gf.stateDir, "graphify.yaml"))
			if err != nil {
				return err
			}
			cfg.Binding = &graphify.Binding{
				Workspace: workspace, IndexPath: indexPath, UpstreamVersion: upstreamVersion,
				SchemaVersion: schemaVersion, GenerationFingerprint: fingerprint,
			}
			if err := graphify.Save(filepath.Join(gf.stateDir, "graphify.yaml"), cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Bound Graphify index %s to workspace %s\n", indexPath, workspace)
			return nil
		},
	}
	cmd.Flags().StringVar(&workspace, "workspace", "", "Absolute or relative workspace path")
	cmd.Flags().StringVar(&indexPath, "index", "", "Absolute or relative Graphify index path")
	cmd.Flags().StringVar(&upstreamVersion, "upstream-version", "", "Pinned Graphify upstream version")
	cmd.Flags().StringVar(&schemaVersion, "schema-version", "", "Pinned Graphify tool schema version")
	cmd.Flags().StringVar(&fingerprint, "fingerprint", "", "Deterministic source-generation fingerprint")
	_ = cmd.MarkFlagRequired("workspace")
	_ = cmd.MarkFlagRequired("index")
	_ = cmd.MarkFlagRequired("upstream-version")
	_ = cmd.MarkFlagRequired("schema-version")
	_ = cmd.MarkFlagRequired("fingerprint")
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
