// Command prism is the CLI entry point for the Prism local specialist agent runner.
package main

import (
	"fmt"
	"os"

	"github.com/bryanbarton525/prism/internal/cli"
)

func main() {
	cfg := &cli.Config{}
	root := cli.NewRootCmd(cfg)
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
