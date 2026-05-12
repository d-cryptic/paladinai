// White-box tests for Pipeline.Run.
//
// Run is wired to jetstream.JetStream, which is a sprawling interface. Rather
// than implement every method, we embed the interface and override only the
// calls Run actually makes (CreateOrUpdateConsumer + Consumer.Messages +
// MessagesContext.Next/Stop). Any unexpected call would panic on a nil
// receiver, which is the desired "fail loudly" behaviour for these tests.
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/paladinai/paladinai/internal/alert"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ─── JetStream fake ──────────────────────────────────────────────────────────

type fakeJetStream struct {
	jetstream.JetStream // embedded — unused methods would panic if called
	consumer            jetstream.Consumer
	createErr           error
	gotStream           string
	gotConfig           jetstream.ConsumerConfig
}

func (f *fakeJetStream) CreateOrUpdateConsumer(
	_ context.Context, stream string, cfg jetstream.ConsumerConfig,
) (jetstream.Consumer, error) {
	f.gotStream = stream
	f.gotConfig = cfg
	if f.createErr != nil {
		return nil, f.createErr
	}
	return f.consumer, nil
}

type fakeConsumer struct {
	jetstream.Consumer
	msgs       jetstream.MessagesContext
	messagesEr error
}

func (f *fakeConsumer) Messages(_ ...jetstream.PullMessagesOpt) (jetstream.MessagesContext, error) {
	if f.messagesEr != nil {
		return nil, f.messagesEr
	}
	return f.msgs, nil
}

// fakeMessages is a controllable MessagesContext. It returns queued messages
// one-by-one, then blocks until Stop is called (mimicking the real iterator's
// blocking semantics).
type fakeMessages struct {
	mu      sync.Mutex
	queue   []jetstream.Msg
	stopped chan struct{}
	once    sync.Once
	nextErr error // returned once after the queue drains
}

func newFakeMessages(msgs []jetstream.Msg, nextErr error) *fakeMessages {
	return &fakeMessages{queue: msgs, stopped: make(chan struct{}), nextErr: nextErr}
}

func (f *fakeMessages) Next() (jetstream.Msg, error) {
	f.mu.Lock()
	if len(f.queue) > 0 {
		m := f.queue[0]
		f.queue = f.queue[1:]
		f.mu.Unlock()
		return m, nil
	}
	f.mu.Unlock()

	// Drained — return a terminating error (or block until Stop is called).
	if f.nextErr != nil {
		return nil, f.nextErr
	}
	<-f.stopped
	return nil, jetstream.ErrMsgIteratorClosed
}

func (f *fakeMessages) Stop() {
	f.once.Do(func() { close(f.stopped) })
}

func (f *fakeMessages) Drain() { f.Stop() }

// ─── Run tests ───────────────────────────────────────────────────────────────

func TestRun_ConsumerCreateError_IsWrapped(t *testing.T) {
	t.Parallel()
	js := &fakeJetStream{createErr: errors.New("stream missing")}
	p := New(&localDedup{}, localCorr{}, &localPub{}, zap.NewNop())

	err := p.Run(context.Background(), js, "test-consumer")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "pipeline consumer create")
	assert.Contains(t, err.Error(), "test-consumer")
	assert.Equal(t, "PALADIN_ALERTS", js.gotStream, "must target the canonical alerts stream")
	assert.Equal(t, "test-consumer", js.gotConfig.Name)
	assert.Equal(t, "test-consumer", js.gotConfig.Durable, "consumer must be durable")
	assert.Equal(t, "paladin.alerts.raw.>", js.gotConfig.FilterSubject)
}

func TestRun_MessagesCreateError_IsWrapped(t *testing.T) {
	t.Parallel()
	cons := &fakeConsumer{messagesEr: errors.New("subscribe failed")}
	js := &fakeJetStream{consumer: cons}
	p := New(&localDedup{}, localCorr{}, &localPub{}, zap.NewNop())

	err := p.Run(context.Background(), js, "test-consumer")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "pipeline consumer messages")
	assert.Contains(t, err.Error(), "test-consumer")
}

func TestRun_ProcessesQueuedMessagesAndExitsOnContextCancel(t *testing.T) {
	t.Parallel()
	env := alert.AlertEnvelope{
		TenantID:    "t-run",
		Fingerprint: "fp-run",
		Source:      alert.SourceAlertmanager,
		Status:      alert.StatusFiring,
		StartsAt:    time.Now(),
	}
	data, err := json.Marshal(env)
	require.NoError(t, err)

	msg := &fakeMsg{data: data, subject: "paladin.alerts.raw.t-run.alertmanager"}
	msgs := newFakeMessages([]jetstream.Msg{msg}, nil)
	cons := &fakeConsumer{msgs: msgs}
	js := &fakeJetStream{consumer: cons}

	pub := &localPub{}
	p := New(&localDedup{}, localCorr{}, pub, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx, js, "test-consumer") }()

	// Wait for the message to be processed.
	require.Eventually(t, msg.isAcked, 2*time.Second, 5*time.Millisecond,
		"queued message should be processed and acked")

	cancel()
	select {
	case runErr := <-done:
		assert.ErrorIs(t, runErr, context.Canceled, "Run must return ctx.Err on cancel")
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after context cancel")
	}

	assert.Equal(t, 1, pub.count, "the single queued alert must be published")
}

func TestRun_IteratorError_LogsAndContinuesUntilCancelled(t *testing.T) {
	t.Parallel()
	// fakeMessages will return an error from Next once the queue is empty.
	// The producer goroutine inside Run logs the error and exits; the consumer
	// loop then sees msgCh close and returns "consumer stopped unexpectedly".
	msgs := newFakeMessages(nil, errors.New("iterator broken"))
	cons := &fakeConsumer{msgs: msgs}
	js := &fakeJetStream{consumer: cons}
	p := New(&localDedup{}, localCorr{}, &localPub{}, zap.NewNop())

	err := p.Run(context.Background(), js, "consumer-broken")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "consumer stopped unexpectedly")
}
