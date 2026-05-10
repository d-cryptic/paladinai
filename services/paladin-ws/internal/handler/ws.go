// Package handler implements the WebSocket upgrade endpoint for live alert streaming.
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/services/paladin-ws/internal/hub"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	maxMsgSize = 256 * 1024 // 256 KiB
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true }, // auth handled by middleware
}

// AlertPayload is the structure broadcast over the wire.
type AlertPayload struct {
	TenantID      string    `json:"tenant_id"`
	Fingerprint   string    `json:"fingerprint"`
	Severity      string    `json:"severity"`
	Status        string    `json:"status"`
	Service       string    `json:"service"`
	Title         string    `json:"title"`
	CorrelationID string    `json:"correlation_id,omitempty"`
	StartsAt      time.Time `json:"starts_at"`
}

// WSHandler handles WebSocket upgrade requests and pumps hub messages to clients.
type WSHandler struct {
	hub *hub.Hub
	log *zap.Logger
}

// New returns a WSHandler wired to h.
func New(h *hub.Hub, log *zap.Logger) *WSHandler {
	return &WSHandler{hub: h, log: log}
}

// ServeHTTP upgrades the connection, subscribes the client, and streams alerts
// until the client disconnects or ctx is cancelled.
//
// Query params:
//
//	severity — optional filter (e.g. "p1")
//	service  — optional filter (e.g. "payments-api")
func (wh *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		http.Error(w, "missing X-Tenant-ID", http.StatusUnauthorized)
		return
	}

	severity := r.URL.Query().Get("severity")
	service := r.URL.Query().Get("service")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		wh.log.Warn("ws upgrade failed", zap.Error(err), zap.String("tenant", tenantID))
		return
	}
	defer conn.Close()

	client := wh.hub.Subscribe(tenantID, severity, service)
	defer wh.hub.Unsubscribe(client)

	wh.log.Info("ws client connected",
		zap.String("tenant", tenantID),
		zap.String("severity", severity),
		zap.String("service", service),
	)

	wh.pump(r.Context(), conn, client)

	wh.log.Info("ws client disconnected", zap.String("tenant", tenantID))
}

// pump writes hub messages to the WebSocket connection until ctx is cancelled
// or the read loop detects the connection is gone.
func (wh *WSHandler) pump(ctx context.Context, conn *websocket.Conn, c *hub.Client) {
	conn.SetReadLimit(maxMsgSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))    //nolint:errcheck
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	// Read loop: detect client-side disconnects (pong timeout or explicit close).
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			conn.WriteControl( //nolint:errcheck
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				time.Now().Add(writeWait),
			)
			return

		case <-readDone:
			return

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait)) //nolint:errcheck
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				return
			}

		case <-c.Done():
			// Hub evicted this client (e.g. server shutdown).
			conn.WriteControl( //nolint:errcheck
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseGoingAway, ""),
				time.Now().Add(writeWait),
			)
			return

		case msg := <-c.Send:
			conn.SetWriteDeadline(time.Now().Add(writeWait)) //nolint:errcheck
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				wh.log.Debug("ws write error", zap.Error(err))
				return
			}
		}
	}
}

// NATSHandler is the callback passed to the NATS consumer.
// It parses the raw NATS message, extracts routing keys, and calls hub.Broadcast.
func NATSHandler(h *hub.Hub, log *zap.Logger) func([]byte) {
	return func(data []byte) {
		var payload AlertPayload
		if err := json.Unmarshal(data, &payload); err != nil {
			log.Warn("nats msg: unmarshal failed", zap.Error(err))
			return
		}
		if payload.TenantID == "" {
			log.Warn("nats msg: missing tenant_id, skipping broadcast")
			return
		}
		h.Broadcast(payload.TenantID, payload.Severity, payload.Service, data)
	}
}
