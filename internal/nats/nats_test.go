package nats_test

import (
	"strings"
	"testing"

	inats "github.com/paladinai/paladinai/internal/nats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var allStreamNames = []string{
	inats.StreamAlerts,
	inats.StreamAgentWork,
	inats.StreamRunbookSteps,
	inats.StreamIncidents,
	inats.StreamAudit,
	inats.StreamEvents,
	inats.StreamMemory,
	inats.StreamDLQ,
	inats.StreamEvals,
}

var allSubjectPatterns = []string{
	inats.SubjectAlertsRaw,
	inats.SubjectAlertsDeduped,
	inats.SubjectAlertsCorrelated,
	inats.SubjectAlertsTriaged,
	inats.SubjectAlertsAnalyzed,
	inats.SubjectAgentWork,
	inats.SubjectRunbookSteps,
	inats.SubjectIncidents,
	inats.SubjectAudit,
	inats.SubjectEvents,
	inats.SubjectMemory,
	inats.SubjectDLQ,
	inats.SubjectEvals,
}

// paladinPrefixedSubjects are the subjects that share the internal "paladin."
// namespace. Stage 2 introduces top-level streams (stream., memory., dlq.,
// evals.) that intentionally live outside that namespace per the spec.
var paladinPrefixedSubjects = []string{
	inats.SubjectAlertsRaw,
	inats.SubjectAlertsDeduped,
	inats.SubjectAlertsCorrelated,
	inats.SubjectAlertsTriaged,
	inats.SubjectAlertsAnalyzed,
	inats.SubjectAgentWork,
	inats.SubjectRunbookSteps,
	inats.SubjectIncidents,
	inats.SubjectAudit,
}

// TestStreamNames pins the NATS stream names that external services depend on.
// Changing these values is a breaking change for all JetStream consumers.
func TestStreamNames(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "PALADIN_ALERTS", inats.StreamAlerts)
	assert.Equal(t, "PALADIN_AGENT_WORK", inats.StreamAgentWork)
	assert.Equal(t, "PALADIN_RUNBOOK_STEPS", inats.StreamRunbookSteps)
	assert.Equal(t, "PALADIN_INCIDENTS", inats.StreamIncidents)
	assert.Equal(t, "PALADIN_AUDIT", inats.StreamAudit)
	assert.Equal(t, "PALADIN_EVENTS", inats.StreamEvents)
	assert.Equal(t, "PALADIN_MEMORY", inats.StreamMemory)
	assert.Equal(t, "PALADIN_DLQ", inats.StreamDLQ)
	assert.Equal(t, "PALADIN_EVALS", inats.StreamEvals)
}

// TestStreamNames_Distinct ensures no two streams share the same name.
func TestStreamNames_Distinct(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool, len(allStreamNames))
	for _, n := range allStreamNames {
		require.False(t, seen[n], "duplicate stream name: %q", n)
		seen[n] = true
	}
}

// TestSubjectPatterns pins the NATS subject wildcards used for stream routing.
func TestSubjectPatterns(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "paladin.alerts.raw.>", inats.SubjectAlertsRaw)
	assert.Equal(t, "paladin.alerts.deduped.>", inats.SubjectAlertsDeduped)
	assert.Equal(t, "paladin.alerts.correlated.>", inats.SubjectAlertsCorrelated)
	assert.Equal(t, "paladin.alerts.triaged.>", inats.SubjectAlertsTriaged)
	assert.Equal(t, "paladin.alerts.analyzed.>", inats.SubjectAlertsAnalyzed)
	assert.Equal(t, "paladin.agent.work.>", inats.SubjectAgentWork)
	assert.Equal(t, "paladin.runbook.steps.>", inats.SubjectRunbookSteps)
	assert.Equal(t, "paladin.incidents.>", inats.SubjectIncidents)
	assert.Equal(t, "paladin.audit.>", inats.SubjectAudit)
	assert.Equal(t, "stream.>", inats.SubjectEvents)
	assert.Equal(t, "memory.consolidation.>", inats.SubjectMemory)
	assert.Equal(t, "dlq.>", inats.SubjectDLQ)
	assert.Equal(t, "evals.>", inats.SubjectEvals)
}

// TestSubjectPatterns_Distinct ensures no two subjects share the same value.
func TestSubjectPatterns_Distinct(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool, len(allSubjectPatterns))
	for _, p := range allSubjectPatterns {
		require.False(t, seen[p], "duplicate subject pattern: %q", p)
		seen[p] = true
	}
}

// TestSubjectPatterns_UseWildcard verifies that every subject pattern ends with
// the NATS full-wildcard ">" so that hierarchical routing is possible.
func TestSubjectPatterns_UseWildcard(t *testing.T) {
	t.Parallel()
	for _, p := range allSubjectPatterns {
		assert.True(t, strings.HasSuffix(p, ">"),
			"subject %q must end with NATS full-wildcard '>'", p)
	}
}

// TestSubjectPatterns_PaladinPrefix ensures the internal "paladin." namespace
// subjects all share that prefix. Stage 2 streams (stream./memory./dlq./evals.)
// are intentionally top-level per the spec and excluded from this check.
func TestSubjectPatterns_PaladinPrefix(t *testing.T) {
	t.Parallel()
	for _, p := range paladinPrefixedSubjects {
		assert.True(t, strings.HasPrefix(p, "paladin."),
			"subject %q must be in the 'paladin.' namespace", p)
	}
}

// TestStage2Subjects_TopLevel ensures Stage 2 streams use their spec-defined
// top-level prefixes rather than the internal `paladin.` namespace.
func TestStage2Subjects_TopLevel(t *testing.T) {
	t.Parallel()
	assert.True(t, strings.HasPrefix(inats.SubjectEvents, "stream."),
		"events subject must use top-level 'stream.' prefix")
	assert.True(t, strings.HasPrefix(inats.SubjectMemory, "memory."),
		"memory subject must use top-level 'memory.' prefix")
	assert.True(t, strings.HasPrefix(inats.SubjectDLQ, "dlq."),
		"dlq subject must use top-level 'dlq.' prefix")
	assert.True(t, strings.HasPrefix(inats.SubjectEvals, "evals."),
		"evals subject must use top-level 'evals.' prefix")
}
