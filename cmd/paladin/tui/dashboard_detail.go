package tui

import (
	"fmt"
	"strings"
)

func (m Model) detailPane() string {
	switch m.mode {
	case "runbooks", "runbook-search":
		return m.runbookDetailPane()
	case "integrations", "integration-enable":
		return m.integrationDetailPane()
	case "doctor":
		return m.doctorDetailPane()
	case "evals":
		return m.evalsDetailPane()
	case "agents":
		return m.agentsDetailPane()
	case "audit", "memory", "memory-query", "pinned", "configure", "config-validate", "tail":
		return m.workflowDetailPane()
	}
	if len(m.alerts) == 0 {
		return strings.Join([]string{
			"Incident detail",
			"",
			"No active alerts.",
			"Run `paladin tail` for the live stream or press r to refresh.",
		}, "\n")
	}
	alert := m.selectedAlert()
	return strings.Join([]string{
		"Incident detail",
		"",
		"Title:       " + nonEmpty(alert.Title, "untitled"),
		"Severity:    " + strings.ToUpper(nonEmpty(alert.Severity, "unknown")),
		"Status:      " + nonEmpty(alert.Status, "unknown"),
		"Correlation: " + nonEmpty(alert.CorrelationID, "none"),
		"Fingerprint: " + nonEmpty(alert.Fingerprint, "none"),
		"Tenant:      " + nonEmpty(alert.Tenant, m.tenant),
		"Focused ID:  " + nonEmpty(m.focusedID, "none"),
		"",
		modeHint(m.mode),
	}, "\n")
}

func (m Model) runbookDetailPane() string {
	if len(m.runbooks) == 0 {
		return strings.Join([]string{
			"Runbook explorer",
			"",
			"No imported runbooks.",
			"Use `paladin runbooks import` or `/runbooks search <query>`.",
		}, "\n")
	}
	runbook := m.selectedRunbook()
	status := "pending"
	if runbook.Embedded {
		status = "embedded"
	}
	return strings.Join([]string{
		"Runbook detail",
		"",
		"Title:   " + nonEmpty(runbook.Title, "untitled"),
		"Status:  " + status,
		"Source:  " + nonEmpty(runbook.Source, "unknown"),
		"Updated: " + nonEmpty(runbook.UpdatedAt, "unknown"),
		"ID:      " + nonEmpty(runbook.ID, "none"),
		"",
		modeHint(m.mode),
	}, "\n")
}

func (m Model) integrationDetailPane() string {
	if len(m.integrations) == 0 {
		return strings.Join([]string{
			"Integration health",
			"",
			"No MCP integrations registered.",
			"Use `paladin mcp register` or `paladin integrations enable`.",
		}, "\n")
	}
	integration := m.selectedIntegration()
	health := "healthy"
	if !integration.Healthy {
		health = "degraded"
	}
	return strings.Join([]string{
		"Integration detail",
		"",
		"Name:     " + nonEmpty(integration.Name, integration.ID),
		"Health:   " + health,
		"Endpoint: " + nonEmpty(integration.Endpoint, "unknown"),
		"Tools:    " + strings.Join(integration.Capabilities, ", "),
		"ID:       " + nonEmpty(integration.ID, "none"),
		"",
		modeHint(m.mode),
	}, "\n")
}

func (m Model) doctorDetailPane() string {
	health := "n/a"
	if m.metrics.TotalIntegrations > 0 {
		health = fmt.Sprintf("%d/%d healthy", m.metrics.HealthyIntegrations, m.metrics.TotalIntegrations)
	}
	return strings.Join([]string{
		"Doctor checks",
		"",
		fmt.Sprintf("Open incidents:       %d", m.metrics.OpenIncidents),
		fmt.Sprintf("Critical incidents:   %d", m.metrics.CriticalIncidents),
		fmt.Sprintf("Embedded runbooks:    %d", m.metrics.EmbeddedRunbooks),
		"Integration health:  " + health,
		"",
		"Run `paladin doctor` for live service probes.",
	}, "\n")
}

func (m Model) evalsDetailPane() string {
	return strings.Join([]string{
		"Eval summary",
		"",
		"Golden cases:       1000",
		fmt.Sprintf("Critical signals:   %d", m.metrics.CriticalIncidents),
		fmt.Sprintf("Runbooks embedded:  %d", m.metrics.EmbeddedRunbooks),
		fmt.Sprintf("Tool checks:        %d/%d", m.metrics.HealthyIntegrations, m.metrics.TotalIntegrations),
		"",
		"Run `make eval-golden` or `make eval-live` for full scoring.",
	}, "\n")
}

func (m Model) agentsDetailPane() string {
	return strings.Join([]string{
		"Agent queues",
		"",
		fmt.Sprintf("Triage queue:    %d open", m.metrics.OpenIncidents),
		fmt.Sprintf("Runbook index:   %d embedded", m.metrics.EmbeddedRunbooks),
		fmt.Sprintf("Tool registry:   %d/%d healthy", m.metrics.HealthyIntegrations, m.metrics.TotalIntegrations),
		"",
		"Use `/investigate <id>` to inspect the active incident path.",
	}, "\n")
}

func (m Model) workflowDetailPane() string {
	return strings.Join([]string{
		"Workflow mode",
		"",
		modeHint(m.mode),
		"",
		fmt.Sprintf("Open incidents:     %d", m.metrics.OpenIncidents),
		fmt.Sprintf("Runbooks embedded:  %d", m.metrics.EmbeddedRunbooks),
		fmt.Sprintf("Healthy tools:      %d", m.metrics.HealthyIntegrations),
	}, "\n")
}

func (m Model) selectedAlert() Alert {
	if len(m.alerts) == 0 {
		return Alert{}
	}
	cursor := m.table.Cursor()
	if cursor < 0 || cursor >= len(m.alerts) {
		return m.alerts[0]
	}
	return m.alerts[cursor]
}

func (m Model) selectedRunbook() Runbook {
	if len(m.runbooks) == 0 {
		return Runbook{}
	}
	cursor := m.table.Cursor()
	if cursor < 0 || cursor >= len(m.runbooks) {
		return m.runbooks[0]
	}
	return m.runbooks[cursor]
}

func (m Model) selectedIntegration() Integration {
	if len(m.integrations) == 0 {
		return Integration{}
	}
	cursor := m.table.Cursor()
	if cursor < 0 || cursor >= len(m.integrations) {
		return m.integrations[0]
	}
	return m.integrations[cursor]
}

func incidentID(alert Alert) string {
	for _, candidate := range []string{alert.CorrelationID, alert.Fingerprint, alert.Title} {
		if strings.TrimSpace(candidate) != "" {
			return candidate
		}
	}
	return "selected"
}
