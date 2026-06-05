package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/bundles"
)

func newBundleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bundle",
		Short: "Manage installed Prism bundles",
	}
	cmd.AddCommand(newBundleListCmd())
	cmd.AddCommand(newBundleVerifyCmd())
	cmd.AddCommand(newBundleInstallCmd())
	cmd.AddCommand(newBundleUpdateCmd())
	cmd.AddCommand(newBundleRollbackCmd())
	return cmd
}

func newBundleListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed bundles",
		RunE: func(_ *cobra.Command, _ []string) error {
			state, err := bundles.Load(installedBundlesPath())
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(state.Bundles)
		},
	}
}

func newBundleVerifyCmd() *cobra.Command {
	var sourceRoot, publicKey, prismVersion, sourceName string
	cmd := &cobra.Command{
		Use:   "verify <manifest>",
		Short: "Verify bundle metadata or a signed registry manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			manifestPath, resolvedSourceRoot, err := resolveRegistrySourceArg(sourceName, args[0], sourceRoot)
			if err != nil {
				return err
			}
			if publicKey != "" {
				manifest, err := bundles.VerifyRegistryManifest(bundles.InstallOptions{
					ManifestPath: manifestPath,
					SourceRoot:   resolvedSourceRoot,
					PublicKey:    publicKey,
					PrismVersion: prismVersion,
				})
				if err != nil {
					return err
				}
				fmt.Printf("registry: %s@%s ok (%d bundle(s))\n", manifest.RegistryID, manifest.Version, len(manifest.Bundles))
				return nil
			}
			manifest, err := bundles.LoadManifest(manifestPath)
			if err != nil {
				return err
			}
			fmt.Printf("bundle: %s@%s ok\n", manifest.ID, manifest.Version)
			return nil
		},
	}
	cmd.Flags().StringVar(&sourceRoot, "source-root", "", "Root containing files referenced by signed registry manifest (default: manifest directory)")
	cmd.Flags().StringVar(&sourceName, "source", "", "Configured registry source name for resolving a relative manifest path")
	cmd.Flags().StringVar(&publicKey, "public-key", "", "Ed25519 public key as base64, hex, or file path; enables signed registry verification")
	cmd.Flags().StringVar(&prismVersion, "prism-version", "0.1.0", "Prism version for compatibility checks")
	return cmd
}

func newBundleInstallCmd() *cobra.Command {
	var sourceRoot, publicKey, destRoot, prismVersion, sourceName string
	cmd := &cobra.Command{
		Use:   "install <registry-manifest.json>",
		Short: "Verify and install a signed registry bundle manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if destRoot == "" {
				destRoot = gf.rootDir
			}
			manifestPath, resolvedSourceRoot, err := resolveRegistrySourceArg(sourceName, args[0], sourceRoot)
			if err != nil {
				return err
			}
			manifest, err := bundles.InstallVerified(bundles.InstallOptions{
				ManifestPath: manifestPath,
				SourceRoot:   resolvedSourceRoot,
				DestRoot:     destRoot,
				StatePath:    installedBundlesPath(),
				PublicKey:    publicKey,
				PrismVersion: prismVersion,
			})
			if err != nil {
				return err
			}
			fmt.Printf("installed registry %s@%s (%d bundle(s))\n", manifest.RegistryID, manifest.Version, len(manifest.Bundles))
			return nil
		},
	}
	cmd.Flags().StringVar(&sourceRoot, "source-root", "", "Root containing files referenced by signed registry manifest (default: manifest directory)")
	cmd.Flags().StringVar(&sourceName, "source", "", "Configured registry source name for resolving a relative manifest path")
	cmd.Flags().StringVar(&publicKey, "public-key", "", "Required Ed25519 public key as base64, hex, or file path")
	cmd.Flags().StringVar(&destRoot, "dest-root", "", "Destination root for installed files (default: --root)")
	cmd.Flags().StringVar(&prismVersion, "prism-version", "0.1.0", "Prism version for compatibility checks")
	_ = cmd.MarkFlagRequired("public-key")
	return cmd
}

func newBundleUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <bundle-id>",
		Short: "Update an installed bundle (future local lifecycle command)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("bundle update is not implemented in v1; use bundle verify and bundle install with a signed registry manifest")
		},
	}
}

func resolveRegistrySourceArg(sourceName, manifestArg, sourceRoot string) (string, string, error) {
	if strings.TrimSpace(sourceName) == "" {
		return manifestArg, sourceRoot, nil
	}
	state, err := bundles.LoadSources(registrySourcesPath())
	if err != nil {
		return "", "", err
	}
	for _, source := range state.Sources {
		if source.Name != sourceName {
			continue
		}
		manifestPath, err := joinRegistrySource(source.URL, manifestArg)
		if err != nil {
			return "", "", err
		}
		if strings.TrimSpace(sourceRoot) == "" {
			sourceRoot = source.URL
		}
		return manifestPath, sourceRoot, nil
	}
	return "", "", fmt.Errorf("registry source %q is not configured", sourceName)
}

func joinRegistrySource(sourceURL, rel string) (string, error) {
	if isHTTPRegistrySource(sourceURL) {
		u, err := url.Parse(sourceURL)
		if err != nil {
			return "", err
		}
		u.Path = strings.TrimSuffix(u.Path, "/")
		for _, part := range strings.Split(filepath.ToSlash(filepath.Clean(rel)), "/") {
			if part == "." || part == ".." {
				return "", fmt.Errorf("registry manifest path escapes source")
			}
			u.Path += "/" + url.PathEscape(part)
		}
		return u.String(), nil
	}
	if filepath.IsAbs(rel) {
		return rel, nil
	}
	clean := filepath.Clean(rel)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("registry manifest path escapes source")
	}
	return filepath.Join(sourceURL, clean), nil
}

func isHTTPRegistrySource(value string) bool {
	u, err := url.Parse(strings.TrimSpace(value))
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func newBundleRollbackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rollback <bundle-id>",
		Short: "Rollback an installed bundle (future local lifecycle command)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("bundle rollback is not implemented in v1; use bundle verify and bundle install with a signed registry manifest")
		},
	}
}
