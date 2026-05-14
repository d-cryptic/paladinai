package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

var (
	// Version is injected at build time via -ldflags.
	Version = "dev"
	Commit  = "none"
)

const defaultLatestVersionURL = "https://releases.paladinai.io/latest.json"

var versionHTTPClient = &http.Client{Timeout: 5 * time.Second}

var (
	updateCheckClock = time.Now
	updateCheckOnce  sync.Once
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print paladin CLI version",
	RunE: func(cmd *cobra.Command, args []string) error {
		result := versionResult{Version: Version, Commit: Commit}
		check, _ := cmd.Flags().GetBool("check")
		if check {
			latestURL, _ := cmd.Flags().GetString("latest-url")
			update, err := fetchLatestVersion(cmd.Context(), latestURL, Version)
			if err != nil {
				return err
			}
			result.Latest = update.Latest
			result.UpdateAvailable = update.UpdateAvailable
			result.Upgrade = update.Upgrade
		}
		return writeVersionResult(cmd, result)
	},
}

type versionResult struct {
	Version         string `json:"version"`
	Commit          string `json:"commit"`
	Latest          string `json:"latest,omitempty"`
	UpdateAvailable bool   `json:"update_available,omitempty"`
	Upgrade         string `json:"upgrade,omitempty"`
}

type latestVersionResponse struct {
	Version string `json:"version"`
	Upgrade string `json:"upgrade"`
}

func writeVersionResult(cmd *cobra.Command, result versionResult) error {
	if outputFormat(cmd) == "json" {
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			return fmt.Errorf("write version: %w", err)
		}
		return nil
	}
	fmt.Printf("paladin %s (%s)\n", result.Version, result.Commit)
	if result.Latest != "" {
		fmt.Printf("latest  %s\n", result.Latest)
	}
	if result.UpdateAvailable {
		fmt.Printf("update available: %s -> %s\n", result.Version, result.Latest)
		if result.Upgrade != "" {
			fmt.Printf("upgrade: %s\n", result.Upgrade)
		}
	}
	return nil
}

func fetchLatestVersion(ctx context.Context, latestURL, current string) (versionResult, error) {
	if strings.TrimSpace(latestURL) == "" {
		latestURL = defaultLatestVersionURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestURL, nil)
	if err != nil {
		return versionResult{}, fmt.Errorf("build latest version request: %w", err)
	}
	resp, err := versionHTTPClient.Do(req)
	if err != nil {
		return versionResult{}, fmt.Errorf("fetch latest version: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return versionResult{}, fmt.Errorf("fetch latest version: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return versionResult{}, fmt.Errorf("read latest version response: %w", err)
	}
	var latest latestVersionResponse
	if err := json.Unmarshal(body, &latest); err != nil {
		return versionResult{}, fmt.Errorf("parse latest version response: %w", err)
	}
	latest.Version = strings.TrimSpace(latest.Version)
	if latest.Version == "" {
		return versionResult{}, fmt.Errorf("latest version response missing version")
	}
	if latest.Upgrade == "" {
		latest.Upgrade = "brew upgrade paladinai/tap/paladin or curl -fsSL https://get.paladinai.io | sh"
	}
	return versionResult{
		Latest:          latest.Version,
		UpdateAvailable: isNewerVersion(latest.Version, current),
		Upgrade:         latest.Upgrade,
	}, nil
}

func maybeStartBackgroundUpdateCheck(cmd *cobra.Command) {
	if shouldSkipBackgroundUpdateCheck(cmd) {
		return
	}
	updateCheckOnce.Do(func() {
		go func() {
			_ = runBackgroundUpdateCheck(cmd.Context(), defaultLatestVersionURL, Version, os.Stderr)
		}()
	})
}

func shouldSkipBackgroundUpdateCheck(cmd *cobra.Command) bool {
	if Version == "" || Version == "dev" || Version == "none" {
		return true
	}
	if isCIMode(cmd) {
		return true
	}
	for c := cmd; c != nil; c = c.Parent() {
		switch c.Name() {
		case "version", "update":
			return true
		}
	}
	return false
}

func runBackgroundUpdateCheck(ctx context.Context, latestURL, current string, stderr io.Writer) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load update check config: %w", err)
	}
	now := updateCheckClock().UTC()
	if !cfg.LastUpdateCheck.IsZero() && now.Sub(cfg.LastUpdateCheck) < 24*time.Hour {
		return nil
	}
	cfg.LastUpdateCheck = now
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("save update check timestamp: %w", err)
	}
	result, err := fetchLatestVersion(ctx, latestURL, current)
	if err != nil {
		return fmt.Errorf("check latest version: %w", err)
	}
	if result.UpdateAvailable {
		upgrade := result.Upgrade
		if upgrade == "" {
			upgrade = "paladin update"
		}
		fmt.Fprintf(stderr, "paladin update available: %s -> %s (run: %s)\n", current, result.Latest, upgrade)
	}
	return nil
}

func isNewerVersion(latest, current string) bool {
	latest = strings.TrimPrefix(strings.TrimSpace(latest), "v")
	current = strings.TrimPrefix(strings.TrimSpace(current), "v")
	if current == "" || current == "dev" || current == "none" {
		return false
	}
	latestParts := parseVersionParts(latest)
	currentParts := parseVersionParts(current)
	for i := range latestParts {
		if latestParts[i] > currentParts[i] {
			return true
		}
		if latestParts[i] < currentParts[i] {
			return false
		}
	}
	return false
}

func parseVersionParts(version string) [3]int {
	var parts [3]int
	fields := strings.Split(version, ".")
	for i := 0; i < len(fields) && i < len(parts); i++ {
		field := fields[i]
		if dash := strings.IndexByte(field, '-'); dash >= 0 {
			field = field[:dash]
		}
		for _, r := range field {
			if r < '0' || r > '9' {
				break
			}
			parts[i] = parts[i]*10 + int(r-'0')
		}
	}
	return parts
}

func init() {
	versionCmd.Flags().Bool("check", false, "Check latest available PaladinAI CLI version")
	versionCmd.Flags().String("latest-url", defaultLatestVersionURL, "Latest version metadata URL")
}
