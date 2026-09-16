package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	prismbundle "github.com/bryanbarton525/prism"
	"github.com/bryanbarton525/prism/internal/downstreammcp"
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
	cmd.Long = "Compatibility alias for graphify setup. Managed dependency installation still requires the explicit --install-managed-environment selection; Prism never builds an index."
	return cmd
}

func newGraphifySetupCmd() *cobra.Command {
	var workspace, indexPath, upstreamVersion, schemaVersion, fingerprint, server string
	var kind, executable, environment, environmentVersion string
	var approve, dryRun bool
	var installManaged bool
	var uvExecutable string
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Record an operator-approved Graphify endpoint and workspace binding",
		Long: `Record an explicit, operator-approved Graphify configuration.

Prism never builds an index. Create and verify the index first, then choose
exactly one endpoint kind. The managed dependency is installed only when
--install-managed-environment is selected explicitly:
  local        user-managed local executable
  self-hosted  an operator-managed downstream MCP endpoint
  managed      a named, version-pinned managed downstream MCP endpoint`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !approve {
				return fmt.Errorf("refusing Graphify setup without --approve; Prism never installs dependencies or builds indexes automatically")
			}
			unlock, err := acquireRuntimeConfigLock(cmd.Context())
			if err != nil {
				return err
			}
			defer unlock()
			workspace, err := filepath.Abs(workspace)
			if err != nil {
				return err
			}
			indexPath, err = filepath.Abs(indexPath)
			if err != nil {
				return err
			}
			if installManaged && graphify.EndpointKind(kind) != graphify.EndpointManaged {
				return fmt.Errorf("--install-managed-environment requires --endpoint-kind managed")
			}
			cfg, err := graphify.Load(filepath.Join(gf.stateDir, "graphify.yaml"))
			if err != nil {
				return err
			}
			registeredServer := false
			managedEnvironmentExisted := false
			if installManaged {
				if environment == "" {
					environment = filepath.Join(gf.stateDir, "graphify", "environments", "v0.9.61")
				}
				environment, err = filepath.Abs(environment)
				if err != nil {
					return err
				}
				if environmentVersion == "" {
					environmentVersion = graphify.PinnedUpstreamVersion
				}
				if environmentVersion != graphify.PinnedUpstreamVersion {
					return fmt.Errorf("managed environment version must be %q", graphify.PinnedUpstreamVersion)
				}
				executable = managedGraphifyExecutable(environment)
				if info, statErr := os.Stat(environment); statErr == nil {
					if !info.IsDir() {
						return fmt.Errorf("managed environment path %s is not a directory", environment)
					}
					managedEnvironmentExisted = true
				} else if !os.IsNotExist(statErr) {
					return statErr
				}
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
				if installManaged {
					fmt.Fprintf(cmd.OutOrStdout(), "Would run: %s sync --frozen --no-dev --no-managed-python --project %s\n", uvExecutable, environment)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Would record operator-approved Graphify %s endpoint %q and bind %s to %s. No executable, endpoint, or index will be started or changed.\n", endpoint.Kind, endpoint.Server, indexPath, workspace)
				return nil
			}
			if installManaged {
				fmt.Fprintf(cmd.OutOrStdout(), "Installing the explicitly selected pinned Graphify environment with: %s sync --frozen --no-dev --no-managed-python --project %s\n", uvExecutable, environment)
				if err := installManagedGraphifyEnvironment(cmd.Context(), environment, uvExecutable); err != nil {
					if !managedEnvironmentExisted {
						if cleanupErr := os.RemoveAll(environment); cleanupErr != nil {
							return fmt.Errorf("managed Graphify dependency installation failed: %w (cleanup failed: %v)", err, cleanupErr)
						}
						return fmt.Errorf("managed Graphify dependency installation failed; the newly created environment was removed and host/runtime configuration was not changed: %w", err)
					}
					return fmt.Errorf("managed Graphify dependency installation failed; the pre-existing environment was preserved and host/runtime configuration was not changed: %w", err)
				}
				server := downstreammcp.Server{Name: server, Transport: downstreammcp.TransportCommand, Command: executable, Args: []string{"--graph", indexPath}, Description: "Prism-managed pinned Graphify MCP endpoint"}
				mutation, err := downstreammcp.NewService(mcpServersPath()).AddOrUpdate(cmd.Context(), server, false)
				if err != nil {
					return fmt.Errorf("register managed Graphify endpoint: %w; managed dependency remains installed", err)
				}
				if mutation.Outcome == downstreammcp.OutcomeConflict {
					return fmt.Errorf("downstream MCP server %q already exists with different settings", server.Name)
				}
				registeredServer = mutation.Outcome == downstreammcp.OutcomeCreated
			}
			cfg.Binding = &graphify.Binding{
				Workspace: workspace, IndexPath: indexPath, UpstreamVersion: upstreamVersion,
				SchemaVersion: schemaVersion, GenerationFingerprint: fingerprint,
			}
			cfg.Endpoint = endpoint
			cfg.OperatorApproved = true
			if err := graphify.Save(filepath.Join(gf.stateDir, "graphify.yaml"), cfg); err != nil {
				if registeredServer {
					_, rollbackErr := downstreammcp.NewService(mcpServersPath()).Remove(cmd.Context(), endpoint.Server)
					if rollbackErr != nil {
						return fmt.Errorf("save Graphify configuration: %w (downstream registration rollback failed: %v; managed dependency remains installed)", err, rollbackErr)
					}
				}
				return fmt.Errorf("save Graphify configuration: %w; managed dependency remains installed and may be retried or removed explicitly", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Recorded operator-approved Graphify %s endpoint %q and bound index %s to workspace %s. Prism did not build or refresh the index.\n", endpoint.Kind, endpoint.Server, indexPath, workspace)
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
	cmd.Flags().BoolVar(&installManaged, "install-managed-environment", false, "Explicitly create and sync Prism's pinned Graphify environment")
	cmd.Flags().StringVar(&uvExecutable, "uv", "uv", "uv executable used for the explicitly selected managed install")
	_ = cmd.MarkFlagRequired("workspace")
	_ = cmd.MarkFlagRequired("index")
	_ = cmd.MarkFlagRequired("fingerprint")
	_ = cmd.MarkFlagRequired("server")
	_ = cmd.MarkFlagRequired("endpoint-kind")
	return cmd
}

func managedGraphifyExecutable(environment string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(environment, ".venv", "Scripts", "graphify-mcp.exe")
	}
	return filepath.Join(environment, ".venv", "bin", "graphify-mcp")
}

func installManagedGraphifyEnvironment(ctx context.Context, environment, uvExecutable string) error {
	uvPath, err := exec.LookPath(uvExecutable)
	if err != nil {
		return fmt.Errorf("find uv: %w", err)
	}
	managedRoot, err := filepath.Abs(filepath.Join(gf.stateDir, "graphify", "environments"))
	if err != nil {
		return err
	}
	environment, err = filepath.Abs(environment)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(managedRoot, environment)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("managed environment must be below %s", managedRoot)
	}
	if err := os.MkdirAll(environment, 0o700); err != nil {
		return err
	}
	const sourceRoot = "skills/graphify-query/references/managed-environment"
	for _, name := range []string{"pyproject.toml", "uv.lock"} {
		data, readErr := fs.ReadFile(prismbundle.BundleFS(), sourceRoot+"/"+name)
		if readErr != nil {
			return readErr
		}
		if writeErr := os.WriteFile(filepath.Join(environment, name), data, 0o600); writeErr != nil {
			return writeErr
		}
	}
	command := exec.CommandContext(ctx, uvPath, "sync", "--frozen", "--no-dev", "--no-managed-python", "--project", environment)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return err
	}
	executable := managedGraphifyExecutable(environment)
	if info, err := os.Stat(executable); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("pinned environment did not produce %s", executable)
	}
	return nil
}

