package correlation_test

import (
	"context"
	"testing"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/internal/correlation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestCorrelator_AssignsCorrelationID(t *testing.T) {
	c := correlation.New(newMemStore(), zap.NewNop())
	ctx := context.Background()

	env := &alert.AlertEnvelope{
		TenantID:    "t1",
		Fingerprint: "fp1",
		Labels:      map[string]string{"namespace": "prod", "job": "api"},
	}

	err := c.Correlate(ctx, env)
	require.NoError(t, err)
	assert.NotEmpty(t, env.CorrelationID)
	assert.True(t, len(env.CorrelationID) > 5, "correlation ID should be non-trivial")
}

func TestCorrelator_SameGroupSameCorrelationID(t *testing.T) {
	store := newMemStore()
	c := correlation.New(store, zap.NewNop())
	ctx := context.Background()

	labels := map[string]string{"namespace": "prod", "job": "api"}

	env1 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp1", Labels: labels}
	env2 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp2", Labels: labels}

	require.NoError(t, c.Correlate(ctx, env1))
	require.NoError(t, c.Correlate(ctx, env2))

	assert.Equal(t, env1.CorrelationID, env2.CorrelationID,
		"alerts with identical group labels should share a correlation ID")
}

func TestCorrelator_DifferentGroupDifferentCorrelationID(t *testing.T) {
	store := newMemStore()
	c := correlation.New(store, zap.NewNop())
	ctx := context.Background()

	env1 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp1",
		Labels: map[string]string{"namespace": "prod", "job": "api"}}
	env2 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp2",
		Labels: map[string]string{"namespace": "prod", "job": "worker"}}

	require.NoError(t, c.Correlate(ctx, env1))
	require.NoError(t, c.Correlate(ctx, env2))

	assert.NotEqual(t, env1.CorrelationID, env2.CorrelationID,
		"alerts with different jobs should have different correlation IDs")
}

func TestCorrelator_DifferentTenantsDifferentCorrelationIDs(t *testing.T) {
	store := newMemStore()
	c := correlation.New(store, zap.NewNop())
	ctx := context.Background()

	labels := map[string]string{"namespace": "prod", "job": "api"}

	env1 := &alert.AlertEnvelope{TenantID: "tenant-A", Fingerprint: "fp1", Labels: labels}
	env2 := &alert.AlertEnvelope{TenantID: "tenant-B", Fingerprint: "fp2", Labels: labels}

	require.NoError(t, c.Correlate(ctx, env1))
	require.NoError(t, c.Correlate(ctx, env2))

	assert.NotEqual(t, env1.CorrelationID, env2.CorrelationID,
		"different tenants must never share correlation IDs")
}

func TestCorrelator_NoLabelsStillGetsCorrelationID(t *testing.T) {
	c := correlation.New(newMemStore(), zap.NewNop())
	ctx := context.Background()

	env := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp1", Labels: nil}

	err := c.Correlate(ctx, env)
	require.NoError(t, err)
	assert.NotEmpty(t, env.CorrelationID, "even label-less alerts should get a correlation ID")
}

func TestCorrelator_CorrelationIDPrefix(t *testing.T) {
	c := correlation.New(newMemStore(), zap.NewNop())
	env := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp1"}
	require.NoError(t, c.Correlate(context.Background(), env))
	assert.True(t, len(env.CorrelationID) > 5 && env.CorrelationID[:5] == "corr-")
}
