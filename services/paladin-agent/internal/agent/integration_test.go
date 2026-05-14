package agent_test

import (
	"context"
	"testing"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaticIntegrationSpecialist_StripeWebhook(t *testing.T) {
	env := &alert.AlertEnvelope{
		Severity:    alert.SeverityP1,
		Title:       "Payment Service Down",
		Description: "All Stripe webhook deliveries failing",
		Labels:      map[string]string{"service": "payments"},
		Annotations: map[string]string{"description": "Stripe webhook deliveries failing"},
	}

	result, err := agent.StaticIntegrationSpecialist{}.DiagnoseIntegration(context.Background(), env)
	require.NoError(t, err)

	assert.Equal(t, "stripe", result.Provider)
	assert.Equal(t, "webhook_delivery_failure", result.FailureMode)
	assert.Contains(t, result.Checks, "verify webhook endpoint health")
	assert.True(t, result.NeedsHuman)
}

func TestStaticIntegrationSpecialist_OAuthFailure(t *testing.T) {
	env := &alert.AlertEnvelope{
		Severity:    alert.SeverityP3,
		Title:       "OAuth token refresh failed",
		Description: "provider returned 401 unauthorized",
		Labels:      map[string]string{"service": "github-sync"},
		Annotations: map[string]string{},
	}

	result, err := agent.StaticIntegrationSpecialist{}.DiagnoseIntegration(context.Background(), env)
	require.NoError(t, err)

	assert.Equal(t, "github", result.Provider)
	assert.Equal(t, "credential_or_oauth_failure", result.FailureMode)
	assert.Contains(t, result.Checks, "refresh OAuth token if needed")
	assert.False(t, result.NeedsHuman)
}
