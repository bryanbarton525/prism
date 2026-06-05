package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

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
			for _, source := range state.Sources {
				if err := validateRegistrySource(source.URL); err != nil {
					return fmt.Errorf("registry source %s: %w", source.Name, err)
				}
			}
			fmt.Printf("registry sources: %d ok\n", len(state.Sources))
			return nil
		},
	}
}

func validateRegistrySource(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("source URL is empty")
	}
	u, err := url.Parse(raw)
	if err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
		req, err := http.NewRequest(http.MethodHead, raw, nil)
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				return nil
			}
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return err
	}
	info, err := os.Stat(raw)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("local source is not a directory")
	}
	return nil
}
