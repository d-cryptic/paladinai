package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
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
		snapshot, fetchErr := fetchDashboardData(cmd, apiURL, tenant)
		if isCIMode(cmd) || isSimpleMode(cmd) {
			if fetchErr != nil {
				return fetchErr
			}
			return writeDashboardSnapshot(cmd, tenant, snapshot)
		}

		m := tui.New(tenant)
		m = m.SetSnapshot(snapshot)
		if fetchErr != nil {
			// Surface error in TUI rather than silently showing an empty dashboard.
			updated, _ := m.Update(tui.ErrMsg{Err: fetchErr})
			m = updated.(tui.Model)
		}

		p := tea.NewProgram(m, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

func fetchDashboardData(cmd *cobra.Command, apiURL, tenant string) (tui.Snapshot, error) {
	ctx := cmd.Context()
	opts, err := commandOptions(cmd, tenant)
	if err != nil {
		return tui.Snapshot{}, err
	}

	alerts, err := fetchAlertsWithOptions(ctx, opts, apiURL, tenant)
	if err != nil {
		return tui.Snapshot{}, err
	}

	type result[T any] struct {
		values []T
		err    error
	}
	runbookCh := make(chan result[tui.Runbook], 1)
	integrationCh := make(chan result[tui.Integration], 1)

	go func() {
		values, err := fetchDashboardRunbooksWithOptions(ctx, opts, apiURL)
		runbookCh <- result[tui.Runbook]{values: values, err: err}
	}()
	go func() {
		values, err := fetchDashboardIntegrationsWithOptions(ctx, opts, apiURL)
		integrationCh <- result[tui.Integration]{values: values, err: err}
	}()

	runbookResult := <-runbookCh
	integrationResult := <-integrationCh
	snapshot := tui.NewSnapshot(alerts, runbookResult.values, integrationResult.values)
	if runbookResult.err != nil || integrationResult.err != nil {
		snapshot.Warnings = dashboardFetchWarnings(runbookResult.err, integrationResult.err)
	}
	return snapshot, nil
}

func dashboardFetchWarnings(errs ...error) []string {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			parts = append(parts, err.Error())
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return []string{"partial dashboard fetch: " + strings.Join(parts, "; ")}
}

// fetchAlerts queries the API and converts the response to []tui.Alert.
func fetchAlerts(cmd *cobra.Command, apiURL, tenant string) ([]tui.Alert, error) {
	opts, err := commandOptions(cmd, tenant)
	if err != nil {
		return nil, err
	}
	return fetchAlertsWithOptions(cmd.Context(), opts, apiURL, tenant)
}

func fetchAlertsWithOptions(ctx context.Context, opts client.Options, apiURL, tenant string) ([]tui.Alert, error) {
	data, err := fetchDashboardEnvelope[dashboardAlertWire](ctx, opts, apiURL, "/api/v1/alerts", map[string]string{"status": "firing"})
	if err != nil {
		return nil, err
	}
	alerts := make([]tui.Alert, len(data))
	for i, a := range data {
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

func fetchDashboardRunbooks(cmd *cobra.Command, apiURL, tenant string) ([]tui.Runbook, error) {
	opts, err := commandOptions(cmd, tenant)
	if err != nil {
		return nil, err
	}
	return fetchDashboardRunbooksWithOptions(cmd.Context(), opts, apiURL)
}

func fetchDashboardRunbooksWithOptions(ctx context.Context, opts client.Options, apiURL string) ([]tui.Runbook, error) {
	data, err := fetchDashboardEnvelope[dashboardRunbookWire](ctx, opts, apiURL, "/api/v1/runbooks", map[string]string{"limit": "20"})
	if err != nil {
		return nil, fmt.Errorf("fetch runbooks: %w", err)
	}
	runbooks := make([]tui.Runbook, len(data))
	for i, runbook := range data {
		runbooks[i] = tui.Runbook{
			ID:        runbook.ID,
			Title:     runbook.Title,
			Source:    runbook.Source,
			Embedded:  runbook.Embedded,
			UpdatedAt: runbook.UpdatedAt,
		}
	}
	return runbooks, nil
}

func fetchDashboardIntegrations(cmd *cobra.Command, apiURL, tenant string) ([]tui.Integration, error) {
	opts, err := commandOptions(cmd, tenant)
	if err != nil {
		return nil, err
	}
	return fetchDashboardIntegrationsWithOptions(cmd.Context(), opts, apiURL)
}

func fetchDashboardIntegrationsWithOptions(ctx context.Context, opts client.Options, apiURL string) ([]tui.Integration, error) {
	data, err := fetchDashboardEnvelope[dashboardIntegrationWire](ctx, opts, apiURL, "/api/v1/mcp/servers", nil)
	if err != nil {
		return nil, fmt.Errorf("fetch integrations: %w", err)
	}
	integrations := make([]tui.Integration, len(data))
	for i, integration := range data {
		integrations[i] = tui.Integration{
			ID:           integration.ID,
			Name:         integration.Name,
			Endpoint:     integration.Endpoint,
			Healthy:      integration.Healthy,
			Capabilities: append([]string(nil), integration.Capabilities...),
		}
	}
	return integrations, nil
}

func fetchDashboardEnvelope[T any](ctx context.Context, opts client.Options, apiURL, path string, query map[string]string) ([]T, error) {
	u, err := url.Parse(apiURL)
	if err != nil {
		return nil, fmt.Errorf("invalid api-url: %w", err)
	}
	u.Path = path
	if len(query) > 0 {
		q := u.Query()
		for key, value := range query {
			q.Set(key, value)
		}
		u.RawQuery = q.Encode()
	}

	body, err := client.Get(ctx, u.String(), opts)
	if err != nil {
		return nil, err
	}

	var response struct {
		Data []T `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return response.Data, nil
}

type dashboardAlertWire struct {
	Fingerprint   string `json:"fingerprint"`
	Severity      string `json:"severity"`
	Status        string `json:"status"`
	Title         string `json:"title"`
	CorrelationID string `json:"correlation_id"`
}

type dashboardRunbookWire struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Source    string `json:"source"`
	Embedded  bool   `json:"embedded"`
	UpdatedAt string `json:"updated_at"`
}

type dashboardIntegrationWire struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Endpoint     string   `json:"endpoint"`
	Healthy      bool     `json:"healthy"`
	Capabilities []string `json:"capabilities"`
}

type dashboardSnapshot struct {
	Tenant       string                 `json:"tenant"`
	Alerts       []dashboardAlert       `json:"alerts"`
	Runbooks     []dashboardRunbook     `json:"runbooks"`
	Integrations []dashboardIntegration `json:"integrations"`
	Metrics      tui.Metrics            `json:"metrics"`
	Warnings     []string               `json:"warnings,omitempty"`
}

type dashboardAlert struct {
	Fingerprint   string `json:"fingerprint"`
	Severity      string `json:"severity"`
	Status        string `json:"status"`
	Title         string `json:"title"`
	CorrelationID string `json:"correlation_id"`
	Tenant        string `json:"tenant"`
}

type dashboardRunbook struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Source    string `json:"source"`
	Embedded  bool   `json:"embedded"`
	UpdatedAt string `json:"updated_at"`
}

type dashboardIntegration struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Endpoint     string   `json:"endpoint"`
	Healthy      bool     `json:"healthy"`
	Capabilities []string `json:"capabilities"`
}

func writeDashboardSnapshot(cmd *cobra.Command, tenant string, data tui.Snapshot) error {
	snapshot := dashboardSnapshot{
		Tenant:       tenant,
		Alerts:       make([]dashboardAlert, len(data.Alerts)),
		Runbooks:     make([]dashboardRunbook, len(data.Runbooks)),
		Integrations: make([]dashboardIntegration, len(data.Integrations)),
		Metrics:      data.Metrics,
		Warnings:     append([]string(nil), data.Warnings...),
	}
	for i, alert := range data.Alerts {
		snapshot.Alerts[i] = dashboardAlert{
			Fingerprint:   alert.Fingerprint,
			Severity:      alert.Severity,
			Status:        alert.Status,
			Title:         alert.Title,
			CorrelationID: alert.CorrelationID,
			Tenant:        alert.Tenant,
		}
	}
	for i, runbook := range data.Runbooks {
		snapshot.Runbooks[i] = dashboardRunbook{
			ID:        runbook.ID,
			Title:     runbook.Title,
			Source:    runbook.Source,
			Embedded:  runbook.Embedded,
			UpdatedAt: runbook.UpdatedAt,
		}
	}
	for i, integration := range data.Integrations {
		snapshot.Integrations[i] = dashboardIntegration{
			ID:           integration.ID,
			Name:         integration.Name,
			Endpoint:     integration.Endpoint,
			Healthy:      integration.Healthy,
			Capabilities: append([]string(nil), integration.Capabilities...),
		}
	}
	if outputFormat(cmd) == "json" {
		if err := json.NewEncoder(os.Stdout).Encode(snapshot); err != nil {
			return fmt.Errorf("write dashboard snapshot: %w", err)
		}
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "TENANT\tACTIVE_ALERTS\tRUNBOOKS\tINTEGRATIONS\n%s\t%d\t%d\t%d/%d healthy\n\n",
		snapshot.Tenant,
		len(snapshot.Alerts),
		len(snapshot.Runbooks),
		snapshot.Metrics.HealthyIntegrations,
		snapshot.Metrics.TotalIntegrations,
	)
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
	if len(snapshot.Runbooks) > 0 {
		fmt.Fprintln(w, "\nRUNBOOK\tSOURCE\tEMBEDDED\tUPDATED")
		for _, runbook := range snapshot.Runbooks {
			fmt.Fprintf(w, "%s\t%s\t%t\t%s\n", runbook.Title, runbook.Source, runbook.Embedded, runbook.UpdatedAt)
		}
	}
	if len(snapshot.Integrations) > 0 {
		fmt.Fprintln(w, "\nINTEGRATION\tHEALTHY\tTOOLS\tENDPOINT")
		for _, integration := range snapshot.Integrations {
			fmt.Fprintf(w, "%s\t%t\t%d\t%s\n",
				integration.Name,
				integration.Healthy,
				len(integration.Capabilities),
				integration.Endpoint,
			)
		}
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
