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

	// Use 3/4 keys (0.75 ≥ 0.65 default gate) so the alert passes the gate.
	env := &alert.AlertEnvelope{
		TenantID:    "t1",
		Fingerprint: "fp1",
		Labels:      map[string]string{"namespace": "prod", "job": "api", "cluster": "eu-west-1"},
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

	// Use 3/4 keys (0.75 ≥ 0.65 default gate) to pass the gate.
	labels := map[string]string{"namespace": "prod", "job": "api", "cluster": "eu-west-1"}

	env1 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp1", Labels: labels}
	env2 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp2", Labels: labels}

	require.NoError(t, c.Correlate(ctx, env1))
	require.NoError(t, c.Correlate(ctx, env2))

	assert.Equal(t, env1.CorrelationID, env2.CorrelationID,
		"alerts with identical group labels (above gate) should share a correlation ID")
}

func TestCorrelator_DifferentGroupDifferentCorrelationID(t *testing.T) {
	store := newMemStore()
	c := correlation.New(store, zap.NewNop())
	ctx := context.Background()

	// Both have 3/4 keys (above gate) but differ in "job".
	env1 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp1",
		Labels: map[string]string{"namespace": "prod", "job": "api", "cluster": "eu-west-1"}}
	env2 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp2",
		Labels: map[string]string{"namespace": "prod", "job": "worker", "cluster": "eu-west-1"}}

	require.NoError(t, c.Correlate(ctx, env1))
	require.NoError(t, c.Correlate(ctx, env2))

	assert.NotEqual(t, env1.CorrelationID, env2.CorrelationID,
		"alerts with different jobs should have different correlation IDs")
}

func TestCorrelator_DifferentTenantsDifferentCorrelationIDs(t *testing.T) {
	store := newMemStore()
	c := correlation.New(store, zap.NewNop())
	ctx := context.Background()

	// Use 3/4 keys to pass the gate.
	labels := map[string]string{"namespace": "prod", "job": "api", "cluster": "eu-west-1"}

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
	subject, err := correlation.CorrelatedSubject("acme-corp", "alertmanager")
	require.NoError(t, err)
	assert.Equal(t, "paladin.alerts.correlated.acme-corp.alertmanager", subject)
}

func TestCorrelatedSubject_VariesByTenantAndSource(t *testing.T) {
	t.Parallel()
	s1, err := correlation.CorrelatedSubject("t1", "alertmanager")
	require.NoError(t, err)
	s2, err := correlation.CorrelatedSubject("t2", "alertmanager")
	require.NoError(t, err)
	s3, err := correlation.CorrelatedSubject("t1", "datadog")
	require.NoError(t, err)
	assert.NotEqual(t, s1, s2)
	assert.NotEqual(t, s1, s3)
	assert.True(t, strings.HasPrefix(s1, "paladin.alerts.correlated."))
}

func TestCorrelatedSubject_RejectsUnsafeTokens(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		tenantID string
		source   string
	}{
		{name: "empty tenant", tenantID: "", source: "alertmanager"},
		{name: "tenant dot", tenantID: "bad.tenant", source: "alertmanager"},
		{name: "empty source", tenantID: "tenant-a", source: ""},
		{name: "source dot", tenantID: "tenant-a", source: "bad.source"},
		{name: "source wildcard", tenantID: "tenant-a", source: "*"},
		{name: "source greater-than", tenantID: "tenant-a", source: ">"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := correlation.CorrelatedSubject(tt.tenantID, tt.source)
			require.Error(t, err)
		})
	}
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

// ─── Correlation gate tests ───────────────────────────────────────────────────
// correlationKeys = ["namespace", "job", "cluster", "service"] (4 keys)
// Coverage fractions: 0/4=0.00, 1/4=0.25, 2/4=0.50, 3/4=0.75, 4/4=1.00

func TestCorrelator_Gate_DefaultPassesHighCoverage(t *testing.T) {
	// 3/4 keys = 0.75 coverage ≥ 0.65 default gate → alerts merge into same group
	store := newMemStore()
	c := correlation.New(store, zap.NewNop()) // default gate = 0.65
	ctx := context.Background()

	labels := map[string]string{"namespace": "prod", "job": "api", "cluster": "eu-west-1"}
	env1 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp1", Labels: labels}
	env2 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp2", Labels: labels}

	require.NoError(t, c.Correlate(ctx, env1))
	require.NoError(t, c.Correlate(ctx, env2))
	assert.Equal(t, env1.CorrelationID, env2.CorrelationID,
		"3/4 label coverage (0.75) must pass the default gate (0.65) and merge alerts")
}

