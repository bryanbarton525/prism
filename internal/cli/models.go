package cli

import (
	"fmt"
	"os"

	"github.com/bryanbarton525/prism/internal/toolmodel"
	"github.com/spf13/cobra"
)

func newModelsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "models", Short: "Manage local tool recommendation models"}
	cmd.AddCommand(&cobra.Command{
		Use: "setup <potion|onnx>", Short: "Install a pinned tool embedding model", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			switch args[0] {
			case "potion":
				if err := toolmodel.SetupPotion(command.Context(), gf.stateDir); err != nil {
					return err
				}
				model, _ := toolmodel.PotionPaths(gf.stateDir)
				fmt.Printf("Potion ready: %s\n", model)
			case "onnx":
				if err := toolmodel.SetupMiniLM(command.Context(), gf.stateDir); err != nil {
					return err
				}
				fmt.Printf("MiniLM ready: %s\n", toolmodel.MiniLMPath(gf.stateDir))
			default:
				return fmt.Errorf("supported models: potion, onnx")
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use: "status", Short: "Show local tool model availability", Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			model, vocab := toolmodel.PotionPaths(gf.stateDir)
			if _, err := os.Stat(model); err != nil {
				fmt.Println("potion: not installed")
			} else if _, err := toolmodel.LoadPotion(model, vocab); err != nil {
				fmt.Printf("potion: invalid (%v)\n", err)
			} else {
				fmt.Printf("potion: ready (%s)\n", toolmodel.PotionRevision)
			}
			if _, err := os.Stat(toolmodel.MiniLMPath(gf.stateDir) + "/model.onnx"); err == nil {
				fmt.Printf("onnx: installed (%s)\n", toolmodel.MiniLMRevision)
			} else {
				fmt.Println("onnx: not installed")
			}
			return nil
		},
	})
	return cmd
}
