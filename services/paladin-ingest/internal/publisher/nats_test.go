package publisher_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/services/paladin-ingest/internal/publisher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeJetStream records the last Publish call and returns a configured response.
type fakeJetStream struct {
	lastSubject string
	lastData    []byte
	returnErr   error
	seq         uint64
}

func (f *fakeJetStream) Publish(_ context.Context, subject string, data []byte) (*jetstream.PubAck, error) {
	f.lastSubject = subject
	f.lastData = data
	if f.returnErr != nil {
		return nil, f.returnErr
	}
	return &jetstream.PubAck{Stream: "PALADIN_ALERTS", Sequence: f.seq}, nil
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
	pub := publisher.NewWithClient(fake, zap.NewNop())

	env := newTestEnvelope("acme-corp")
	require.NoError(t, pub.PublishAlert(context.Background(), env))

	assert.Equal(t, "paladin.alerts.raw.acme-corp.alertmanager", fake.lastSubject)
}

func TestPublishAlert_PayloadIsValidJSON(t *testing.T) {
	t.Parallel()
	fake := &fakeJetStream{seq: 2}
	pub := publisher.NewWithClient(fake, zap.NewNop())

	env := newTestEnvelope("acme-corp")
	require.NoError(t, pub.PublishAlert(context.Background(), env))

	var decoded alert.AlertEnvelope
	require.NoError(t, json.Unmarshal(fake.lastData, &decoded))
	assert.Equal(t, env.TenantID, decoded.TenantID)
	assert.Equal(t, env.Fingerprint, decoded.Fingerprint)
}

func TestPublishAlert_PayloadContainsTenantID(t *testing.T) {
	t.Parallel()
	fake := &fakeJetStream{seq: 3}
	pub := publisher.NewWithClient(fake, zap.NewNop())

	env := newTestEnvelope("acme-corp")
	require.NoError(t, pub.PublishAlert(context.Background(), env))

	assert.Contains(t, string(fake.lastData), "acme-corp")
}

func TestPublishAlert_NATSErrorPropagated(t *testing.T) {
	t.Parallel()
	natsErr := errors.New("jetstream: connection refused")
	fake := &fakeJetStream{returnErr: natsErr}
	pub := publisher.NewWithClient(fake, zap.NewNop())

	env := newTestEnvelope("acme-corp")
	err := pub.PublishAlert(context.Background(), env)
	require.Error(t, err)
	assert.ErrorIs(t, err, natsErr)
	assert.Contains(t, err.Error(), "nats publish")
}

func TestPublishAlert_SubjectVariesByTenant(t *testing.T) {
	t.Parallel()
	tenants := []string{"alpha", "beta", "gamma-corp"}
	for _, tenant := range tenants {
		t.Run(tenant, func(t *testing.T) {
			t.Parallel()
			fake := &fakeJetStream{seq: 1}
			pub := publisher.NewWithClient(fake, zap.NewNop())
			env := newTestEnvelope(tenant)
			require.NoError(t, pub.PublishAlert(context.Background(), env))
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
			pub := publisher.NewWithClient(fake, zap.NewNop())
			env := newTestEnvelope("acme-corp")
			env.Source = tc.source
			require.NoError(t, pub.PublishAlert(context.Background(), env))
			assert.Equal(t, tc.wantSub, fake.lastSubject)
		})
	}
}
