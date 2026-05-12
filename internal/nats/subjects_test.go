package nats_test

import (
	"strings"
	"testing"

	natspkg "github.com/paladinai/paladinai/internal/nats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── AlertSubject ─────────────────────────────────────────────────────────────

func TestAlertSubject_ValidTokens_ReturnsCorrectSubject(t *testing.T) {
	got, err := natspkg.AlertSubject("tenant-a", "critical", "alertmanager")
	require.NoError(t, err)
	assert.Equal(t, "alerts.tenant-a.critical.alertmanager", got)
}

func TestAlertSubject_EmptyTenantID_ReturnsError(t *testing.T) {
	_, err := natspkg.AlertSubject("", "critical", "alertmanager")
	require.ErrorIs(t, err, natspkg.ErrInvalidToken)
}

func TestAlertSubject_DotInSeverity_ReturnsError(t *testing.T) {
	_, err := natspkg.AlertSubject("tenant-a", "crit.ical", "alertmanager")
	require.ErrorIs(t, err, natspkg.ErrInvalidToken)
}

func TestAlertSubject_WildcardInSource_ReturnsError(t *testing.T) {
	_, err := natspkg.AlertSubject("tenant-a", "critical", "*")
	require.ErrorIs(t, err, natspkg.ErrInvalidToken)
}

func TestAlertSubject_GTInSource_ReturnsError(t *testing.T) {
	_, err := natspkg.AlertSubject("tenant-a", "critical", ">")
	require.ErrorIs(t, err, natspkg.ErrInvalidToken)
}

func TestAlertSubject_ValidSeverities(t *testing.T) {
	for _, sev := range []string{"critical", "warning", "info"} {
		got, err := natspkg.AlertSubject("t1", sev, "datadog")
		require.NoError(t, err)
		assert.Equal(t, "alerts.t1."+sev+".datadog", got)
	}
}

// ─── AlertSubscribePattern ────────────────────────────────────────────────────

func TestAlertSubscribePattern_Valid(t *testing.T) {
	got, err := natspkg.AlertSubscribePattern("tenant-b")
	require.NoError(t, err)
	assert.Equal(t, "alerts.tenant-b.>", got)
}

func TestAlertSubscribePattern_EmptyTenant_ReturnsError(t *testing.T) {
	_, err := natspkg.AlertSubscribePattern("")
	require.ErrorIs(t, err, natspkg.ErrInvalidToken)
}

func TestAlertSubscribePattern_DotInTenant_ReturnsError(t *testing.T) {
	_, err := natspkg.AlertSubscribePattern("bad.tenant")
	require.ErrorIs(t, err, natspkg.ErrInvalidToken)
}

// ─── IncidentSubject ──────────────────────────────────────────────────────────

func TestIncidentSubject_ValidTokens(t *testing.T) {
	got, err := natspkg.IncidentSubject("tenant-a", "inc-001")
	require.NoError(t, err)
	assert.Equal(t, "incidents.tenant-a.inc-001", got)
}

func TestIncidentSubject_EmptyIncidentID_ReturnsError(t *testing.T) {
	_, err := natspkg.IncidentSubject("tenant-a", "")
	require.ErrorIs(t, err, natspkg.ErrInvalidToken)
}

func TestIncidentSubject_DotInIncidentID_ReturnsError(t *testing.T) {
	_, err := natspkg.IncidentSubject("tenant-a", "inc.001")
	require.ErrorIs(t, err, natspkg.ErrInvalidToken)
}

// ─── RunbookStepSubject ───────────────────────────────────────────────────────

func TestRunbookStepSubject_ValidTokens(t *testing.T) {
	got, err := natspkg.RunbookStepSubject("tenant-a", "inc-001", "step-3")
	require.NoError(t, err)
	assert.Equal(t, "runbooks.tenant-a.inc-001.step-3", got)
}

func TestRunbookStepSubject_EmptyStepID_ReturnsError(t *testing.T) {
	_, err := natspkg.RunbookStepSubject("tenant-a", "inc-001", "")
	require.ErrorIs(t, err, natspkg.ErrInvalidToken)
}

func TestRunbookStepSubject_WildcardInStep_ReturnsError(t *testing.T) {
	_, err := natspkg.RunbookStepSubject("tenant-a", "inc-001", "*")
	require.ErrorIs(t, err, natspkg.ErrInvalidToken)
}

// ─── WorkingMemoryKey ─────────────────────────────────────────────────────────

func TestWorkingMemoryKey_ValidTokens(t *testing.T) {
	got, err := natspkg.WorkingMemoryKey("tenant-a", "sess-42", "hypothesis")
	require.NoError(t, err)
	assert.Equal(t, "wm:tenant-a:sess-42:hypothesis", got)
}

func TestWorkingMemoryKey_EmptyKey_ReturnsError(t *testing.T) {
	_, err := natspkg.WorkingMemoryKey("tenant-a", "sess-42", "")
	require.Error(t, err)
}

func TestWorkingMemoryKey_EmptySession_ReturnsError(t *testing.T) {
	_, err := natspkg.WorkingMemoryKey("tenant-a", "", "k")
	require.Error(t, err)
}

// ─── ToolCacheKey ─────────────────────────────────────────────────────────────

func TestToolCacheKey_ValidTokens(t *testing.T) {
	got, err := natspkg.ToolCacheKey("tenant-a", "abc123")
	require.NoError(t, err)
	assert.Equal(t, "tool:l3:tenant-a:abc123", got)
}

func TestToolCacheKey_EmptyHash_ReturnsError(t *testing.T) {
	_, err := natspkg.ToolCacheKey("tenant-a", "")
	require.Error(t, err)
}

// ─── TenantConfigKeyPattern ───────────────────────────────────────────────────

func TestTenantConfigKeyPattern_Valid(t *testing.T) {
	got, err := natspkg.TenantConfigKeyPattern("tenant-a")
	require.NoError(t, err)
	assert.Equal(t, "config:tenant-a:*", got)
	assert.True(t, strings.HasSuffix(got, ":*"))
}

func TestTenantConfigKeyPattern_EmptyTenant_ReturnsError(t *testing.T) {
	_, err := natspkg.TenantConfigKeyPattern("")
	require.Error(t, err)
}

// ─── Subject hierarchy isolation ─────────────────────────────────────────────

func TestSubjects_DifferentTenantsProduceDifferentSubjects(t *testing.T) {
	s1, _ := natspkg.AlertSubject("tenant-a", "critical", "alertmanager")
	s2, _ := natspkg.AlertSubject("tenant-b", "critical", "alertmanager")
	assert.NotEqual(t, s1, s2)
}

func TestSubjects_AllContainTenantID(t *testing.T) {
	tid := "my-tenant"
	alert, _ := natspkg.AlertSubject(tid, "info", "slack")
	incident, _ := natspkg.IncidentSubject(tid, "i-1")
	runbook, _ := natspkg.RunbookStepSubject(tid, "i-1", "s-1")
	pattern, _ := natspkg.AlertSubscribePattern(tid)

	for _, s := range []string{alert, incident, runbook, pattern} {
		assert.Contains(t, s, tid, "subject %q should contain tenant ID", s)
	}
}
