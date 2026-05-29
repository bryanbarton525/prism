package cli

import (
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect and validate Prism configuration",
	}
	cmd.AddCommand(newConfigDoctorCmd())
	return cmd
}

func newConfigDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check Prism configuration and Ollama connectivity",
		Long:  "Prints the resolved configuration and probes the Ollama API for connectivity.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := buildApp(cmd)
			if err != nil {
				return err
			}
			cfg := a.Config

			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "=== Prism configuration ===")
			fmt.Fprintf(out, "  ollama_host   : %s\n", cfg.OllamaHost)
			fmt.Fprintf(out, "  default_model : %s\n", orNone(cfg.DefaultModel))
			fmt.Fprintf(out, "  agent_dir     : %s\n", cfg.AgentDir)
			fmt.Fprintf(out, "  config_file   : %s\n", orNone(cfg.ConfigPath))
			fmt.Fprintln(out)

			fmt.Fprintln(out, "=== Ollama connectivity ===")
			status, latency, probeErr := probeOllama(cfg.OllamaHost)
			if probeErr != nil {
				fmt.Fprintf(out, "  status : UNREACHABLE (%v)\n", probeErr)
			} else {
				fmt.Fprintf(out, "  status  : %s\n", status)
				fmt.Fprintf(out, "  latency : %s\n", latency)
			}

			return nil
		},
	}
}

func probeOllama(host string) (status string, latency time.Duration, err error) {
	client := &http.Client{Timeout: 5 * time.Second}
	start := time.Now()
	resp, err := client.Get(host)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	return resp.Status, time.Since(start), nil
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
