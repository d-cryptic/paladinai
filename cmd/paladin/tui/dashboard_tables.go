package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
)

func (m *Model) refreshTableRows() {
	switch m.mode {
	case "runbooks", "runbook-search":
		m.table.SetColumns(runbookColumns())
		m.table.SetRows(runbookRows(m.runbooks))
	case "integrations", "integration-enable":
		m.table.SetColumns(integrationColumns())
		m.table.SetRows(integrationRows(m.integrations))
	case "evals":
		m.table.SetColumns(evalColumns())
		m.table.SetRows(evalRows(m.metrics))
	case "agents":
		m.table.SetColumns(agentColumns())
		m.table.SetRows(agentRows(m.metrics))
	default:
		m.table.SetColumns(incidentColumns())
		m.table.SetRows(alertRows(m.alerts))
	}
}

func incidentColumns() []table.Column {
	return []table.Column{
		{Title: "SEV", Width: 4},
		{Title: "STATUS", Width: 10},
		{Title: "TITLE", Width: 34},
		{Title: "CORRELATION", Width: 18},
		{Title: "FINGERPRINT", Width: 16},
	}
}

func runbookColumns() []table.Column {
	return []table.Column{
		{Title: "STATUS", Width: 10},
		{Title: "TITLE", Width: 38},
		{Title: "SOURCE", Width: 12},
		{Title: "UPDATED", Width: 19},
		{Title: "ID", Width: 16},
	}
}

func integrationColumns() []table.Column {
	return []table.Column{
		{Title: "HEALTH", Width: 9},
		{Title: "NAME", Width: 24},
		{Title: "TOOLS", Width: 7},
		{Title: "ENDPOINT", Width: 34},
		{Title: "ID", Width: 16},
	}
}

func evalColumns() []table.Column {
	return []table.Column{
		{Title: "SUITE", Width: 24},
		{Title: "CASES", Width: 8},
		{Title: "SIGNAL", Width: 14},
		{Title: "STATUS", Width: 12},
		{Title: "SOURCE", Width: 20},
	}
}

func agentColumns() []table.Column {
	return []table.Column{
		{Title: "AGENT", Width: 24},
		{Title: "QUEUE", Width: 8},
		{Title: "LOAD", Width: 8},
		{Title: "STATUS", Width: 12},
		{Title: "INPUT", Width: 24},
	}
}

func alertRows(alerts []Alert) []table.Row {
	rows := make([]table.Row, len(alerts))
	for i, alert := range alerts {
		rows[i] = table.Row{
			colourSeverity(alert.Severity),
			alert.Status,
			truncateStr(alert.Title, 34),
			truncateStr(alert.CorrelationID, 18),
			truncateStr(alert.Fingerprint, 16),
		}
	}
	return rows
}

func runbookRows(runbooks []Runbook) []table.Row {
	rows := make([]table.Row, len(runbooks))
	for i, runbook := range runbooks {
		status := "pending"
		if runbook.Embedded {
			status = "embedded"
		}
		rows[i] = table.Row{
			status,
			truncateStr(runbook.Title, 38),
			truncateStr(runbook.Source, 12),
			truncateStr(runbook.UpdatedAt, 19),
			truncateStr(runbook.ID, 16),
		}
	}
	return rows
}

func integrationRows(integrations []Integration) []table.Row {
	rows := make([]table.Row, len(integrations))
	for i, integration := range integrations {
		health := "healthy"
		if !integration.Healthy {
			health = "degraded"
		}
		rows[i] = table.Row{
			health,
			truncateStr(nonEmpty(integration.Name, integration.ID), 24),
			fmt.Sprintf("%d", len(integration.Capabilities)),
			truncateStr(integration.Endpoint, 34),
			truncateStr(integration.ID, 16),
		}
	}
	return rows
}

func evalRows(metrics Metrics) []table.Row {
	status := "ready"
	if metrics.CriticalIncidents > 0 {
		status = "watch"
	}
	return []table.Row{
		{"golden-regression", "1000", fmt.Sprintf("%d P1", metrics.CriticalIncidents), status, "incident replay"},
		{"runbook-grounding", fmt.Sprintf("%d", metrics.EmbeddedRunbooks), "embedded", "ready", "runbook index"},
		{"tool-health", fmt.Sprintf("%d", metrics.TotalIntegrations), fmt.Sprintf("%d healthy", metrics.HealthyIntegrations), "ready", "mcp registry"},
	}
}

func agentRows(metrics Metrics) []table.Row {
	triageStatus := "idle"
	if metrics.OpenIncidents > 0 {
		triageStatus = "running"
	}
	runbookStatus := "ready"
	if metrics.EmbeddedRunbooks == 0 {
		runbookStatus = "waiting"
	}
	return []table.Row{
		{"triage-reactor", fmt.Sprintf("%d", metrics.OpenIncidents), loadLabel(metrics.OpenIncidents), triageStatus, "paladin.alerts"},
		{"runbook-executor", fmt.Sprintf("%d", metrics.EmbeddedRunbooks), loadLabel(metrics.EmbeddedRunbooks), runbookStatus, "paladin.runbook.steps"},
		{"tool-supervisor", fmt.Sprintf("%d", metrics.TotalIntegrations), loadLabel(metrics.TotalIntegrations), "ready", "mcp registry"},
	}
}

func loadLabel(count int) string {
	switch {
	case count >= 10:
		return "high"
	case count > 0:
		return "normal"
	default:
		return "idle"
	}
}