func newGraphifyDoctorCmd() *cobra.Command {
	var workspace, fingerprint string
	var probe bool
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
					if probe {
						listed, probeErr := downstreammcp.New(state).ListTools(cmd.Context(), server.Name, downstreammcp.ListToolsOptions{IncludeSchema: true})
						if probeErr != nil {
							ready.Ready = false
							ready.Message = fmt.Sprintf("Graphify endpoint probe failed: %v", probeErr)
							ready.Checks = append(ready.Checks, graphify.Check{Name: "endpoint_probe", Message: ready.Message})
						} else {
							contracts := make([]graphify.ToolContract, 0, len(listed.Tools))
							for _, tool := range listed.Tools {
								schema, ok := tool.InputSchema.(map[string]any)
								if !ok {
									probeErr = fmt.Errorf("tool %q has a non-object schema", tool.Name)
									break
								}
								contracts = append(contracts, graphify.ToolContract{Name: tool.Name, InputSchema: schema})
							}
							if probeErr == nil {
								probeErr = graphify.ValidatePinnedToolInventory(contracts)
							}
							if probeErr != nil {
								ready.Ready = false
								ready.Message = probeErr.Error()
								ready.Checks = append(ready.Checks, graphify.Check{Name: "endpoint_probe", Message: probeErr.Error()})
							} else {
								ready.Checks = append(ready.Checks, graphify.Check{Name: "endpoint_probe", Ready: true, Message: "live endpoint matches the pinned Graphify MCP contract"})
							}
						}
					}
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
	cmd.Flags().BoolVar(&probe, "probe", false, "Explicitly connect to the configured endpoint and verify its live tool contract")
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
			unlock, err := acquireRuntimeConfigLock(cmd.Context())
			if err != nil {
				return err
			}
			defer unlock()
			path := filepath.Join(gf.stateDir, "graphify.yaml")
			cfg, err := graphify.Load(path)
			if err != nil {
				return err
			}
			managedEnvironment := ""
			if cfg.Endpoint != nil && cfg.Endpoint.Kind == graphify.EndpointManaged && cfg.Endpoint.Executable != "" {
				managedEnvironment = cfg.Endpoint.Environment
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Would remove Prism Graphify configuration %s", path)
				if managedEnvironment != "" {
					fmt.Fprintf(cmd.OutOrStdout(), ", Prism-owned managed environment %s, and its exact downstream MCP registration", managedEnvironment)
				}
				fmt.Fprintln(cmd.OutOrStdout(), ". User-managed executables, indexes, endpoints, and MCP servers are preserved.")
				return nil
			}
			if managedEnvironment != "" {
				if err := removeManagedGraphifyEnvironment(cmd.Context(), cfg); err != nil {
					return err
				}
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove Graphify configuration: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Removed Prism Graphify configuration and any exactly matching Prism-owned managed environment. User-managed executables, indexes, endpoints, and MCP servers were preserved.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&approve, "approve", false, "Confirm removal of Prism's Graphify configuration")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview removal without writing")
	return cmd
}

