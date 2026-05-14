// Package cmd contains all Cobra commands for the paladin CLI.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mattn/go-isatty"
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
  PALADIN_API_URL   Base URL of paladin-edge (default: http://localhost:9002)
  PALADIN_AUTH_URL  Base URL of paladin-auth (default: http://localhost:9003)
  PALADIN_TENANT    Tenant ID for all requests
  PALADIN_TOKEN     API authentication token`,
	PersistentPreRun: func(cmd *cobra.Command, _ []string) {
		maybeStartBackgroundUpdateCheck(cmd)
	},
	RunE: runRoot,
}

var rootIsTerminal = isatty.IsTerminal

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
	rootCmd.PersistentFlags().Bool("json-stream", false, "Stream newline-delimited JSON events")
	rootCmd.PersistentFlags().Bool("simple", false, "Use text-only mode instead of launching the TUI")
	rootCmd.PersistentFlags().Bool("tour", false, "Show the PaladinAI quick tour")

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

func outputFormat(cmd *cobra.Command) string {
	if isCIMode(cmd) {
		return "json"
	}
	f := cmd.Flag("output")
	if f == nil || f.Value.String() == "" {
		return "table"
	}
	return f.Value.String()
}

// isCIMode reports whether --ci was set or PALADIN_CI env var is non-empty.
func isCIMode(cmd *cobra.Command) bool {
	if f := cmd.Flag("ci"); f != nil && f.Value.String() == "true" {
		return true
	}
	if f := cmd.InheritedFlags().Lookup("ci"); f != nil && f.Value.String() == "true" {
		return true
	}
	if root := cmd.Root(); root != nil {
		if f := root.PersistentFlags().Lookup("ci"); f != nil && f.Value.String() == "true" {
			return true
		}
	}
	if f := cmd.Flags().Lookup("ci"); f != nil && f.Value.String() == "true" {
		return true
	}
	return os.Getenv("PALADIN_CI") != ""
}

// telemetryEnabled reports whether anonymous telemetry should be collected.
// Opt-out via PALADIN_NO_TELEMETRY=1 or config telemetry=false.
func telemetryEnabled() bool {
	return os.Getenv("PALADIN_NO_TELEMETRY") == ""
}

func runRoot(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return cmd.Help()
	}
	if tour, _ := cmd.Flags().GetBool("tour"); tour {
		return writeTour(cmd)
	}
	if simple, _ := cmd.Flags().GetBool("simple"); simple {
		return alertListCmd.RunE(cmd, nil)
	}
	if jsonStream, _ := cmd.Flags().GetBool("json-stream"); jsonStream {
		return runTail(cmd, nil)
	}
	if isCIMode(cmd) || !rootIsTerminal(os.Stdout.Fd()) {
		return cmd.Help()
	}
	if err := maybeShowFirstRunTour(cmd); err != nil {
		return err
	}
	return dashboardCmd.RunE(cmd, nil)
}

type tourResult struct {
	Steps []string `json:"steps"`
}

func writeTour(cmd *cobra.Command) error {
	result := tourResult{Steps: []string{
		"This is your live incident feed.",
		"Press Enter on any incident to see agent reasoning and pending actions.",
		"Type natural language questions or use slash commands.",
		"Press ? anytime to see keyboard shortcuts.",
	}}
	if outputFormat(cmd) == "json" {
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			return fmt.Errorf("write tour: %w", err)
		}
		return nil
	}
	fmt.Fprintln(os.Stdout, "Welcome to PaladinAI. Quick tour:")
	for i, step := range result.Steps {
		fmt.Fprintf(os.Stdout, "  [%d/%d] %s\n", i+1, len(result.Steps), step)
	}
	return nil
}

func firstRunPath() string {
	return filepath.Join(configDir(), "first_run")
}

func maybeShowFirstRunTour(cmd *cobra.Command) error {
	path := firstRunPath()
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check first-run marker: %w", err)
	}
	if err := writeTour(cmd); err != nil {
		return err
	}
	if err := os.MkdirAll(configDir(), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(path, []byte("shown\n"), 0o600); err != nil {
		return fmt.Errorf("write first-run marker: %w", err)
	}
	return nil
}
