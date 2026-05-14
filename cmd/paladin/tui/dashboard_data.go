package tui

import "strings"

// Runbook is the TUI view model for an imported runbook.
type Runbook struct {
	ID        string
	Title     string
	Source    string
	Embedded  bool
	UpdatedAt string
}

// Integration is the TUI view model for an MCP integration/server.
type Integration struct {
	ID           string
	Name         string
	Endpoint     string
	Healthy      bool
	Capabilities []string
}

// Metrics contains derived operational counters rendered in the TUI.
type Metrics struct {
	OpenIncidents       int
	CriticalIncidents   int
	EmbeddedRunbooks    int
	HealthyIntegrations int
	TotalIntegrations   int
}

// Snapshot is the full dashboard payload used by the interactive TUI and CI output.
type Snapshot struct {
	Alerts       []Alert
	Runbooks     []Runbook
	Integrations []Integration
	Metrics      Metrics
	Warnings     []string
}

func NewSnapshot(alerts []Alert, runbooks []Runbook, integrations []Integration) Snapshot {
	return Snapshot{
		Alerts:       append([]Alert(nil), alerts...),
		Runbooks:     append([]Runbook(nil), runbooks...),
		Integrations: append([]Integration(nil), integrations...),
		Metrics:      deriveMetrics(alerts, runbooks, integrations),
	}
}

func deriveMetrics(alerts []Alert, runbooks []Runbook, integrations []Integration) Metrics {
	metrics := Metrics{
		OpenIncidents:     len(alerts),
		TotalIntegrations: len(integrations),
	}
	for _, alert := range alerts {
		if strings.EqualFold(alert.Severity, "P1") || strings.EqualFold(alert.Severity, "critical") {
			metrics.CriticalIncidents++
		}
	}
	for _, runbook := range runbooks {
		if runbook.Embedded {
			metrics.EmbeddedRunbooks++
		}
	}
	for _, integration := range integrations {
		if integration.Healthy {
			metrics.HealthyIntegrations++
		}
	}
	return metrics
}
