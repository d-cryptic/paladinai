// Package cmd contains all Cobra commands for the paladin CLI.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const defaultAPIURL = "http://localhost:9002"

var rootCmd = &cobra.Command{
	Use:   "paladin",
	Short: "PaladinAI CLI — manage alerts, incidents, and MCP servers",
	Long: `paladin is the command-line interface for PaladinAI.

Use it to query active alerts, manage incidents, register MCP servers,
and launch the interactive TUI dashboard.

Environment variables:
  PALADIN_API_URL   Base URL of paladin-hub/edge (default: http://localhost:9002)
  PALADIN_TENANT    Tenant ID for all requests
  PALADIN_TOKEN     API authentication token`,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().String("api-url", envStr("PALADIN_API_URL", defaultAPIURL), "PaladinAI API base URL")
	rootCmd.PersistentFlags().String("tenant", os.Getenv("PALADIN_TENANT"), "Tenant ID")
	rootCmd.PersistentFlags().String("token", os.Getenv("PALADIN_TOKEN"), "Bearer token for authentication")
	rootCmd.PersistentFlags().StringP("output", "o", "table", "Output format: table or json")
	// --ci disables TUI and forces JSON output; useful in GitHub Actions / ArgoCD.
	rootCmd.PersistentFlags().Bool("ci", envStr("PALADIN_CI", "") != "", "CI mode: disable TUI, emit JSON, exit-code-only")

	rootCmd.AddCommand(alertCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(dashboardCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(incidentCmd)
	rootCmd.AddCommand(tenantCmd)

	// Cobra registers the completion command automatically when CompletionOptions
	// are not explicitly hidden. Call InitDefaultCompletionCmd to ensure it is
	// always available regardless of execution environment.
	rootCmd.InitDefaultCompletionCmd()
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// requireTenant reads the tenant from the persistent --tenant flag.
func requireTenant(cmd *cobra.Command) (string, error) {
	f := cmd.Flag("tenant")
	if f == nil || f.Value.String() == "" {
		return "", fmt.Errorf("tenant ID is required — set --tenant or PALADIN_TENANT env var")
	}
	return f.Value.String(), nil
}

// optToken returns the --token flag value (may be empty for unauthenticated requests).
func optToken(cmd *cobra.Command) string {
	f := cmd.Flag("token")
	if f == nil {
		return ""
	}
	return f.Value.String()
}

// apiURL returns the --api-url flag value.
func apiURL(cmd *cobra.Command) string {
	f := cmd.Flag("api-url")
	if f == nil {
		return defaultAPIURL
	}
	return f.Value.String()
}

// isCIMode reports whether --ci was set or PALADIN_CI env var is non-empty.
func isCIMode(cmd *cobra.Command) bool {
	if f := cmd.Flag("ci"); f != nil && f.Value.String() == "true" {
		return true
	}
	return os.Getenv("PALADIN_CI") != ""
}

// telemetryEnabled reports whether anonymous telemetry should be collected.
// Opt-out via PALADIN_NO_TELEMETRY=1 or config telemetry=false.
func telemetryEnabled() bool {
	return os.Getenv("PALADIN_NO_TELEMETRY") == ""
}
