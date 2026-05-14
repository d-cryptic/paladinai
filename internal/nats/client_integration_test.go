package nats_test

import (
	"context"
	"strings"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	natsgo "github.com/nats-io/nats.go"
	inats "github.com/paladinai/paladinai/internal/nats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// startInProcessJetStream boots an in-process NATS server with JetStream
// enabled and returns its client URL plus a teardown function. The server's
// store directory is created inside t.TempDir so files are cleaned up after
// each test automatically.
func startInProcessJetStream(t *testing.T) (string, func()) {
	t.Helper()
	opts := &natsserver.Options{
		Host:      "127.0.0.1",
		Port:      -1, // random
		JetStream: true,
		StoreDir:  t.TempDir(),
		NoLog:     true,
		NoSigs:    true,
	}
	srv, err := natsserver.NewServer(opts)
	require.NoError(t, err)

	go srv.Start()
	if !srv.ReadyForConnections(5 * time.Second) {
		srv.Shutdown()
		t.Fatal("nats server did not become ready in time")
	}

	teardown := func() {
		srv.Shutdown()
		srv.WaitForShutdown()
	}
	return srv.ClientURL(), teardown
}

// TestConnect_EnsuresAllStreams verifies the full Connect path:
// NATS connect, JetStream init, and idempotent stream creation.
func TestConnect_EnsuresAllStreams(t *testing.T) {
	url, stop := startInProcessJetStream(t)
	defer stop()

	log := zap.NewNop()
	client, err := inats.Connect(url, log)
	require.NoError(t, err)
	require.NotNil(t, client)
	defer client.Close()

	// JS() / Conn() accessors should be non-nil after Connect.
	assert.NotNil(t, client.JS(), "JS() must return a non-nil JetStream context")
	assert.NotNil(t, client.Conn(), "Conn() must return the raw NATS connection")
	assert.True(t, client.Conn().IsConnected())

	// All declared streams must exist.
	js := client.JS()
	ctx := context.Background()
	for _, name := range []string{
		inats.StreamAlerts,
		inats.StreamAgentWork,
		inats.StreamRunbookSteps,
		inats.StreamIncidents,
		inats.StreamAudit,
		inats.StreamEvents,
		inats.StreamMemory,
		inats.StreamDLQ,
		inats.StreamEvals,
	} {
		stream, err := js.Stream(ctx, name)
		require.NoError(t, err, "stream %s must exist", name)
		info, err := stream.Info(ctx)
		require.NoError(t, err)
		assert.Equal(t, name, info.Config.Name)
	}
}

// TestConnect_IsIdempotent calls Connect twice against the same server.
// The second call must succeed (CreateOrUpdateStream is idempotent).
func TestConnect_IsIdempotent(t *testing.T) {
	url, stop := startInProcessJetStream(t)
	defer stop()

	c1, err := inats.Connect(url, zap.NewNop())
	require.NoError(t, err)
	c1.Close()

	c2, err := inats.Connect(url, zap.NewNop())
	require.NoError(t, err)
	defer c2.Close()
}

func TestConnectWithContext_CanceledContext(t *testing.T) {
	url, stop := startInProcessJetStream(t)
	defer stop()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client, err := inats.ConnectWithContext(ctx, url, zap.NewNop())
	require.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "context canceled")
}

// TestConnect_BadURL exercises the failure path when NATS is unreachable.
func TestConnect_BadURL(t *testing.T) {
	// Use NoReconnect via an explicit invalid URL so Connect errors fast.
	// nats.Connect blocks retrying by default; use a syntactically invalid URL.
	_, err := inats.Connect("nats://127.0.0.1:1", zap.NewNop())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nats connect")
}

// TestPublish_RoundTrips publishes a message to PALADIN_ALERTS and verifies
// it lands in the stream.
func TestPublish_RoundTrips(t *testing.T) {
	url, stop := startInProcessJetStream(t)
	defer stop()

	client, err := inats.Connect(url, zap.NewNop())
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()
	ack, err := client.Publish(ctx, "paladin.alerts.raw.tenant1.alertmanager", []byte(`{"hello":"world"}`))
	require.NoError(t, err)
	require.NotNil(t, ack)
	assert.Equal(t, inats.StreamAlerts, ack.Stream)
	assert.Equal(t, uint64(1), ack.Sequence)
}

