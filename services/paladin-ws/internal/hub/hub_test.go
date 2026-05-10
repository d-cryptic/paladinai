package hub

import (
	"sync"
	"testing"
	"time"
)

func TestSubscribeAddsClient(t *testing.T) {
	h := New()
	c := h.Subscribe("tenant1", "", "")
	if h.Len() != 1 {
		t.Fatalf("expected 1 client, got %d", h.Len())
	}
	if c.TenantID != "tenant1" {
		t.Errorf("unexpected tenant %q", c.TenantID)
	}
}

func TestUnsubscribeRemovesClient(t *testing.T) {
	h := New()
	c := h.Subscribe("tenant1", "", "")
	h.Unsubscribe(c)
	if h.Len() != 0 {
		t.Fatalf("expected 0 clients after unsubscribe, got %d", h.Len())
	}
}

func TestUnsubscribeSignalsDone(t *testing.T) {
	h := New()
	c := h.Subscribe("tenant1", "", "")
	h.Unsubscribe(c)
	// Done channel must be closed — receive should not block.
	select {
	case <-c.Done():
		// correct
	default:
		t.Error("Done() channel should be closed after Unsubscribe")
	}
}

func TestUnsubscribeIdempotent(t *testing.T) {
	h := New()
	c := h.Subscribe("t1", "", "")
	h.Unsubscribe(c)
	// Second unsubscribe must not panic (double-close).
	h.Unsubscribe(c)
}

func TestBroadcastTenantIsolation(t *testing.T) {
	h := New()
	c1 := h.Subscribe("tenant-a", "", "")
	c2 := h.Subscribe("tenant-b", "", "")

	h.Broadcast("tenant-a", "p1", "api", []byte(`{"alert":1}`))

	select {
	case msg := <-c1.Send:
		if string(msg) != `{"alert":1}` {
			t.Errorf("c1 got wrong msg: %s", msg)
		}
	default:
		t.Error("tenant-a client should have received the broadcast")
	}

	select {
	case <-c2.Send:
		t.Error("tenant-b client should NOT receive tenant-a broadcast")
	default:
		// correct: no message
	}
}

func TestBroadcastSeverityFilter(t *testing.T) {
	h := New()
	c := h.Subscribe("t1", "p1", "") // only p1 alerts

	h.Broadcast("t1", "p2", "api", []byte("noise"))
	select {
	case <-c.Send:
		t.Error("severity filter: p2 alert should not reach p1-only client")
	default:
	}

	h.Broadcast("t1", "p1", "api", []byte("match"))
	select {
	case msg := <-c.Send:
		if string(msg) != "match" {
			t.Errorf("unexpected msg: %s", msg)
		}
	default:
		t.Error("p1 alert should reach p1-only client")
	}
}

func TestBroadcastServiceFilter(t *testing.T) {
	h := New()
	c := h.Subscribe("t1", "", "payments") // only payments service

	h.Broadcast("t1", "p1", "auth", []byte("noise"))
	select {
	case <-c.Send:
		t.Error("service filter: auth alert should not reach payments-only client")
	default:
	}

	h.Broadcast("t1", "p1", "payments", []byte("match"))
	select {
	case msg := <-c.Send:
		if string(msg) != "match" {
			t.Errorf("unexpected msg: %s", msg)
		}
	default:
		t.Error("payments alert should reach payments-only client")
	}
}

func TestBroadcastNoFiltersReceivesAll(t *testing.T) {
	h := New()
	c := h.Subscribe("t1", "", "") // no filters

	h.Broadcast("t1", "p3", "some-service", []byte("any"))
	select {
	case <-c.Send:
	default:
		t.Error("unfiltered client should receive all broadcasts for its tenant")
	}
}

func TestBroadcastSlowClientDropped(t *testing.T) {
	h := New()
	c := h.Subscribe("t1", "", "")

	// Fill the buffer completely.
	msg := []byte("x")
	for i := 0; i < sendBufSize; i++ {
		c.Send <- msg
	}

	// Buffer full — next Broadcast must not block.
	done := make(chan struct{})
	go func() {
		h.Broadcast("t1", "p1", "svc", msg)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Broadcast blocked on a slow client — should have dropped the frame")
	}
}

func TestConcurrentSubscribeUnsubscribe(t *testing.T) {
	h := New()
	const n = 50
	var wg sync.WaitGroup
	wg.Add(n * 2)

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			c := h.Subscribe("t1", "", "")
			h.Unsubscribe(c)
		}()
		go func() {
			defer wg.Done()
			h.Broadcast("t1", "p1", "svc", []byte("msg"))
		}()
	}
	wg.Wait()
}

func TestLen(t *testing.T) {
	h := New()
	if h.Len() != 0 {
		t.Fatalf("fresh hub should have 0 clients")
	}
	c1 := h.Subscribe("t1", "", "")
	c2 := h.Subscribe("t1", "", "")
	if h.Len() != 2 {
		t.Fatalf("expected 2, got %d", h.Len())
	}
	h.Unsubscribe(c1)
	if h.Len() != 1 {
		t.Fatalf("expected 1, got %d", h.Len())
	}
	h.Unsubscribe(c2)
	if h.Len() != 0 {
		t.Fatalf("expected 0, got %d", h.Len())
	}
}
