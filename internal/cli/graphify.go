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
	cmd := &cobra.Command{Use: "graphify", Short: "Configure and inspect explicit Graphify runtime bindings"}
	cmd.AddCommand(newGraphifyDoctorCmd(), newGraphifySetupCmd(), newGraphifyBindCmd(), newGraphifyRemoveCmd())
	return cmd
}

func newGraphifyBindCmd() *cobra.Command {
	cmd := newGraphifySetupCmd()
	cmd.Use = "bind"
	cmd.Short = "Compatibility alias for graphify setup"
	cmd.Long = "Compatibility alias for graphify setup. It records configuration only; it never installs Graphify or builds an index."
	return cmd
}

func newGraphifySetupCmd() *cobra.Command {
	var workspace, indexPath, upstreamVersion, schemaVersion, fingerprint, server string
	var kind, executable, environment, environmentVersion string
	var approve, dryRun bool
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Record an operator-approved Graphify endpoint and workspace binding",
		Long: `Record only an explicit, operator-approved Graphify configuration.

This command never downloads or installs Graphify, starts an endpoint, or builds
an index. Create and verify the index outside Prism first, then choose exactly
one endpoint kind:
  local        user-managed local executable
  self-hosted  an operator-managed downstream MCP endpoint
  managed      a named, version-pinned managed downstream MCP endpoint`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !approve {
				return fmt.Errorf("refusing Graphify setup without --approve; Prism never installs Graphify or builds indexes automatically")
			}
			workspace, err := filepath.Abs(workspace)
			if err != nil {
				return err
			}
			indexPath, err = filepath.Abs(indexPath)
			if err != nil {
				return err
			}
			endpoint := &graphify.Endpoint{
				Server: server, Kind: graphify.EndpointKind(kind), Executable: executable,
				Environment: environment, EnvironmentVersion: environmentVersion,
			}
			if err := endpoint.Validate(); err != nil {
				return err
			}
			if upstreamVersion != graphify.PinnedUpstreamVersion {
				return fmt.Errorf("Graphify upstream version %q is unsupported; Prism requires pinned release %q", upstreamVersion, graphify.PinnedUpstreamVersion)
			}
			if schemaVersion != graphify.PinnedContractID {
				return fmt.Errorf("Graphify tool contract %q is unsupported; Prism requires %q", schemaVersion, graphify.PinnedContractID)
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Would record operator-approved Graphify %s endpoint %q and bind %s to %s. No executable, endpoint, or index will be started or changed.\n", endpoint.Kind, endpoint.Server, indexPath, workspace)
				return nil
			}
			cfg, err := graphify.Load(filepath.Join(gf.stateDir, "graphify.yaml"))
			if err != nil {
				return err
			}
			cfg.Binding = &graphify.Binding{
				Workspace: workspace, IndexPath: indexPath, UpstreamVersion: upstreamVersion,
				SchemaVersion: schemaVersion, GenerationFingerprint: fingerprint,
			}
			cfg.Endpoint = endpoint
			cfg.OperatorApproved = true
			if err := graphify.Save(filepath.Join(gf.stateDir, "graphify.yaml"), cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Recorded operator-approved Graphify %s endpoint %q and bound index %s to workspace %s. Prism did not install Graphify or build the index.\n", endpoint.Kind, endpoint.Server, indexPath, workspace)
			return nil
		},
	}
	cmd.Flags().StringVar(&workspace, "workspace", "", "Absolute or relative workspace path")
	cmd.Flags().StringVar(&indexPath, "index", "", "Absolute or relative Graphify graph.json path")
	cmd.Flags().StringVar(&upstreamVersion, "upstream-version", graphify.PinnedUpstreamVersion, "Pinned Graphify upstream version")
	cmd.Flags().StringVar(&schemaVersion, "schema-version", graphify.PinnedContractID, "Pinned Prism Graphify MCP contract ID")
	cmd.Flags().StringVar(&fingerprint, "fingerprint", "", "Deterministic source-generation fingerprint")
	cmd.Flags().StringVar(&server, "server", "", "Explicit downstream MCP server that serves this Graphify index")
	cmd.Flags().StringVar(&kind, "endpoint-kind", "", "Endpoint ownership: local, self-hosted, or managed")
	cmd.Flags().StringVar(&executable, "executable", "", "Approved local Graphify executable (required for --endpoint-kind local)")
	cmd.Flags().StringVar(&environment, "environment", "", "Managed environment name (required for --endpoint-kind managed)")
	cmd.Flags().StringVar(&environmentVersion, "environment-version", "", "Managed environment version pin (required for --endpoint-kind managed)")
	cmd.Flags().BoolVar(&approve, "approve", false, "Confirm that this exact Graphify configuration is operator-approved")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview configuration only; never write or run Graphify")
	_ = cmd.MarkFlagRequired("workspace")
	_ = cmd.MarkFlagRequired("index")
	_ = cmd.MarkFlagRequired("fingerprint")
	_ = cmd.MarkFlagRequired("server")
	_ = cmd.MarkFlagRequired("endpoint-kind")
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
			if cfg.Endpoint != nil {
				wasReady := ready.Ready
				state, err := configuredDownstreamMCPState()
				if err != nil {
					return err
				}
				if server, ok := state.Get(cfg.Endpoint.Server); !ok {
					ready.Ready = false
					ready.Checks = append(ready.Checks, graphify.Check{
						Name: "endpoint_server", Message: fmt.Sprintf("configured Graphify endpoint %q is not registered", cfg.Endpoint.Server),
					})
					if wasReady {
						ready.Message = fmt.Sprintf("configured Graphify endpoint %q is not registered", cfg.Endpoint.Server)
					}
				} else if err := cfg.Endpoint.ValidateServer(server.Name, server.Transport, server.Command); err != nil {
					ready.Ready = false
					ready.Checks = append(ready.Checks, graphify.Check{Name: "endpoint_server", Message: err.Error()})
					if wasReady {
						ready.Message = err.Error()
					}
				} else {
					ready.Checks = append(ready.Checks, graphify.Check{
						Name: "endpoint_server", Ready: true,
						Message: fmt.Sprintf("configured Graphify endpoint %q is registered (%s transport; not contacted by doctor)", server.Name, server.Transport),
					})
				}
			}
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

func newGraphifyRemoveCmd() *cobra.Command {
	var approve, dryRun bool
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove Prism's Graphify configuration without touching user resources",
		Long: `Remove only Prism's graphify.yaml configuration. This never removes a
user-managed executable, index, endpoint, or downstream MCP server.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !approve {
				return fmt.Errorf("refusing to remove Graphify configuration without --approve")
			}
			path := filepath.Join(gf.stateDir, "graphify.yaml")
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Would remove Prism Graphify configuration %s only. User-managed executables, indexes, endpoints, and MCP servers are preserved.\n", path)
				return nil
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove Graphify configuration: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Removed Prism Graphify configuration only. User-managed executables, indexes, endpoints, and MCP servers were preserved.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&approve, "approve", false, "Confirm removal of Prism's Graphify configuration")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview removal without writing")
	return cmd
}
