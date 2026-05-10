// Package hub manages the WebSocket connection registry and alert fan-out.
// Each tenant has an independent set of subscribers; messages are only
// delivered to clients that match the tenant of the alert.
package hub

import (
	"sync"
)

// Client represents a connected WebSocket subscriber.
// Send is a buffered channel; if full the client is considered slow and dropped.
// done is closed by Unsubscribe to signal the pump goroutine to stop reading.
type Client struct {
	TenantID string
	Severity string // optional filter; empty means all severities
	Service  string // optional filter; empty means all services
	Send     chan []byte
	done     chan struct{}
}

// Done returns a channel that is closed when the client is unsubscribed.
// The WebSocket pump goroutine should select on this to detect eviction.
func (c *Client) Done() <-chan struct{} { return c.done }

const sendBufSize = 64

// Hub holds all connected clients and broadcasts NATS messages to them.
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]struct{}
}

// New creates an empty Hub.
func New() *Hub {
	return &Hub{clients: make(map[*Client]struct{})}
}

// Subscribe registers a client and returns it.
func (h *Hub) Subscribe(tenantID, severity, service string) *Client {
	c := &Client{
		TenantID: tenantID,
		Severity: severity,
		Service:  service,
		Send:     make(chan []byte, sendBufSize),
		done:     make(chan struct{}),
	}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	return c
}

// Unsubscribe removes the client from the hub and signals it via Done().
// The Send channel is NOT closed here — closing is the receiver's responsibility
// to prevent "send on closed channel" panics in concurrent Broadcast calls.
func (h *Hub) Unsubscribe(c *Client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.done)
	}
	h.mu.Unlock()
}

// Broadcast delivers msg to all subscribed clients whose filters match.
// tenantID, severity, and service are extracted from the alert message
// by the caller. Slow clients (full send buffer) are dropped without error.
func (h *Hub) Broadcast(tenantID, severity, service string, msg []byte) {
	h.mu.RLock()
	// collect matching clients under read lock, then send without holding it
	var targets []*Client
	for c := range h.clients {
		if c.TenantID != tenantID {
			continue
		}
		if c.Severity != "" && c.Severity != severity {
			continue
		}
		if c.Service != "" && c.Service != service {
			continue
		}
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		trySend(c, msg)
	}
}

// trySend attempts a non-blocking send to c.Send.
// If the client has been unsubscribed (c.done closed) the message is discarded.
// Slow clients (full buffer) are also dropped rather than blocked.
func trySend(c *Client, msg []byte) {
	select {
	case <-c.done:
		// client unsubscribed between collection and send — discard
	case c.Send <- msg:
	default:
		// slow client — drop this frame rather than block
	}
}

// CloseAll unsubscribes every connected client, triggering a CloseGoingAway
// frame from each pump goroutine. Call this before http.Server.Shutdown to
// drain WebSocket connections within the shutdown timeout.
func (h *Hub) CloseAll() {
	h.mu.Lock()
	for c := range h.clients {
		delete(h.clients, c)
		close(c.done)
	}
	h.mu.Unlock()
}

// Len returns the number of connected clients.
func (h *Hub) Len() int {
	h.mu.RLock()
	n := len(h.clients)
	h.mu.RUnlock()
	return n
}
