package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Version is injected at build time via -ldflags.
	Version = "dev"
	Commit  = "none"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print paladin CLI version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("paladin %s (%s)\n", Version, Commit)
	},
}
