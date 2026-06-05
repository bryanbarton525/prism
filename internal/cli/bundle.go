package cli

import (
	"encoding/json"
	"fmt"
	"os"

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
	var sourceRoot, publicKey, prismVersion string
	cmd := &cobra.Command{
		Use:   "verify <manifest>",
		Short: "Verify bundle metadata or a signed registry manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if publicKey != "" {
				manifest, err := bundles.VerifyRegistryManifest(bundles.InstallOptions{
					ManifestPath: args[0],
					SourceRoot:   sourceRoot,
					PublicKey:    publicKey,
					PrismVersion: prismVersion,
				})
				if err != nil {
					return err
				}
				fmt.Printf("registry: %s@%s ok (%d bundle(s))\n", manifest.RegistryID, manifest.Version, len(manifest.Bundles))
				return nil
			}
			manifest, err := bundles.LoadManifest(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("bundle: %s@%s ok\n", manifest.ID, manifest.Version)
			return nil
		},
	}
	cmd.Flags().StringVar(&sourceRoot, "source-root", "", "Root containing files referenced by signed registry manifest (default: manifest directory)")
	cmd.Flags().StringVar(&publicKey, "public-key", "", "Ed25519 public key as base64, hex, or file path; enables signed registry verification")
	cmd.Flags().StringVar(&prismVersion, "prism-version", "0.1.0", "Prism version for compatibility checks")
	return cmd
}

func newBundleInstallCmd() *cobra.Command {
	var sourceRoot, publicKey, destRoot, prismVersion string
	cmd := &cobra.Command{
		Use:   "install <registry-manifest.json>",
		Short: "Verify and install a signed registry bundle manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if destRoot == "" {
				destRoot = gf.rootDir
			}
			manifest, err := bundles.InstallVerified(bundles.InstallOptions{
				ManifestPath: args[0],
				SourceRoot:   sourceRoot,
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