func TestPublish_AnalyzedAlertRoundTrips(t *testing.T) {
	url, stop := startInProcessJetStream(t)
	defer stop()

	client, err := inats.Connect(url, zap.NewNop())
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()
	ack, err := client.Publish(ctx, "paladin.alerts.analyzed.tenant1.alertmanager", []byte(`{"hello":"rca"}`))
	require.NoError(t, err)
	require.NotNil(t, ack)
	assert.Equal(t, inats.StreamAlerts, ack.Stream)
}

// TestPublish_NoStreamReturnsError publishes to a subject without a backing
// stream and asserts that the wrapped error is returned.
func TestPublish_NoStreamReturnsError(t *testing.T) {
	url, stop := startInProcessJetStream(t)
	defer stop()

	client, err := inats.Connect(url, zap.NewNop())
	require.NoError(t, err)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = client.Publish(ctx, "no.such.subject", []byte("x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "publish to no.such.subject")
}

// TestPublishDLQ_RoutesToEscapedSubject exercises the DLQ subject builder and
// confirms the message reaches the PALADIN_DLQ stream.
func TestPublishDLQ_RoutesToEscapedSubject(t *testing.T) {
	url, stop := startInProcessJetStream(t)
	defer stop()

	client, err := inats.Connect(url, zap.NewNop())
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()
	ack, err := client.PublishDLQ(ctx, "acme", "paladin.alerts.correlated.acme.alertmanager", []byte(`{"err":"boom"}`))
	require.NoError(t, err)
	require.NotNil(t, ack)
	assert.Equal(t, inats.StreamDLQ, ack.Stream)
}

func TestPublishDLQ_ValidatesInputs(t *testing.T) {
	url, stop := startInProcessJetStream(t)
	defer stop()

	client, err := inats.Connect(url, zap.NewNop())
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	_, err = client.PublishDLQ(ctx, "", "paladin.alerts.x", []byte("x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant")

	_, err = client.PublishDLQ(ctx, "bad.tenant", "paladin.alerts.x", []byte("x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant")

	_, err = client.PublishDLQ(ctx, "tenant", "", []byte("x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "originalSubject")
}

// TestPublishDLQ_EscapesOriginalSubject verifies the escape rule by
// inspecting the actual subject the message lands on. We use a core NATS
// subscriber on the dlq.> wildcard to capture it.
func TestPublishDLQ_EscapesOriginalSubject(t *testing.T) {
	url, stop := startInProcessJetStream(t)
	defer stop()

	// Plain NATS subscriber to capture the routed subject before JetStream ack.
	nc, err := natsgo.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	got := make(chan string, 1)
	sub, err := nc.Subscribe("dlq.>", func(m *natsgo.Msg) {
		select {
		case got <- m.Subject:
		default:
		}
	})
	require.NoError(t, err)
	defer func() { _ = sub.Unsubscribe() }()

	client, err := inats.Connect(url, zap.NewNop())
	require.NoError(t, err)
	defer client.Close()

	_, err = client.PublishDLQ(context.Background(), "acme",
		"paladin.alerts.>.acme.*", []byte("x"))
	require.NoError(t, err)

	select {
	case subject := <-got:
		// dots in the original subject must collapse to underscores.
		assert.True(t, strings.HasPrefix(subject, "dlq.acme."),
			"DLQ subject must start with dlq.<tenant>.: %s", subject)
		assert.False(t, strings.Contains(subject[len("dlq.acme."):], "."),
			"original-subject token must not contain dots: %s", subject)
		assert.NotContains(t, subject[len("dlq.acme."):], "*")
		assert.NotContains(t, subject[len("dlq.acme."):], ">")
		assert.Contains(t, subject, "paladin_alerts___acme__")
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive DLQ message")
	}
}

// TestClose_Drains ensures Close() runs without panicking and the connection
// transitions out of the connected state.
func TestClose_Drains(t *testing.T) {
	url, stop := startInProcessJetStream(t)
	defer stop()

	client, err := inats.Connect(url, zap.NewNop())
	require.NoError(t, err)
	require.True(t, client.Conn().IsConnected())

	client.Close()
	// Drain runs asynchronously; wait briefly for the closed state.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if client.Conn().IsClosed() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	assert.True(t, client.Conn().IsClosed(), "connection should be closed after Close()")
}
