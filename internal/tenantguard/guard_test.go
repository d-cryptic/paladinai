package tenantguard_test

import (
	"strings"
	"testing"

	"github.com/paladinai/paladinai/internal/tenantguard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── CheckToolParams ──────────────────────────────────────────────────────────

func TestCheckToolParams_CleanParams(t *testing.T) {
	params := map[string]string{
		"namespace": "prod",
		"service":   "api",
		"query":     "rate(http_requests_total[5m])",
	}
	assert.NoError(t, tenantguard.CheckToolParams(params))
}

func TestCheckToolParams_ContainsTenantID(t *testing.T) {
	params := map[string]string{
		"namespace": "prod",
		"tenant_id": "evil-tenant",
	}
	err := tenantguard.CheckToolParams(params)
	require.Error(t, err)
	assert.ErrorIs(t, err, tenantguard.ErrConfusedDeputy)
}

func TestCheckToolParams_CamelCaseTenantId(t *testing.T) {
	params := map[string]string{"tenantId": "evil"}
	err := tenantguard.CheckToolParams(params)
	assert.ErrorIs(t, err, tenantguard.ErrConfusedDeputy)
}

func TestCheckToolParams_HyphenatedTenantId(t *testing.T) {
	params := map[string]string{"tenant-id": "evil"}
	err := tenantguard.CheckToolParams(params)
	assert.ErrorIs(t, err, tenantguard.ErrConfusedDeputy)
}

func TestCheckToolParams_LowercaseTenantid(t *testing.T) {
	params := map[string]string{"tenantid": "evil"}
	err := tenantguard.CheckToolParams(params)
	assert.ErrorIs(t, err, tenantguard.ErrConfusedDeputy)
}

func TestCheckToolParams_EmptyParams(t *testing.T) {
	assert.NoError(t, tenantguard.CheckToolParams(nil))
	assert.NoError(t, tenantguard.CheckToolParams(map[string]string{}))
}

// ─── CheckToolParamsAny ───────────────────────────────────────────────────────

func TestCheckToolParamsAny_CleanParams(t *testing.T) {
	params := map[string]any{
		"window": "5m",
		"labels": map[string]string{"service": "api"},
	}
	assert.NoError(t, tenantguard.CheckToolParamsAny(params))
}

func TestCheckToolParamsAny_ContainsTenantID(t *testing.T) {
	params := map[string]any{
		"namespace": "prod",
		"tenant_id": "injected",
	}
	err := tenantguard.CheckToolParamsAny(params)
	assert.ErrorIs(t, err, tenantguard.ErrConfusedDeputy)
}

// ─── WrapAlertContent ─────────────────────────────────────────────────────────

func TestWrapAlertContent_WrapsCorrectly(t *testing.T) {
	wrapped := tenantguard.WrapAlertContent("High CPU on api-1")
	assert.True(t, strings.HasPrefix(wrapped, "<ALERT>"), "should start with <ALERT>")
	assert.True(t, strings.HasSuffix(wrapped, "</ALERT>"), "should end with </ALERT>")
	assert.Contains(t, wrapped, "High CPU on api-1")
}

func TestWrapAlertContent_EmptyContent(t *testing.T) {
	wrapped := tenantguard.WrapAlertContent("")
	assert.Contains(t, wrapped, "<ALERT>")
	assert.Contains(t, wrapped, "</ALERT>")
}

// ─── ValidateTenantConsistency ────────────────────────────────────────────────

func TestValidateTenantConsistency_Matching(t *testing.T) {
	err := tenantguard.ValidateTenantConsistency("tenant-1", "tenant-1", "tenant-1")
	assert.NoError(t, err)
}

func TestValidateTenantConsistency_Mismatch(t *testing.T) {
	err := tenantguard.ValidateTenantConsistency("tenant-1", "tenant-1", "tenant-EVIL")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant_id mismatch")
}

func TestValidateTenantConsistency_EmptyClaimedIgnored(t *testing.T) {
	// Empty claimed values are allowed (field not present in payload)
	err := tenantguard.ValidateTenantConsistency("tenant-1", "", "tenant-1", "")
	assert.NoError(t, err)
}

func TestValidateTenantConsistency_NoClaimed(t *testing.T) {
	err := tenantguard.ValidateTenantConsistency("tenant-1")
	assert.NoError(t, err)
}

// ─── TrustedBoundarySystemPrompt ─────────────────────────────────────────────

func TestTrustedBoundarySystemPrompt_NotEmpty(t *testing.T) {
	assert.NotEmpty(t, tenantguard.TrustedBoundarySystemPrompt)
	assert.Contains(t, strings.ToLower(tenantguard.TrustedBoundarySystemPrompt), "alert")
}