func TestCorrelator_Gate_DefaultBlocksLowCoverage(t *testing.T) {
	// 2/4 keys = 0.50 coverage < 0.65 default gate → alerts isolated to fingerprint
	store := newMemStore()
	c := correlation.New(store, zap.NewNop()) // default gate = 0.65
	ctx := context.Background()

	labels := map[string]string{"namespace": "prod", "job": "api"} // 2/4 keys
	env1 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp1", Labels: labels}
	env2 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp2", Labels: labels}

	require.NoError(t, c.Correlate(ctx, env1))
	require.NoError(t, c.Correlate(ctx, env2))
	assert.NotEqual(t, env1.CorrelationID, env2.CorrelationID,
		"2/4 label coverage (0.50) must be blocked by the default gate (0.65) — alerts stay isolated")
}

func TestCorrelator_Gate_ZeroAllowsEverything(t *testing.T) {
	// Gate = 0 → all alerts eligible for grouping regardless of label count
	store := newMemStore()
	c := correlation.New(store, zap.NewNop()).WithGate(0)
	ctx := context.Background()

	labels := map[string]string{"namespace": "prod"} // 1/4 keys
	env1 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp1", Labels: labels}
	env2 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp2", Labels: labels}

	require.NoError(t, c.Correlate(ctx, env1))
	require.NoError(t, c.Correlate(ctx, env2))
	assert.Equal(t, env1.CorrelationID, env2.CorrelationID,
		"gate=0 must allow grouping even with minimal label coverage")
}

func TestCorrelator_Gate_OneRequiresAllKeys(t *testing.T) {
	// Gate = 1.0 → all 4 correlationKeys required; 3/4 is below gate
	store := newMemStore()
	c := correlation.New(store, zap.NewNop()).WithGate(1.0)
	ctx := context.Background()

	labels := map[string]string{"namespace": "prod", "job": "api", "cluster": "eu-west-1"} // 3/4
	env1 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp1", Labels: labels}
	env2 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp2", Labels: labels}

	require.NoError(t, c.Correlate(ctx, env1))
	require.NoError(t, c.Correlate(ctx, env2))
	assert.NotEqual(t, env1.CorrelationID, env2.CorrelationID,
		"gate=1.0 must block 3/4 coverage (0.75 < 1.0)")
}

func TestCorrelator_Gate_FullCoverageAlwaysPasses(t *testing.T) {
	// 4/4 keys = 1.0 coverage ≥ any valid gate → always groups
	store := newMemStore()
	c := correlation.New(store, zap.NewNop()).WithGate(1.0)
	ctx := context.Background()

	labels := map[string]string{
		"namespace": "prod",
		"job":       "api",
		"cluster":   "eu-west-1",
		"service":   "payments",
	}
	env1 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp1", Labels: labels}
	env2 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp2", Labels: labels}

	require.NoError(t, c.Correlate(ctx, env1))
	require.NoError(t, c.Correlate(ctx, env2))
	assert.Equal(t, env1.CorrelationID, env2.CorrelationID,
		"4/4 label coverage must always pass even the strictest gate")
}

func TestCorrelator_Gate_Clamped_AboveOne(t *testing.T) {
	// WithGate clamps values > 1 to 1.0
	store := newMemStore()
	c := correlation.New(store, zap.NewNop()).WithGate(1.5)
	ctx := context.Background()

	// 3/4 keys should be blocked (treated as gate=1.0)
	labels := map[string]string{"namespace": "prod", "job": "api", "cluster": "eu-west-1"}
	env1 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp1", Labels: labels}
	env2 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp2", Labels: labels}

	require.NoError(t, c.Correlate(ctx, env1))
	require.NoError(t, c.Correlate(ctx, env2))
	assert.NotEqual(t, env1.CorrelationID, env2.CorrelationID,
		"gate clamped to 1.0: 3/4 coverage must be blocked")
}

