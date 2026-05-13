package cmd

import (
	"encoding/json"
	"fmt"
	"os"

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
	RunE: func(cmd *cobra.Command, args []string) error {
		return writeVersionResult(cmd, versionResult{Version: Version, Commit: Commit})
	},
}

type versionResult struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

func writeVersionResult(cmd *cobra.Command, result versionResult) error {
	if outputFormat(cmd) == "json" {
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			return fmt.Errorf("write version: %w", err)
		}
		return nil
	}
	fmt.Printf("paladin %s (%s)\n", result.Version, result.Commit)
	return nil
}
