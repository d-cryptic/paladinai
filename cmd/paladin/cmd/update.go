package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var (
	updateLookPath = exec.LookPath
	updateRunCmd   = runUpdateCommand
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the paladin CLI",
	Long: `Checks the latest PaladinAI CLI release and prints the upgrade command.

By default this command does not modify the system. Pass --yes to run the
selected upgrade command.`,
	RunE: runUpdate,
}

type updateResult struct {
	Current         string `json:"current"`
	Latest          string `json:"latest,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
	Command         string `json:"command,omitempty"`
	Updated         bool   `json:"updated"`
	Message         string `json:"message"`
}

func runUpdate(cmd *cobra.Command, _ []string) error {
	latestURL, _ := cmd.Flags().GetString("latest-url")
	yes, _ := cmd.Flags().GetBool("yes")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	latest, err := fetchLatestVersion(cmd.Context(), latestURL, Version)
	if err != nil {
		return err
	}

	result := updateResult{
		Current:         Version,
		Latest:          latest.Latest,
		UpdateAvailable: latest.UpdateAvailable,
	}

	if !latest.UpdateAvailable {
		result.Message = "paladin is already up to date"
		return writeUpdateResult(cmd, result)
	}

	name, args, display := updateCommand()
	result.Command = display
	if dryRun || !yes {
		result.Message = "update available; rerun with --yes to install"
		return writeUpdateResult(cmd, result)
	}

	if err := updateRunCmd(cmd.Context(), name, args...); err != nil {
		return fmt.Errorf("run update command %q: %w", display, err)
	}
	result.Updated = true
	result.Message = "paladin update completed"
	return writeUpdateResult(cmd, result)
}

func writeUpdateResult(cmd *cobra.Command, result updateResult) error {
	if outputFormat(cmd) == "json" {
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			return fmt.Errorf("write update result: %w", err)
		}
		return nil
	}
	fmt.Fprintln(os.Stdout, result.Message)
	if result.Latest != "" {
		fmt.Fprintf(os.Stdout, "current: %s\nlatest:  %s\n", result.Current, result.Latest)
	}
	if result.Command != "" {
		fmt.Fprintf(os.Stdout, "command: %s\n", result.Command)
	}
	return nil
}

func updateCommand() (string, []string, string) {
	if _, err := updateLookPath("brew"); err == nil {
		return "brew", []string{"upgrade", "paladinai/tap/paladin"}, "brew upgrade paladinai/tap/paladin"
	}
	script := "curl -fsSL https://get.paladinai.io | sh"
	return "sh", []string{"-c", script}, script
}

func runUpdateCommand(ctx context.Context, name string, args ...string) error {
	c := exec.CommandContext(ctx, name, args...) //nolint:gosec
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}

func init() {
	updateCmd.Flags().Bool("yes", false, "Run the selected update command")
	updateCmd.Flags().Bool("dry-run", false, "Check and print the update command without running it")
	updateCmd.Flags().String("latest-url", defaultLatestVersionURL, "Latest version metadata URL")
	rootCmd.AddCommand(updateCmd)
}