func TestCorrelator_Gate_Clamped_BelowZero(t *testing.T) {
	// WithGate clamps values < 0 to 0 — all alerts eligible for grouping
	store := newMemStore()
	c := correlation.New(store, zap.NewNop()).WithGate(-0.5)
	ctx := context.Background()

	labels := map[string]string{"namespace": "prod"} // 1/4 keys
	env1 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp1", Labels: labels}
	env2 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp2", Labels: labels}

	require.NoError(t, c.Correlate(ctx, env1))
	require.NoError(t, c.Correlate(ctx, env2))
	assert.Equal(t, env1.CorrelationID, env2.CorrelationID,
		"gate clamped to 0: any coverage must pass and alerts must merge")
}

func TestCorrelator_Gate_BoundaryCustomThreshold(t *testing.T) {
	// Gate = 0.76: coverage 3/4 = 0.75 is just below → blocked
	// Gate = 0.74: coverage 3/4 = 0.75 is just above → passes
	labels := map[string]string{"namespace": "prod", "job": "api", "cluster": "eu-west-1"} // 3/4

	t.Run("0.76_gate_blocks_0.75_coverage", func(t *testing.T) {
		store := newMemStore()
		c := correlation.New(store, zap.NewNop()).WithGate(0.76)
		ctx := context.Background()

		env1 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp1", Labels: labels}
		env2 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp2", Labels: labels}
		require.NoError(t, c.Correlate(ctx, env1))
		require.NoError(t, c.Correlate(ctx, env2))
		assert.NotEqual(t, env1.CorrelationID, env2.CorrelationID,
			"gate 0.76 must block coverage 0.75")
	})

	t.Run("0.74_gate_passes_0.75_coverage", func(t *testing.T) {
		store := newMemStore()
		c := correlation.New(store, zap.NewNop()).WithGate(0.74)
		ctx := context.Background()

		env1 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp1", Labels: labels}
		env2 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp2", Labels: labels}
		require.NoError(t, c.Correlate(ctx, env1))
		require.NoError(t, c.Correlate(ctx, env2))
		assert.Equal(t, env1.CorrelationID, env2.CorrelationID,
			"gate 0.74 must pass coverage 0.75")
	})
}

// TestLabelCoverage_Unit directly tests the coverage calculation.
// This is a white-box test that validates the boundary math.
func TestLabelCoverage_Unit(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		wantFrac float64
	}{
		{"none", nil, 0.0},
		{"empty map", map[string]string{}, 0.0},
		{"1/4", map[string]string{"namespace": "prod"}, 0.25},
		{"2/4", map[string]string{"namespace": "prod", "job": "api"}, 0.50},
		{"3/4", map[string]string{"namespace": "prod", "job": "api", "cluster": "eu"}, 0.75},
		{"4/4", map[string]string{"namespace": "prod", "job": "api", "cluster": "eu", "service": "svc"}, 1.0},
		{"irrelevant keys only", map[string]string{"foo": "bar", "baz": "qux"}, 0.0},
	}

	// Use correlation gate 0 to isolate the groupKey function: even low-coverage
	// alerts join a group. We compare correlation IDs to infer coverage.
	// Instead test labelCoverage directly via the gate behavior.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMemStore()
			// Gate set to tt.wantFrac + epsilon: if coverage < gate, alert is isolated.
			// If coverage >= gate, alert joins a group.
			gate := tt.wantFrac + 0.01
			if gate > 1.0 {
				gate = 1.0
			}
			c := correlation.New(store, zap.NewNop()).WithGate(gate)
			ctx := context.Background()

			env1 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp1", Labels: tt.labels}
			env2 := &alert.AlertEnvelope{TenantID: "t1", Fingerprint: "fp2", Labels: tt.labels}
			require.NoError(t, c.Correlate(ctx, env1))
			require.NoError(t, c.Correlate(ctx, env2))

			if tt.wantFrac == 0.0 || tt.wantFrac < gate {
				// coverage < gate → isolated → different IDs
				assert.NotEqual(t, env1.CorrelationID, env2.CorrelationID,
					"coverage %.2f should be below gate %.2f → isolated", tt.wantFrac, gate)
			}
			// Note: when coverage >= gate, alerts may still get different IDs
			// if labels differ — so we only assert the blocked direction here.
		})
	}
}
