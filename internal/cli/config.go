package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/app"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration and environment management",
	}
	cmd.AddCommand(newDoctorCmd())
	return cmd
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check Ollama connectivity, model availability, and configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := buildConfig()
			runner, err := app.New(cfg)
			if err != nil {
				return err
			}
			dr, err := runner.Doctor(context.Background())
			if err != nil {
				return err
			}

			if global.jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(dr)
			}

			fmt.Printf("Ollama host:  %s\n", dr.OllamaHost)
			fmt.Printf("Agent dir:    %s\n", dr.AgentDir)
			fmt.Printf("Agents:       %d\n", dr.AgentCount)
			fmt.Printf("Skills:       %d\n", dr.SkillCount)
			fmt.Println()

			allOK := true
			for _, check := range dr.Checks {
				icon := "✓"
				if check.Status == "fail" {
					icon = "✗"
					allOK = false
				} else if check.Status == "warn" {
					icon = "!"
				}
				fmt.Printf("[%s] %s: %s\n", icon, check.Name, check.Message)
			}

			fmt.Println()
			if allOK {
				fmt.Println("Status: ok")
			} else {
				fmt.Println("Status: degraded")
				return fmt.Errorf("one or more checks failed")
			}
			return nil
		},
	}
}
