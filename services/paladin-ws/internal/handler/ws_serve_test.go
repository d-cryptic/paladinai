package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/paladinai/paladinai/internal/auth"
	"github.com/paladinai/paladinai/services/paladin-ws/internal/handler"
	"github.com/paladinai/paladinai/services/paladin-ws/internal/hub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ─── Helpers ─────────────────────────────────────────────────────────────────

// newTestServer wraps the handler in a middleware that injects tenantID into
// the request context, mimicking what the JWT middleware would normally do.
func newTestServer(wh *handler.WSHandler, tenantID string) *httptest.Server {
	mw := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if tenantID != "" {
			r = r.WithContext(auth.WithTenantID(r.Context(), tenantID))
		}
		wh.ServeHTTP(w, r)
	})
	return httptest.NewServer(mw)
}

// dialWS connects a WebSocket client to the test server.
func dialWS(t *testing.T, srv *httptest.Server, path string) *websocket.Conn {
	t.Helper()
	u := "ws" + strings.TrimPrefix(srv.URL, "http") + path
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)
	return conn
}

// ─── Constructor ─────────────────────────────────────────────────────────────

func TestNew_ReturnsNonNil(t *testing.T) {
	h := hub.New()
	wh := handler.New(h, zap.NewNop())
	assert.NotNil(t, wh)
}

// ─── ServeHTTP — no tenant ID in context ─────────────────────────────────────

func TestServeHTTP_MissingTenantID_Returns401(t *testing.T) {
	h := hub.New()
	wh := handler.New(h, zap.NewNop())
	// No middleware — tenant ID is absent from context.
	srv := httptest.NewServer(wh)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/ws")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// ─── ServeHTTP — WebSocket upgrade ───────────────────────────────────────────

func TestServeHTTP_ValidRequest_ConnectsAndCloses(t *testing.T) {
	h := hub.New()
	wh := handler.New(h, zap.NewNop())
	srv := newTestServer(wh, "tenant-ws")
	defer srv.Close()

	conn := dialWS(t, srv, "/ws")
	// Normal close — client initiates.
	err := conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	assert.NoError(t, err)
	conn.Close()
}

func TestServeHTTP_MessageDelivered_ToSubscribedClient(t *testing.T) {
	h := hub.New()
	wh := handler.New(h, zap.NewNop())
	srv := newTestServer(wh, "tenant-msg")
	defer srv.Close()

	conn := dialWS(t, srv, "/ws")
	defer conn.Close()

	// Give the handler goroutine time to call h.Subscribe before broadcasting.
	time.Sleep(20 * time.Millisecond)

	// Broadcast a payload via the hub directly.
	payload, _ := json.Marshal(map[string]any{
		"tenant_id":   "tenant-msg",
		"fingerprint": "fp-1",
		"severity":    "p2",
		"status":      "firing",
	})
	h.Broadcast("tenant-msg", "p2", "", payload)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err, "should receive the broadcast message")
	assert.Equal(t, payload, msg)
}

func TestServeHTTP_SeverityFilter_OnlyMatchingDelivered(t *testing.T) {
	h := hub.New()
	wh := handler.New(h, zap.NewNop())
	srv := newTestServer(wh, "tenant-filter")
	defer srv.Close()

	// Connect with severity=p1 filter via query param.
	conn := dialWS(t, srv, "/ws?severity=p1")
	defer conn.Close()

	time.Sleep(20 * time.Millisecond)

	payload, _ := json.Marshal(map[string]any{"tenant_id": "tenant-filter", "fingerprint": "fp-p3"})
	h.Broadcast("tenant-filter", "p3", "", payload) // p3 should not reach p1 client

	conn.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	_, _, err := conn.ReadMessage()
	assert.Error(t, err, "p3 message should not be delivered to p1 client (deadline exceeded)")
}

func TestServeHTTP_ServiceFilter_OnlyMatchingDelivered(t *testing.T) {
	h := hub.New()
	wh := handler.New(h, zap.NewNop())
	srv := newTestServer(wh, "tenant-svc")
	defer srv.Close()

	conn := dialWS(t, srv, "/ws?service=payments")
	defer conn.Close()

	time.Sleep(20 * time.Millisecond)

	payload, _ := json.Marshal(map[string]any{"tenant_id": "tenant-svc", "fingerprint": "fp-auth"})
	h.Broadcast("tenant-svc", "p1", "auth", payload) // wrong service

	conn.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	_, _, err := conn.ReadMessage()
	assert.Error(t, err, "auth alert should not be delivered to payments-only client")
}

func TestServeHTTP_ContextCancellation_DisconnectsClient(t *testing.T) {
	h := hub.New()
	wh := handler.New(h, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(auth.WithTenantID(ctx, "tenant-ctx"))
		wh.ServeHTTP(w, r)
	}))
	defer srv.Close()

	conn := dialWS(t, srv, "/ws")
	defer conn.Close()

	time.Sleep(20 * time.Millisecond)

	// Cancel the server-side context — pump should send CloseNormalClosure.
	cancel()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := conn.ReadMessage()
	// Expect a close frame or EOF.
	assert.Error(t, err, "context cancellation should close the WebSocket")
}
