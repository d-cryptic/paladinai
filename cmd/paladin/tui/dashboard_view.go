package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	header := titleStyle.Width(contentWidth(m.width)).Render(
		fmt.Sprintf(" PaladinAI Dashboard — tenant: %s • mode: %s ", m.tenant, m.mode),
	)
	help := helpStyle.Render("↑/↓ navigate  •  tab surface  •  enter inspect  •  / commands  •  ? help  •  r refresh  •  q quit")

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		baseStyle.Render(m.table.View()),
		" ",
		baseStyle.Width(detailWidth(m.width)).Render(m.detailPane()),
	)

	parts := []string{header, summaryStrip(m.alerts), metricsStrip(m.metrics), body}
	if m.err != nil {
		parts = append(parts, errorLine(fmt.Sprintf("Error: %v", m.err)))
	}
	if len(m.warnings) > 0 {
		parts = append(parts, warningLine("Warning: "+strings.Join(m.warnings, "; ")))
	}
	if m.helpVisible {
		parts = append(parts, helpOverlay())
	}
	parts = append(parts, commandBar(m), help)
	return strings.Join(parts, "\n")
}

func errorLine(message string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render(message)
}

func warningLine(message string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#D97706")).Render(message)
}

func commandBar(m Model) string {
	input := m.commandInput
	if input == "" {
		input = "_"
	}
	recent := ""
	if len(m.recent) > 0 {
		recent = "  recent: " + strings.Join(m.recent[:minInt(len(m.recent), 3)], "  ")
	}
	return helpStyle.Render("/incidents  /investigate  /runbooks  /integrations  /evals  /agents  /doctor  /help"+recent) + "\n> " + input
}

func helpOverlay() string {
	return baseStyle.Render(strings.Join([]string{
		"Keyboard",
		"  /      command mode",
		"  ?      toggle help",
		"  q      quit",
		"  tab    next surface",
		"  shift+tab previous surface",
		"Commands",
		"  /incidents  monitor mode",
		"  /investigate <id>  incident deep-dive",
		"  /tail       live tail mode",
		"  /runbooks   runbook explorer",
		"  /integrations  integration health",
		"  /evals      eval summary",
		"  /agents     agent queues",
		"  /audit      audit log viewer",
		"  /memory query <text>  memory search",
		"  /pinned     pinned items",
		"  /doctor     health checks",
		"  /config     configure mode",
		"  /quit       exit",
	}, "\n"))
}

func summaryStrip(alerts []Alert) string {
	counts := severityCounts(alerts)
	statuses := statusCounts(alerts)
	parts := []string{
		fmt.Sprintf("active=%d", len(alerts)),
		fmt.Sprintf("P1=%d", counts["P1"]),
		fmt.Sprintf("P2=%d", counts["P2"]),
		fmt.Sprintf("P3=%d", counts["P3"]),
		fmt.Sprintf("P4=%d", counts["P4"]),
	}
	for _, status := range sortedKeys(statuses) {
		parts = append(parts, fmt.Sprintf("%s=%d", status, statuses[status]))
	}
	return helpStyle.Render(strings.Join(parts, "  "))
}

func metricsStrip(metrics Metrics) string {
	health := "n/a"
	if metrics.TotalIntegrations > 0 {
		health = fmt.Sprintf("%d/%d", metrics.HealthyIntegrations, metrics.TotalIntegrations)
	}
	return helpStyle.Render(strings.Join([]string{
		fmt.Sprintf("open=%d", metrics.OpenIncidents),
		fmt.Sprintf("critical=%d", metrics.CriticalIncidents),
		fmt.Sprintf("runbooks=%d", metrics.EmbeddedRunbooks),
		"integrations=" + health,
	}, "  "))
}

func severityCounts(alerts []Alert) map[string]int {
	counts := map[string]int{"P1": 0, "P2": 0, "P3": 0, "P4": 0}
	for _, alert := range alerts {
		severity := strings.ToUpper(alert.Severity)
		if _, ok := counts[severity]; !ok {
			severity = "P4"
		}
		counts[severity]++
	}
	return counts
}

func statusCounts(alerts []Alert) map[string]int {
	counts := make(map[string]int)
	for _, alert := range alerts {
		status := strings.ToLower(strings.TrimSpace(alert.Status))
		if status == "" {
			status = "unknown"
		}
		counts[status]++
	}
	return counts
}

func sortedKeys(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func modeHint(mode string) string {
	switch mode {
	case "tail":
		return "Tail mode: watch alert flow and correlation changes."
	case "investigate":
		return "Investigate mode: inspect the selected incident timeline and actions."
	case "runbooks", "runbook-search":
		return "Runbooks mode: search response procedures before approving actions."
	case "doctor":
		return "Doctor mode: verify API, auth, integration, and stream health."
	case "integrations", "integration-enable":
		return "Integrations mode: review tool health and enable response sources."
	case "evals":
		return "Evals mode: review regression coverage and replay readiness."
	case "agents":
		return "Agents mode: inspect queue pressure and runtime readiness."
	case "audit":
		return "Audit mode: review incident decisions and approval history."
	case "memory", "memory-query":
		return "Memory mode: query prior incidents, runbooks, and learned procedures."
	case "pinned":
		return "Pinned mode: revisit saved incidents, runbooks, and actions."
	case "configure", "config-validate":
		return "Config mode: inspect local settings and validate paladin.yaml."
	default:
		return "Monitor mode: review active alerts and select incidents for detail."
	}
}
