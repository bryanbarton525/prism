package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the Prism version",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println("prism version", version)
		},
	}
}
