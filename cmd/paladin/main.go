// Command paladin is the PaladinAI command-line interface.
// It provides commands for managing alerts, incidents, MCP servers, and
// the interactive TUI dashboard.
package main

import (
	"os"

	"github.com/paladinai/paladinai/cmd/paladin/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
