package handler_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/services/paladin-ws/internal/handler"
	"github.com/paladinai/paladinai/services/paladin-ws/internal/hub"
)

// alertMsg marshals an AlertPayload-compatible map and returns the raw bytes.
func alertMsg(t *testing.T, tenantID, severity, service string) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"tenant_id":   tenantID,
		"fingerprint": "fp-1",
		"severity":    severity,
		"status":      "firing",
		"service":     service,
		"title":       "High CPU",
		"starts_at":   time.Now().UTC(),
	})
	require.NoError(t, err)
	return b
}

func TestNATSHandler_ValidMessage_BroadcastsAndAcks(t *testing.T) {
	h := hub.New()
	c := h.Subscribe("tenant-1", "", "")

	handle := handler.NATSHandler(h, zap.NewNop())
	msg := alertMsg(t, "tenant-1", "p1", "api")

	ack := handle(msg)

	assert.True(t, ack, "valid message should be Ack'd")

	select {
	case got := <-c.Send:
		assert.Equal(t, msg, got, "broadcast payload should match input bytes verbatim")
	default:
		t.Fatal("expected message on Send channel but none received")
	}
}

func TestNATSHandler_InvalidJSON_TermsMessage(t *testing.T) {
	h := hub.New()
	handle := handler.NATSHandler(h, zap.NewNop())

	ack := handle([]byte("not valid json {{"))

	assert.False(t, ack, "unparseable message should be Term'd (return false)")
}

func TestNATSHandler_MissingTenantID_TermsMessage(t *testing.T) {
	h := hub.New()
	handle := handler.NATSHandler(h, zap.NewNop())

	b, err := json.Marshal(map[string]any{
		"severity": "p1",
		"service":  "api",
		// tenant_id intentionally omitted
	})
	require.NoError(t, err)

	ack := handle(b)

	assert.False(t, ack, "message without tenant_id should be Term'd")
}

func TestNATSHandler_EmptyTenantID_TermsMessage(t *testing.T) {
	h := hub.New()
	handle := handler.NATSHandler(h, zap.NewNop())

	b, err := json.Marshal(map[string]any{
		"tenant_id": "",
		"severity":  "p1",
	})
	require.NoError(t, err)

	ack := handle(b)

	assert.False(t, ack, "empty tenant_id should be Term'd")
}

func TestNATSHandler_TenantIsolation(t *testing.T) {
	h := hub.New()
	cA := h.Subscribe("tenant-a", "", "")
	cB := h.Subscribe("tenant-b", "", "")

	handle := handler.NATSHandler(h, zap.NewNop())
	handle(alertMsg(t, "tenant-a", "p1", "api"))

	select {
	case <-cA.Send:
		// expected
	default:
		t.Fatal("tenant-a should receive its own alert")
	}

	select {
	case <-cB.Send:
		t.Fatal("tenant-b should NOT receive tenant-a alert")
	default:
		// expected
	}
}

func TestNATSHandler_SeverityFilteredClient_NotDelivered(t *testing.T) {
	h := hub.New()
	// Client subscribes only to p1 alerts.
	c := h.Subscribe("t1", "p1", "")

	handle := handler.NATSHandler(h, zap.NewNop())
	handle(alertMsg(t, "t1", "p3", "api")) // p3 should not match

	select {
	case <-c.Send:
		t.Fatal("p1-only client should not receive a p3 alert")
	default:
	}
}

func TestNATSHandler_ServiceFilteredClient_NotDelivered(t *testing.T) {
	h := hub.New()
	// Client subscribes only to "payments" service.
	c := h.Subscribe("t1", "", "payments")

	handle := handler.NATSHandler(h, zap.NewNop())
	handle(alertMsg(t, "t1", "p1", "auth")) // auth service should not match

	select {
	case <-c.Send:
		t.Fatal("payments-only client should not receive an auth alert")
	default:
	}
}

func TestNATSHandler_NoClients_AcksWithoutPanic(t *testing.T) {
	h := hub.New()
	handle := handler.NATSHandler(h, zap.NewNop())

	ack := handle(alertMsg(t, "t1", "p1", "api"))
	assert.True(t, ack, "message is valid even when no clients are connected")
}