func removeManagedGraphifyEnvironment(ctx context.Context, cfg graphify.Config) error {
	endpoint := cfg.Endpoint
	if endpoint == nil || endpoint.Kind != graphify.EndpointManaged || endpoint.Executable == "" {
		return nil
	}
	managedRoot, err := filepath.Abs(filepath.Join(gf.stateDir, "graphify", "environments"))
	if err != nil {
		return err
	}
	environment, err := filepath.Abs(endpoint.Environment)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(managedRoot, environment)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("refusing to remove managed environment outside %s", managedRoot)
	}
	state, err := downstreammcp.Load(mcpServersPath())
	if err != nil {
		return err
	}
	if server, ok := state.Get(endpoint.Server); ok {
		if server.Transport != downstreammcp.TransportCommand || filepath.Clean(server.Command) != filepath.Clean(endpoint.Executable) || cfg.Binding == nil || len(server.Args) != 2 || server.Args[0] != "--graph" || filepath.Clean(server.Args[1]) != filepath.Clean(cfg.Binding.IndexPath) {
			return fmt.Errorf("refusing to remove downstream MCP server %q because it no longer matches the Prism-owned Graphify registration", endpoint.Server)
		}
		if _, err := downstreammcp.NewService(mcpServersPath()).Remove(ctx, endpoint.Server); err != nil {
			return err
		}
	}
	if err := os.RemoveAll(environment); err != nil {
		return fmt.Errorf("remove Prism-owned managed Graphify environment: %w", err)
	}
	return nil
}
