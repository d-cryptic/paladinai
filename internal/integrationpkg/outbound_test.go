package integrationpkg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ── fake DLQ ──────────────────────────────────────────────────────────────────

type fakeDLQ struct {
	published [][]byte
	err       error
}

func (f *fakeDLQ) Publish(_ context.Context, _ string, data []byte) error {
	f.published = append(f.published, data)
	return f.err
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestSend_SuccessFirstAttempt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	dlq := &fakeDLQ{}
	sender := NewOutboundSender(srv.Client(), dlq, "slack", "acme")
	// Override OutboundBackoff temporarily for speed (not needed here — first attempt succeeds).
	err := sender.Send(context.Background(), http.MethodPost, srv.URL, []byte(`{}`), nil)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if len(dlq.published) != 0 {
		t.Error("expected no DLQ publish on success")
	}
}

func TestSend_PermanentClientError_NoRetry(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusBadRequest) // 400 — not retried
	}))
	defer srv.Close()

	dlq := &fakeDLQ{}
	sender := NewOutboundSender(srv.Client(), dlq, "pagerduty", "acme")
	err := sender.Send(context.Background(), http.MethodPost, srv.URL, []byte(`{}`), nil)
	if err != nil {
		t.Errorf("expected nil (400 is not retried, treated as success), got: %v", err)
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("expected exactly 1 attempt for 400, got %d", callCount)
	}
}

func TestSend_ServerErrors_RetriesThenDLQ(t *testing.T) {
	// Shorten backoffs for test speed.
	orig := OutboundBackoff
	OutboundBackoff = [OutboundMaxRetries]time.Duration{1 * time.Millisecond, 1 * time.Millisecond, 1 * time.Millisecond}
	defer func() { OutboundBackoff = orig }()

	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusServiceUnavailable) // 503 — retried
	}))
	defer srv.Close()

	dlq := &fakeDLQ{}
	sender := NewOutboundSender(srv.Client(), dlq, "slack", "globex")
	err := sender.Send(context.Background(), http.MethodPost, srv.URL, []byte(`payload`), nil)

	if err == nil {
		t.Error("expected error after all retries exhausted")
	}
	if !strings.Contains(err.Error(), "dlq") {
		t.Errorf("expected error to mention DLQ, got: %v", err)
	}
	if len(dlq.published) != 1 {
		t.Errorf("expected 1 DLQ publish, got %d", len(dlq.published))
	}
	if string(dlq.published[0]) != "payload" {
		t.Errorf("DLQ payload = %q, want %q", dlq.published[0], "payload")
	}
	// OutboundMaxRetries=3 retries + 1 first attempt = 4 total calls.
	if atomic.LoadInt32(&callCount) != OutboundMaxRetries+1 {
		t.Errorf("expected %d total calls, got %d", OutboundMaxRetries+1, callCount)
	}
}

func TestSend_ContextCancelled_Stops(t *testing.T) {
	orig := OutboundBackoff
	OutboundBackoff = [OutboundMaxRetries]time.Duration{5 * time.Second, 5 * time.Second, 5 * time.Second}
	defer func() { OutboundBackoff = orig }()

	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	dlq := &fakeDLQ{}
	sender := NewOutboundSender(srv.Client(), dlq, "slack", "acme")

	done := make(chan error, 1)
	go func() {
		done <- sender.Send(ctx, http.MethodPost, srv.URL, []byte(`{}`), nil)
	}()

	// Let first attempt complete, then cancel during first retry backoff.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected error on context cancel")
		}
	case <-time.After(3 * time.Second):
		t.Error("Send did not return after context cancel within 3s")
	}
}

func TestSend_CustomHeaders(t *testing.T) {
	var receivedHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("X-Paladin-Token")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	dlq := &fakeDLQ{}
	sender := NewOutboundSender(srv.Client(), dlq, "pagerduty", "acme")
	err := sender.Send(context.Background(), http.MethodPost, srv.URL, []byte(`{}`),
		map[string]string{"X-Paladin-Token": "tok-abc"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if receivedHeader != "tok-abc" {
		t.Errorf("received header = %q, want %q", receivedHeader, "tok-abc")
	}
}
