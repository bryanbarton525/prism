package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration and diagnostics",
	}
	cmd.AddCommand(newDoctorCmd())
	return cmd
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check Ollama connectivity and Prism configuration",
		Long:  "Reports agent and skill registry state and probes the local Ollama server.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			runner, err := newRunner()
			if err != nil {
				return err
			}
			dr, err := runner.Doctor(cmd.Context())
			if err != nil {
				return err
			}

			if gf.jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(dr)
			}

			fmt.Printf("Ollama host:  %s\n", dr.OllamaHost)
			fmt.Printf("Agent dir:    %s\n", dr.AgentDir)
			fmt.Printf("Skills dir:   %s\n", dr.SkillsDir)
			fmt.Printf("Agents:       %d\n", dr.AgentCount)
			fmt.Printf("Skills:       %d\n", dr.SkillCount)
			fmt.Println()

			allOK := true
			for _, check := range dr.Checks {
				icon := "✓"
				switch check.Status {
				case "fail":
					icon = "✗"
					allOK = false
				case "warn":
					icon = "!"
				}
				fmt.Printf("[%s] %s: %s\n", icon, check.Name, check.Message)
			}
			fmt.Println()
			if dr.Status == "ok" && allOK {
				fmt.Println("Status: ok")
				return nil
			}
			fmt.Println("Status: degraded")
			if !allOK {
				return fmt.Errorf("one or more checks failed")
			}
			return nil
		},
	}
}
