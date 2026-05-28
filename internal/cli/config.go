package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	agentpkg "github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/ollama"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration and environment diagnostics",
	}
	cmd.AddCommand(newConfigDoctorCmd())
	return cmd
}

func newConfigDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check Ollama connectivity and agent spec validity",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return configDoctor(cmd.Context())
		},
	}
}

func configDoctor(ctx context.Context) error {
	ok := true

	// 1. Ollama connectivity.
	fmt.Printf("Ollama host:   %s\n", gf.ollamaHost)
	oc := ollama.NewClient(gf.ollamaHost)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := oc.Ping(pingCtx); err != nil {
		fmt.Printf("  [FAIL] %s\n", err)
		ok = false
	} else {
		fmt.Println("  [OK]   reachable")

		// List available models.
		models, err := oc.ListModels(ctx)
		if err != nil {
			fmt.Printf("  [WARN] could not list models: %s\n", err)
		} else if len(models) == 0 {
			fmt.Println("  [WARN] no models found – run: ollama pull llama3.1:8b")
		} else {
			fmt.Printf("  [OK]   %d model(s) available\n", len(models))
			for _, m := range models {
				fmt.Printf("         - %s\n", m)
			}
		}
	}

	// 2. Agent directory.
	fmt.Printf("\nAgent directory: %s\n", gf.agentDir)
	reg := agentpkg.NewRegistry(gf.agentDir)
	if err := reg.Load(); err != nil {
		fmt.Printf("  [FAIL] %s\n", err)
		ok = false
	} else {
		summaries := reg.List()
		if len(summaries) == 0 {
			fmt.Println("  [WARN] no valid agent specs found")
		} else {
			fmt.Printf("  [OK]   %d agent(s) loaded\n", len(summaries))
			for _, s := range summaries {
				fmt.Printf("         - %s (%s)\n", s.ID, s.Model)
			}
		}
	}

	// 3. Skills directory.
	fmt.Printf("\nSkills directory: %s\n", gf.skillsDir)
	entries, err := os.ReadDir(gf.skillsDir)
	if err != nil {
		fmt.Printf("  [FAIL] %s\n", err)
		ok = false
	} else {
		skillCount := 0
		for _, e := range entries {
			if e.IsDir() {
				skillCount++
			}
		}
		if skillCount == 0 {
			fmt.Println("  [WARN] no skill directories found")
		} else {
			fmt.Printf("  [OK]   %d skill director(ies) found\n", skillCount)
		}
	}

	fmt.Println()
	if ok {
		fmt.Println("doctor: all checks passed")
	} else {
		fmt.Println("doctor: one or more checks failed – see above")
	}
	return nil
}
