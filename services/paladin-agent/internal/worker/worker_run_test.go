// Integration tests for Worker.Run using an in-process NATS JetStream server.
// Kept in package worker_test to exercise the exported surface only.
package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/agent"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// startJetStream boots an in-process NATS+JetStream server and creates the
// PALADIN_ALERTS stream that Worker.Run consumes from.
func startJetStream(t *testing.T) (jetstream.JetStream, *natsgo.Conn) {
	t.Helper()
	opts := &natsserver.Options{
		Host:      "127.0.0.1",
		Port:      -1,
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
	t.Cleanup(func() {
		srv.Shutdown()
		srv.WaitForShutdown()
	})

	nc, err := natsgo.Connect(srv.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { nc.Close() })

	js, err := jetstream.New(nc)
	require.NoError(t, err)

	_, err = js.CreateOrUpdateStream(context.Background(), jetstream.StreamConfig{
		Name:     "PALADIN_ALERTS",
		Subjects: []string{"paladin.alerts.correlated.>"},
		Storage:  jetstream.MemoryStorage,
	})
	require.NoError(t, err)

	return js, nc
}

// blockingTriager increments a counter, signals the first call via a channel,
// then returns the configured result/err. Used to confirm Worker.Run dispatches
// to a real triager when a message lands on the consumer.
type blockingTriager struct {
	mu      sync.Mutex
	calls   int32
	gotOne  chan struct{}
	result  *agent.TriageResult
	callErr error
}

func (b *blockingTriager) Triage(_ context.Context, _ *alert.AlertEnvelope) (*agent.TriageResult, error) {
	atomic.AddInt32(&b.calls, 1)
	b.mu.Lock()
	if b.gotOne != nil {
		select {
		case <-b.gotOne:
		default:
			close(b.gotOne)
		}
	}
	b.mu.Unlock()
	return b.result, b.callErr
}

type recordingPublisher struct {
	mu       sync.Mutex
	subjects []string
}

func (r *recordingPublisher) Publish(_ context.Context, subject string, _ []byte) (worker.PublishResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subjects = append(r.subjects, subject)
	return worker.PublishResult{}, nil
}

// TestRun_ConsumesAndProcessesMessage publishes a correlated envelope to
// JetStream, starts Worker.Run, and asserts the triager fires before cancel.
func TestRun_ConsumesAndProcessesMessage(t *testing.T) {
	js, _ := startJetStream(t)

	trig := &blockingTriager{
		result: &agent.TriageResult{ConfirmedSeverity: "P2"},
		gotOne: make(chan struct{}),
	}
	pub := &recordingPublisher{}
	w := worker.New(trig, pub, 2*time.Second, 1, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runErr := make(chan error, 1)
	go func() { runErr <- w.Run(ctx, js, "test-consumer") }()

	// Wait until Run has created its durable consumer; only then will
	// DeliverNewPolicy admit our publish. Poll the consumer info.
	consumerUp := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := js.Consumer(context.Background(), "PALADIN_ALERTS", "test-consumer"); err == nil {
			consumerUp = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.True(t, consumerUp, "consumer was not created by Run")

	env := alert.AlertEnvelope{
		TenantID:    "t1",
		Fingerprint: "fp-run-1",
		Source:      alert.SourceAlertmanager,
		Status:      alert.StatusFiring,
		Severity:    alert.SeverityP2,
	}
	body, _ := json.Marshal(env)
	_, err := js.Publish(context.Background(), "paladin.alerts.correlated.t1.alertmanager", body)
	require.NoError(t, err)

	select {
	case <-trig.gotOne:
		// Triager fired — Run dispatched the message.
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("triager was not called within timeout")
	}

	cancel()
	select {
	case err := <-runErr:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}

	require.Greater(t, atomic.LoadInt32(&trig.calls), int32(0))
}

// TestRun_ContextCancelledImmediately ensures Run exits cleanly with the
// cancel error when ctx is cancelled before any message arrives.
func TestRun_ContextCancelledImmediately(t *testing.T) {
	js, _ := startJetStream(t)

	trig := &blockingTriager{result: &agent.TriageResult{ConfirmedSeverity: "P3"}}
	pub := &recordingPublisher{}
	w := worker.New(trig, pub, 1*time.Second, 1, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- w.Run(ctx, js, "cancel-consumer") }()

	// Give the consumer a moment to start, then cancel.
	time.Sleep(150 * time.Millisecond)
	cancel()

	select {
	case err := <-runErr:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}
}

// errJS implements only the StreamConsumerManager method that Run calls and
// returns a forced error from CreateOrUpdateConsumer. The remaining methods
// panic — Run must short-circuit before touching them.
type errJS struct {
	jetstream.JetStream
}

func (errJS) CreateOrUpdateConsumer(_ context.Context, _ string, _ jetstream.ConsumerConfig) (jetstream.Consumer, error) {
	return nil, errors.New("forced consumer create failure")
}

// TestRun_ConsumerCreateError surfaces the wrapping path when JetStream
// refuses to create the consumer (e.g. mis-named stream in production).
func TestRun_ConsumerCreateError(t *testing.T) {
	trig := &blockingTriager{result: &agent.TriageResult{ConfirmedSeverity: "P3"}}
	pub := &recordingPublisher{}
	w := worker.New(trig, pub, 1*time.Second, 1, zap.NewNop())

	err := w.Run(context.Background(), errJS{}, "bad-consumer")
	require.Error(t, err)
	require.Contains(t, err.Error(), "worker consumer create bad-consumer")
}
