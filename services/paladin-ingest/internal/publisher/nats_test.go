package publisher_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/services/paladin-ingest/internal/publisher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeJetStream records the last Publish call.
// Each test constructs its own instance — no shared state, safe for t.Parallel.
type fakeJetStream struct {
	lastSubject string
	lastData    []byte
	returnErr   error
	seq         uint64
}

func (f *fakeJetStream) Publish(_ context.Context, subject string, data []byte) (publisher.PublishResult, error) {
	f.lastSubject = subject
	f.lastData = data
	if f.returnErr != nil {
		return publisher.PublishResult{}, f.returnErr
	}
	return publisher.PublishResult{Sequence: f.seq}, nil
}

func newTestEnvelope(tenantID string) alert.AlertEnvelope {
	labels := map[string]string{"alertname": "CPUHigh", "env": "prod"}
	return alert.AlertEnvelope{
		TenantID:    tenantID,
		Source:      alert.SourceAlertmanager,
		Fingerprint: alert.ComputeFingerprint(alert.SourceAlertmanager, labels),
		Labels:      labels,
		StartsAt:    time.Now(),
	}
}

func TestPublishAlert_CallsPublishWithCorrectSubject(t *testing.T) {
	t.Parallel()
	fake := &fakeJetStream{seq: 1}
	pub := publisher.NewFromJS(fake, zap.NewNop())

	env := newTestEnvelope("acme-corp")
	require.NoError(t, pub.PublishAlert(context.Background(), env))
	assert.Equal(t, "paladin.alerts.raw.acme-corp.alertmanager", fake.lastSubject)
}

func TestPublishAlert_PayloadRoundTrips(t *testing.T) {
	t.Parallel()
	fake := &fakeJetStream{seq: 2}
	pub := publisher.NewFromJS(fake, zap.NewNop())

	env := newTestEnvelope("acme-corp")
	require.NoError(t, pub.PublishAlert(context.Background(), env))

	var decoded alert.AlertEnvelope
	require.NoError(t, json.Unmarshal(fake.lastData, &decoded))
	assert.Equal(t, env.TenantID, decoded.TenantID)
	assert.Equal(t, env.Fingerprint, decoded.Fingerprint)
	assert.Equal(t, env.Labels, decoded.Labels)
}

func TestPublishAlert_NATSErrorPropagated(t *testing.T) {
	t.Parallel()
	natsErr := errors.New("jetstream: connection refused")
	fake := &fakeJetStream{returnErr: natsErr}
	pub := publisher.NewFromJS(fake, zap.NewNop())

	err := pub.PublishAlert(context.Background(), newTestEnvelope("acme-corp"))
	require.Error(t, err)
	assert.ErrorIs(t, err, natsErr)
	assert.Contains(t, err.Error(), "nats publish")
}

func TestPublishAlert_InvalidTenantReturnsError(t *testing.T) {
	t.Parallel()
	fake := &fakeJetStream{}
	pub := publisher.NewFromJS(fake, zap.NewNop())

	env := newTestEnvelope("") // empty string fails ValidateTenantID
	err := pub.PublishAlert(context.Background(), env)
	require.Error(t, err, "invalid tenant ID must return an error, not panic")
	assert.Contains(t, err.Error(), "invalid tenant")
	// Fake must not have been called.
	assert.Empty(t, fake.lastSubject)
}

func TestPublishAlert_CancelledContextReturnsError(t *testing.T) {
	t.Parallel()
	fake := &fakeJetStream{}
	pub := publisher.NewFromJS(fake, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before publish
	err := pub.PublishAlert(ctx, newTestEnvelope("acme-corp"))
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	// Fake must not have been called.
	assert.Empty(t, fake.lastSubject)
}

func TestPublishAlert_SubjectVariesByTenant(t *testing.T) {
	t.Parallel()
	for _, tenant := range []string{"alpha", "beta", "gamma-corp"} {
		t.Run(tenant, func(t *testing.T) {
			t.Parallel()
			fake := &fakeJetStream{seq: 1}
			pub := publisher.NewFromJS(fake, zap.NewNop())
			require.NoError(t, pub.PublishAlert(context.Background(), newTestEnvelope(tenant)))
			assert.Contains(t, fake.lastSubject, tenant)
		})
	}
}

func TestPublishAlert_SubjectVariesBySource(t *testing.T) {
	t.Parallel()
	cases := []struct {
		source  alert.Source
		wantSub string
	}{
		{alert.SourceAlertmanager, "paladin.alerts.raw.acme-corp.alertmanager"},
		{alert.SourceDatadog, "paladin.alerts.raw.acme-corp.datadog"},
		{alert.SourcePagerDuty, "paladin.alerts.raw.acme-corp.pagerduty"},
	}
	for _, tc := range cases {
		t.Run(string(tc.source), func(t *testing.T) {
			t.Parallel()
			fake := &fakeJetStream{seq: 1}
			pub := publisher.NewFromJS(fake, zap.NewNop())
			env := newTestEnvelope("acme-corp")
			env.Source = tc.source
			require.NoError(t, pub.PublishAlert(context.Background(), env))
			assert.Equal(t, tc.wantSub, fake.lastSubject)
		})
	}
}
