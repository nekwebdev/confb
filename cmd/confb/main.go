package main

import (
	"fmt"
	"os"

	"github.com/nekwebdev/confb/internal/cli" // ← match your module path!
)

// version gets set at build time by -ldflags in the Makefile.
var version = "dev"

func main() {
	// set up the root CLI command
	root := cli.NewRootCmd(version)

	// execute parses CLI args and runs the right subcommand
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

