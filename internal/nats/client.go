// Package nats wraps the NATS JetStream client with PaladinAI stream definitions.
package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// StreamName constants — all streams used across PaladinAI services.
const (
	StreamAlerts      = "PALADIN_ALERTS"
	StreamAgentWork   = "PALADIN_AGENT_WORK"
	StreamRunbookSteps = "PALADIN_RUNBOOK_STEPS"
	StreamIncidents   = "PALADIN_INCIDENTS"
	StreamAudit       = "PALADIN_AUDIT"
)

// Subject patterns
const (
	SubjectAlertsRaw        = "paladin.alerts.raw.>"   // raw inbound from integrations
	SubjectAlertsDeduped    = "paladin.alerts.deduped.>" // after fingerprint dedup
	SubjectAlertsCorrelated = "paladin.alerts.correlated.>" // after correlation window
	SubjectAgentWork        = "paladin.agent.work.>"
	SubjectRunbookSteps     = "paladin.runbook.steps.>"
	SubjectIncidents        = "paladin.incidents.>"
	SubjectAudit            = "paladin.audit.>"
)

// Client wraps nats.Conn + JetStream context.
type Client struct {
	nc  *nats.Conn
	js  jetstream.JetStream
	log *zap.Logger
}

// Connect establishes a NATS connection and ensures all streams exist.
func Connect(url string, log *zap.Logger) (*Client, error) {
	nc, err := nats.Connect(url,
		nats.Name("paladin"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			log.Warn("nats disconnected", zap.Error(err))
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Info("nats reconnected", zap.String("url", nc.ConnectedUrl()))
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect %s: %w", url, err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream init: %w", err)
	}

	c := &Client{nc: nc, js: js, log: log}
	if err := c.ensureStreams(context.Background()); err != nil {
		nc.Close()
		return nil, err
	}

	log.Info("nats connected", zap.String("url", nc.ConnectedUrl()))
	return c, nil
}

// JS returns the raw JetStream context for direct use.
func (c *Client) JS() jetstream.JetStream { return c.js }

// Close drains the connection gracefully.
func (c *Client) Close() {
	if err := c.nc.Drain(); err != nil {
		c.log.Warn("nats drain error", zap.Error(err))
	}
}

// ensureStreams creates streams if they don't exist (idempotent).
func (c *Client) ensureStreams(ctx context.Context) error {
	streams := []jetstream.StreamConfig{
		{
			Name:        StreamAlerts,
			Description: "All alert events: raw, deduped, correlated",
			Subjects:    []string{SubjectAlertsRaw, SubjectAlertsDeduped, SubjectAlertsCorrelated},
			Retention:   jetstream.LimitsPolicy,
			MaxAge:      72 * time.Hour, // 3 days
			MaxMsgs:     5_000_000,
			Storage:     jetstream.FileStorage,
			Replicas:    1, // increase to 3 in production
			Duplicates:  5 * time.Minute, // NATS-level dedup window
		},
		{
			Name:        StreamAgentWork,
			Description: "Work items dispatched to paladin-agent workers",
			Subjects:    []string{SubjectAgentWork},
			Retention:   jetstream.WorkQueuePolicy,
			MaxAge:      24 * time.Hour,
			MaxMsgs:     1_000_000,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
		},
		{
			Name:        StreamRunbookSteps,
			Description: "Individual runbook execution steps (P1/P2 workflows)",
			Subjects:    []string{SubjectRunbookSteps},
			Retention:   jetstream.LimitsPolicy,
			MaxAge:      7 * 24 * time.Hour,
			MaxMsgs:     500_000,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
		},
		{
			Name:        StreamIncidents,
			Description: "Incident lifecycle events (created, updated, resolved)",
			Subjects:    []string{SubjectIncidents},
			Retention:   jetstream.LimitsPolicy,
			MaxAge:      30 * 24 * time.Hour, // 30 days for postmortem
			MaxMsgs:     1_000_000,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
		},
		{
			Name:        StreamAudit,
			Description: "Immutable audit trail: all agent actions, approvals, tool calls",
			Subjects:    []string{SubjectAudit},
			Retention:   jetstream.LimitsPolicy,
			MaxAge:      90 * 24 * time.Hour, // 90 days audit retention
			MaxMsgs:     10_000_000,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
		},
	}

	for _, cfg := range streams {
		_, err := c.js.CreateOrUpdateStream(ctx, cfg)
		if err != nil {
			return fmt.Errorf("ensure stream %s: %w", cfg.Name, err)
		}
		c.log.Debug("stream ready", zap.String("stream", cfg.Name))
	}
	return nil
}

// Publish publishes a raw message to the given subject.
func (c *Client) Publish(ctx context.Context, subject string, data []byte) (*jetstream.PubAck, error) {
	ack, err := c.js.Publish(ctx, subject, data)
	if err != nil {
		return nil, fmt.Errorf("publish to %s: %w", subject, err)
	}
	return ack, nil
}
