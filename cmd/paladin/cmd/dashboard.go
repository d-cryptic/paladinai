package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"text/tabwriter"

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
		alerts, fetchErr := fetchAlerts(cmd, apiURL, tenant)
		if isCIMode(cmd) || isSimpleMode(cmd) {
			if fetchErr != nil {
				return fetchErr
			}
			return writeDashboardSnapshot(cmd, tenant, alerts)
		}

		m := tui.New(tenant)
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

	opts, err := commandOptions(cmd, tenant)
	if err != nil {
		return nil, err
	}
	body, err := client.Get(cmd.Context(), u.String(), opts)
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

type dashboardSnapshot struct {
	Tenant string           `json:"tenant"`
	Alerts []dashboardAlert `json:"alerts"`
}

type dashboardAlert struct {
	Fingerprint   string `json:"fingerprint"`
	Severity      string `json:"severity"`
	Status        string `json:"status"`
	Title         string `json:"title"`
	CorrelationID string `json:"correlation_id"`
	Tenant        string `json:"tenant"`
}

func writeDashboardSnapshot(cmd *cobra.Command, tenant string, alerts []tui.Alert) error {
	snapshot := dashboardSnapshot{Tenant: tenant, Alerts: make([]dashboardAlert, len(alerts))}
	for i, alert := range alerts {
		snapshot.Alerts[i] = dashboardAlert{
			Fingerprint:   alert.Fingerprint,
			Severity:      alert.Severity,
			Status:        alert.Status,
			Title:         alert.Title,
			CorrelationID: alert.CorrelationID,
			Tenant:        alert.Tenant,
		}
	}
	if outputFormat(cmd) == "json" {
		if err := json.NewEncoder(os.Stdout).Encode(snapshot); err != nil {
			return fmt.Errorf("write dashboard snapshot: %w", err)
		}
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "TENANT\tACTIVE_ALERTS\n%s\t%d\n\n", snapshot.Tenant, len(snapshot.Alerts))
	fmt.Fprintln(w, "SEVERITY\tSTATUS\tTITLE\tCORRELATION\tFINGERPRINT")
	for _, alert := range snapshot.Alerts {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			alert.Severity,
			alert.Status,
			alert.Title,
			alert.CorrelationID,
			alert.Fingerprint,
		)
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("write dashboard table: %w", err)
	}
	return nil
}

func isSimpleMode(cmd *cobra.Command) bool {
	if f := cmd.Flag("simple"); f != nil && f.Value.String() == "true" {
		return true
	}
	if f := cmd.InheritedFlags().Lookup("simple"); f != nil && f.Value.String() == "true" {
		return true
	}
	if root := cmd.Root(); root != nil {
		if f := root.PersistentFlags().Lookup("simple"); f != nil && f.Value.String() == "true" {
			return true
		}
	}
	if f := cmd.Flags().Lookup("simple"); f != nil && f.Value.String() == "true" {
		return true
	}
	return false
}
