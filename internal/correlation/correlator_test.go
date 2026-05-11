package correlation_test

import (
	"context"
	"errors"
	"strings"
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
	assert.True(t, strings.HasPrefix(env.CorrelationID, "corr-"))
}

func TestCorrelator_StoreErrorIsReturned(t *testing.T) {
	storeErr := errors.New("valkey: connection refused")
	c := correlation.New(&errStore{err: storeErr}, zap.NewNop())
	env := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp1"}

	err := c.Correlate(context.Background(), env)
	require.Error(t, err)
	assert.ErrorIs(t, err, storeErr)
	assert.Contains(t, err.Error(), "correlate")
}

func TestCorrelatedSubject_Format(t *testing.T) {
	t.Parallel()
	subject := correlation.CorrelatedSubject("acme-corp", "alertmanager")
	assert.Equal(t, "paladin.alerts.correlated.acme-corp.alertmanager", subject)
}

func TestCorrelatedSubject_VariesByTenantAndSource(t *testing.T) {
	t.Parallel()
	s1 := correlation.CorrelatedSubject("t1", "alertmanager")
	s2 := correlation.CorrelatedSubject("t2", "alertmanager")
	s3 := correlation.CorrelatedSubject("t1", "datadog")
	assert.NotEqual(t, s1, s2)
	assert.NotEqual(t, s1, s3)
	assert.True(t, strings.HasPrefix(s1, "paladin.alerts.correlated."))
}

func TestCorrelator_NoLabelsDontCollapse(t *testing.T) {
	// Two label-less alerts from the same tenant must NOT share a correlation group.
	// Without the fingerprint fallback, they would both hash to "tenant:T1" only.
	store := newMemStore()
	c := correlation.New(store, zap.NewNop())
	ctx := context.Background()

	env1 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp1", Labels: nil}
	env2 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp2", Labels: nil}

	require.NoError(t, c.Correlate(ctx, env1))
	require.NoError(t, c.Correlate(ctx, env2))

	assert.NotEqual(t, env1.CorrelationID, env2.CorrelationID,
		"label-less alerts with different fingerprints should not share a correlation group")
}
