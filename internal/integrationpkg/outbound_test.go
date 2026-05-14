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
	subjects  []string
	err       error
}

func (f *fakeDLQ) Publish(_ context.Context, subject string, data []byte) error {
	f.subjects = append(f.subjects, subject)
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
	if len(dlq.subjects) != 1 || dlq.subjects[0] != "dlq.outbound.slack.globex" {
		t.Errorf("DLQ subject = %v, want [dlq.outbound.slack.globex]", dlq.subjects)
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

func TestSend_ServerErrorsRejectsInvalidDLQSubject(t *testing.T) {
	orig := OutboundBackoff
	OutboundBackoff = [OutboundMaxRetries]time.Duration{1 * time.Millisecond, 1 * time.Millisecond, 1 * time.Millisecond}
	defer func() { OutboundBackoff = orig }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	for _, tc := range []struct {
		name        string
		integration string
		tenantID    string
		want        string
	}{
		{name: "empty integration", integration: "", tenantID: "acme", want: "integration"},
		{name: "wildcard integration", integration: "slack.*", tenantID: "acme", want: "integration"},
		{name: "empty tenant", integration: "slack", tenantID: "", want: "tenantID"},
		{name: "dotted tenant", integration: "slack", tenantID: "bad.tenant", want: "tenantID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dlq := &fakeDLQ{}
			sender := NewOutboundSender(srv.Client(), dlq, tc.integration, tc.tenantID)

			err := sender.Send(context.Background(), http.MethodPost, srv.URL, []byte(`payload`), nil)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want it to mention %q", err.Error(), tc.want)
			}
			if len(dlq.published) != 0 {
				t.Fatalf("expected no DLQ publish for invalid subject, got %d", len(dlq.published))
			}
		})
	}
}

func TestOutboundDLQSubjectValidatesTokens(t *testing.T) {
	subject, err := outboundDLQSubject("pagerduty", "tenant-1")
	if err != nil {
		t.Fatalf("expected valid subject, got %v", err)
	}
	if subject != "dlq.outbound.pagerduty.tenant-1" {
		t.Fatalf("subject = %q, want %q", subject, "dlq.outbound.pagerduty.tenant-1")
	}

	for _, tc := range []struct {
		name        string
		integration string
		tenantID    string
	}{
		{name: "dotted integration", integration: "pager.duty", tenantID: "tenant-1"},
		{name: "glob integration", integration: "pagerduty.>", tenantID: "tenant-1"},
		{name: "star tenant", integration: "pagerduty", tenantID: "*"},
		{name: "dotted tenant", integration: "pagerduty", tenantID: "tenant.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := outboundDLQSubject(tc.integration, tc.tenantID); err == nil {
				t.Fatal("expected invalid subject token error")
			}
		})
	}
}
