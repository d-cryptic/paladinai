// Package handler implements the WebSocket upgrade endpoint for live alert streaming.
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/auth"
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
	// Origin validation is handled by the JWT middleware — by the time we reach
	// the upgrade, the request already carries a verified bearer token, making
	// cross-site WebSocket hijacking impossible without a valid credential.
	CheckOrigin: func(r *http.Request) bool { return true },
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

type agentResultPayload struct {
	Envelope AlertPayload `json:"envelope"`
	Triage   struct {
		ConfirmedSeverity string   `json:"confirmed_severity"`
		AffectedServices  []string `json:"affected_services"`
	} `json:"triage"`
}

// WSHandler handles WebSocket upgrade requests and pumps hub messages to clients.
type WSHandler struct {
	h   *hub.Hub
	log *zap.Logger
}

// New returns a WSHandler wired to h.
func New(h *hub.Hub, log *zap.Logger) *WSHandler {
	return &WSHandler{h: h, log: log}
}

// ServeHTTP upgrades the connection, subscribes the client, and streams alerts
// until the client disconnects or ctx is cancelled.
//
// The JWT middleware must run before this handler. TenantID is derived from the
// verified token claim — never from a client-supplied header.
//
// Query params:
//
//	severity — optional filter (e.g. "p1")
//	service  — optional filter (e.g. "payments-api")
func (wh *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := auth.TenantIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	severity := r.URL.Query().Get("severity")
	service := r.URL.Query().Get("service")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		wh.log.Warn("ws upgrade failed", zap.Error(err), zap.String("tenant", tenantID))
		return
	}

	client := wh.h.Subscribe(tenantID, severity, service)
	wh.log.Info("ws client connected",
		zap.String("tenant", tenantID),
		zap.String("severity", severity),
		zap.String("service", service),
	)

	wh.pump(r.Context(), conn, client)
	wh.h.Unsubscribe(client)

	conn.Close()
	wh.log.Info("ws client disconnected", zap.String("tenant", tenantID))
}

// pump writes hub messages to the WebSocket connection until ctx is cancelled
// or the read loop detects the connection is gone.
//
// Invariant: all WriteMessage calls happen on this goroutine only.
// WriteControl is the sole ws API safe to call from other goroutines.
func (wh *WSHandler) pump(ctx context.Context, conn *websocket.Conn, c *hub.Client) {
	conn.SetReadLimit(maxMsgSize)
	conn.SetReadDeadline(time.Now().Add(pongWait)) //nolint:errcheck
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

	defer func() {
		// Unblock the read goroutine by closing the connection.
		// ServeHTTP's explicit conn.Close() acts as a backup, but we also
		// close here so the read goroutine exits synchronously before pump returns.
		conn.Close()
		<-readDone
	}()

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

		case <-c.Done():
			// Hub evicted this client (e.g. hub shutdown).
			conn.WriteControl( //nolint:errcheck
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseGoingAway, ""),
				time.Now().Add(writeWait),
			)
			return

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait)) //nolint:errcheck
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				return
			}

		case msg := <-c.Send:
			conn.SetWriteDeadline(time.Now().Add(writeWait)) //nolint:errcheck
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				wh.log.Debug("ws write error", zap.Error(err))
				return
			}
		}
	}
}

// NATSHandler returns a callback for the NATS consumer.
// It parses the raw NATS message, extracts routing keys, and calls hub.Broadcast.
// Returns true if the message should be Ack'd, false if it should be Term'd.
func NATSHandler(h *hub.Hub, log *zap.Logger) func([]byte) bool {
	return func(data []byte) bool {
		payload, err := parseAlertPayload(data)
		if err != nil {
			log.Warn("nats msg: unmarshal failed — terminating", zap.Error(err))
			return false // permanently bad; Term at call site
		}
		if payload.TenantID == "" {
			log.Warn("nats msg: missing tenant_id — terminating")
			return false
		}
		h.Broadcast(payload.TenantID, payload.Severity, payload.Service, data)
		return true
	}
}

func parseAlertPayload(data []byte) (AlertPayload, error) {
	var payload AlertPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return AlertPayload{}, err
	}
	if payload.TenantID != "" {
		return payload, nil
	}

	var result agentResultPayload
	if err := json.Unmarshal(data, &result); err != nil {
		return AlertPayload{}, err
	}
	payload = result.Envelope
	if result.Triage.ConfirmedSeverity != "" {
		payload.Severity = result.Triage.ConfirmedSeverity
	}
	if payload.Service == "" && len(result.Triage.AffectedServices) > 0 {
		payload.Service = result.Triage.AffectedServices[0]
	}
	return payload, nil
}
