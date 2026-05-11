package nats_test

import (
	"strings"
	"testing"

	inats "github.com/paladinai/paladinai/internal/nats"
	"github.com/stretchr/testify/assert"
)

// TestStreamNames pins the NATS stream names that external services depend on.
// Changing these values is a breaking change for all JetStream consumers.
func TestStreamNames(t *testing.T) {
	assert.Equal(t, "PALADIN_ALERTS", inats.StreamAlerts)
	assert.Equal(t, "PALADIN_AGENT_WORK", inats.StreamAgentWork)
	assert.Equal(t, "PALADIN_RUNBOOK_STEPS", inats.StreamRunbookSteps)
	assert.Equal(t, "PALADIN_INCIDENTS", inats.StreamIncidents)
	assert.Equal(t, "PALADIN_AUDIT", inats.StreamAudit)
}

// TestStreamNames_Distinct ensures no two streams share the same name.
func TestStreamNames_Distinct(t *testing.T) {
	names := []string{
		inats.StreamAlerts,
		inats.StreamAgentWork,
		inats.StreamRunbookSteps,
		inats.StreamIncidents,
		inats.StreamAudit,
	}
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		assert.False(t, seen[n], "duplicate stream name: %q", n)
		seen[n] = true
	}
}

// TestSubjectPatterns pins the NATS subject wildcards used for stream routing.
func TestSubjectPatterns(t *testing.T) {
	assert.Equal(t, "paladin.alerts.raw.>", inats.SubjectAlertsRaw)
	assert.Equal(t, "paladin.alerts.deduped.>", inats.SubjectAlertsDeduped)
	assert.Equal(t, "paladin.alerts.correlated.>", inats.SubjectAlertsCorrelated)
	assert.Equal(t, "paladin.agent.work.>", inats.SubjectAgentWork)
	assert.Equal(t, "paladin.runbook.steps.>", inats.SubjectRunbookSteps)
	assert.Equal(t, "paladin.incidents.>", inats.SubjectIncidents)
	assert.Equal(t, "paladin.audit.>", inats.SubjectAudit)
}

// TestSubjectPatterns_UseWildcard verifies that every subject pattern ends with
// the NATS full-wildcard ">" so that hierarchical routing is possible.
func TestSubjectPatterns_UseWildcard(t *testing.T) {
	patterns := []string{
		inats.SubjectAlertsRaw,
		inats.SubjectAlertsDeduped,
		inats.SubjectAlertsCorrelated,
		inats.SubjectAgentWork,
		inats.SubjectRunbookSteps,
		inats.SubjectIncidents,
		inats.SubjectAudit,
	}
	for _, p := range patterns {
		assert.True(t, strings.HasSuffix(p, ">"),
			"subject %q must end with NATS full-wildcard '>'", p)
	}
}

// TestSubjectPatterns_PaladinPrefix ensures all subjects share the "paladin." namespace.
func TestSubjectPatterns_PaladinPrefix(t *testing.T) {
	patterns := []string{
		inats.SubjectAlertsRaw,
		inats.SubjectAlertsDeduped,
		inats.SubjectAlertsCorrelated,
		inats.SubjectAgentWork,
		inats.SubjectRunbookSteps,
		inats.SubjectIncidents,
		inats.SubjectAudit,
	}
	for _, p := range patterns {
		assert.True(t, strings.HasPrefix(p, "paladin."),
			"subject %q must be in the 'paladin.' namespace", p)
	}
}
