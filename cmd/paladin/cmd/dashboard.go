package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/paladinai/paladinai/cmd/paladin/tui"
	"github.com/spf13/cobra"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Launch the interactive TUI dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant, err := requireTenant(cmd)
		if err != nil {
			return err
		}

		apiURL, _ := cmd.Flags().GetString("api-url")
		m := tui.New(tenant)

		// Pre-load alerts before launching TUI
		alerts, fetchErr := fetchAlerts(cmd, apiURL, tenant)
		if fetchErr != nil {
			m = m.SetAlerts(nil)
		} else {
			m = m.SetAlerts(alerts)
		}

		p := tea.NewProgram(m, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

// fetchAlerts queries the API and converts the response to []tui.Alert.
func fetchAlerts(cmd *cobra.Command, apiURL, tenant string) ([]tui.Alert, error) {
	url := fmt.Sprintf("%s/api/v1/alerts?status=firing", apiURL)
	req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Tenant-ID", tenant)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API %d", resp.StatusCode)
	}

	var response struct {
		Data []struct {
			Fingerprint   string `json:"fingerprint"`
			Severity      string `json:"severity"`
			Status        string `json:"status"`
			Title         string `json:"title"`
			CorrelationID string `json:"correlation_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	alerts := make([]tui.Alert, len(response.Data))
	for i, a := range response.Data {
		alerts[i] = tui.Alert{
			Fingerprint:   a.Fingerprint,
			Severity:      a.Severity,
			Status:        a.Status,
			Title:         a.Title,
			CorrelationID: a.CorrelationID,
			Tenant:        tenant,
		}
	}
	return alerts, nil
}
