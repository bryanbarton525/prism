package main

import (
	"fmt"
	"os"

	"github.com/bryanbarton525/prism/internal/cli"
)

func main() {
	root := cli.NewRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
