package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/bundles"
)

func newRegistryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registry",
		Short: "Manage Prism registry sources",
	}
	source := &cobra.Command{Use: "source", Short: "Manage registry sources"}
	source.AddCommand(newRegistrySourceAddCmd())
	source.AddCommand(newRegistrySourceListCmd())
	cmd.AddCommand(source)
	cmd.AddCommand(newRegistrySyncCmd())
	return cmd
}

func newRegistrySourceAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> <url>",
		Short: "Add a registry source",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			state, err := bundles.LoadSources(registrySourcesPath())
			if err != nil {
				return err
			}
			src := bundles.RegistrySource{Name: args[0], URL: args[1]}
			replaced := false
			for i := range state.Sources {
				if state.Sources[i].Name == src.Name {
					state.Sources[i] = src
					replaced = true
				}
			}
			if !replaced {
				state.Sources = append(state.Sources, src)
			}
			if err := bundles.SaveSources(registrySourcesPath(), state); err != nil {
				return err
			}
			fmt.Printf("registry source %s saved\n", src.Name)
			return nil
		},
	}
}

func newRegistrySourceListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List registry sources",
		RunE: func(_ *cobra.Command, _ []string) error {
			state, err := bundles.LoadSources(registrySourcesPath())
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(state.Sources)
		},
	}
}

func newRegistrySyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Validate configured registry sources are available for bundle commands",
		RunE: func(_ *cobra.Command, _ []string) error {
			state, err := bundles.LoadSources(registrySourcesPath())
			if err != nil {
				return err
			}
			fmt.Printf("registry sources: %d\n", len(state.Sources))
			return nil
		},
	}
}
