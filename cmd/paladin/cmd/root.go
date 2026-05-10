// Package cmd contains all Cobra commands for the paladin CLI.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// flags shared across commands
	flagAPIURL  string
	flagTenant  string
	flagOutput  string // json | table
)

var rootCmd = &cobra.Command{
	Use:   "paladin",
	Short: "PaladinAI CLI — manage alerts, incidents, and MCP servers",
	Long: `paladin is the command-line interface for PaladinAI.

Use it to query active alerts, manage incidents, register MCP servers,
and launch the interactive TUI dashboard.

Environment variables:
  PALADIN_API_URL   Base URL of paladin-hub/edge (default: http://localhost:8080)
  PALADIN_TENANT    Tenant ID for all requests
  PALADIN_TOKEN     API authentication token`,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagAPIURL, "api-url", envStr("PALADIN_API_URL", "http://localhost:8080"), "PaladinAI API base URL")
	rootCmd.PersistentFlags().StringVar(&flagTenant, "tenant", os.Getenv("PALADIN_TENANT"), "Tenant ID")
	rootCmd.PersistentFlags().StringVarP(&flagOutput, "output", "o", "table", "Output format: table or json")

	rootCmd.AddCommand(alertCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(dashboardCmd)
	rootCmd.AddCommand(versionCmd)
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// requireTenant checks that a tenant ID is set, printing a friendly error if not.
func requireTenant(cmd *cobra.Command) (string, error) {
	t, _ := cmd.Flags().GetString("tenant")
	if t == "" {
		if f := cmd.InheritedFlags().Lookup("tenant"); f != nil {
			t = f.Value.String()
		}
	}
	if t == "" {
		return "", fmt.Errorf("tenant ID is required — set --tenant or PALADIN_TENANT env var")
	}
	return t, nil
}
