package main

import (
	"os"

	"github.com/bryanbarton525/prism/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
