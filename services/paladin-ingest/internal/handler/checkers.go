package handler

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
)

// NATSChecker pings the NATS server via a round-trip RTT measurement.
// This verifies actual reachability, not just cached connection state.
type NATSChecker struct {
	conn *nats.Conn
}

// NewNATSChecker wraps an existing NATS connection.
func NewNATSChecker(conn *nats.Conn) *NATSChecker {
	return &NATSChecker{conn: conn}
}

func (c *NATSChecker) Name() string { return "nats" }

func (c *NATSChecker) Check(ctx context.Context) error {
	// Fast path: if disconnected, no need for a round-trip.
	if !c.conn.IsConnected() {
		return fmt.Errorf("nats: not connected (status: %s)", c.conn.Status())
	}

	if _, err := c.conn.RTT(); err != nil {
		return fmt.Errorf("nats: RTT failed: %w", err)
	}
	return nil
}

// ValkeyChecker pings the Valkey/Redis server.
type ValkeyChecker struct {
	client *redis.Client
}

// NewValkeyChecker wraps an existing redis.Client.
func NewValkeyChecker(client *redis.Client) *ValkeyChecker {
	return &ValkeyChecker{client: client}
}

func (c *ValkeyChecker) Name() string { return "valkey" }

func (c *ValkeyChecker) Check(ctx context.Context) error {
	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("valkey: ping failed: %w", err)
	}
	return nil
}
