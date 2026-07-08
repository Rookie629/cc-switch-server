package main

import (
	"fmt"
	"os"

	"github.com/Rookie629/cc-switch-server/internal/cli"
)

// Build-time variables injected via -ldflags.
var (
	Version   = "0.1.0-dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	// Handle --version / -V before delegating to CLI
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-V") {
		fmt.Printf("cc-switch-server %s (commit: %s, built: %s)\n", Version, GitCommit, BuildTime)
		os.Exit(0)
	}
	cli.Execute()
}
