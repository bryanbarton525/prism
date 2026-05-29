// Command prism is the entry point for the Prism CLI.
package main

import (
	"os"

	"github.com/bryanbarton525/prism/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
