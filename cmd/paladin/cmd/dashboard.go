package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/paladinai/paladinai/cmd/paladin/client"
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

		alerts, fetchErr := fetchAlerts(cmd, apiURL, tenant)
		if fetchErr != nil {
			// Surface error in TUI rather than silently showing an empty dashboard.
			updated, _ := m.Update(tui.ErrMsg{Err: fetchErr})
			m = updated.(tui.Model)
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
	u, err := url.Parse(apiURL)
	if err != nil {
		return nil, fmt.Errorf("invalid api-url: %w", err)
	}
	u.Path = "/api/v1/alerts"
	q := u.Query()
	q.Set("status", "firing")
	u.RawQuery = q.Encode()

	body, err := client.Get(cmd.Context(), u.String(), tenant)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("parse response: %w", err)
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
