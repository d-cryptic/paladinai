// Package nats wraps the NATS JetStream client with PaladinAI stream definitions.
package nats

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/paladinai/paladinai/internal/telemetry"
	"go.uber.org/zap"
)

// StreamName constants — all streams used across PaladinAI services.
const (
	StreamAlerts       = "PALADIN_ALERTS"
	StreamAgentWork    = "PALADIN_AGENT_WORK"
	StreamRunbookSteps = "PALADIN_RUNBOOK_STEPS"
	StreamIncidents    = "PALADIN_INCIDENTS"
	StreamAudit        = "PALADIN_AUDIT"
	StreamEvents       = "PALADIN_EVENTS" // agent token/event streaming (ephemeral)
	StreamMemory       = "PALADIN_MEMORY" // memory consolidation triggers
	StreamDLQ          = "PALADIN_DLQ"    // dead-letter queue
	StreamEvals        = "PALADIN_EVALS"  // eval replay harness
)

const (
	defaultConnectTimeout      = 5 * time.Second
	defaultEnsureStreamTimeout = 10 * time.Second
)

// Subject patterns.
//
// Note: the Stage 2 spec describes alert subjects in the form
// `alerts.{tenant_id}.{severity}.{source}` but the internal services have
// always used the `paladin.alerts.*` prefix so that every PaladinAI subject
// shares a common namespace. We keep the internal prefix here to avoid a
// breaking change for existing publishers and consumers. The Stage 2 streams
// added below (events/memory/dlq/evals) use their bare top-level prefixes as
// the spec dictates.
const (
	SubjectAlertsRaw        = "paladin.alerts.raw.>"        // raw inbound from integrations
	SubjectAlertsDeduped    = "paladin.alerts.deduped.>"    // after fingerprint dedup
	SubjectAlertsCorrelated = "paladin.alerts.correlated.>" // after correlation window
	SubjectAlertsTriaged    = "paladin.alerts.triaged.>"    // after agent triage fallback
	SubjectAlertsAnalyzed   = "paladin.alerts.analyzed.>"   // after agent RCA analysis
	SubjectAgentWork        = "paladin.agent.work.>"
	SubjectRunbookSteps     = "paladin.runbook.steps.>"
	SubjectIncidents        = "paladin.incidents.>"
	SubjectAudit            = "paladin.audit.>"
	SubjectEvents           = "stream.>"               // agent streaming events to paladin-ws
	SubjectMemory           = "memory.consolidation.>" // weekly memory consolidation
	SubjectDLQ              = "dlq.>"                  // dead-letter messages
	SubjectEvals            = "evals.>"                // eval replay harness
)

// Client wraps nats.Conn + JetStream context.
type Client struct {
	nc  *nats.Conn
	js  jetstream.JetStream
	log *zap.Logger
}

// Connect establishes a NATS connection and ensures all streams exist.
func Connect(url string, log *zap.Logger) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultEnsureStreamTimeout)
	defer cancel()
	return ConnectWithContext(ctx, url, log)
}

// ConnectWithContext establishes a NATS connection and ensures all streams
// exist before returning. The context bounds JetStream setup.
func ConnectWithContext(ctx context.Context, url string, log *zap.Logger) (*Client, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("nats connect setup context: %w", err)
	}
	if log == nil {
		log = zap.NewNop()
	}
	nc, err := nats.Connect(url,
		nats.Name("paladin"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.Timeout(defaultConnectTimeout),
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
	if err := c.ensureStreams(ctx); err != nil {
		nc.Close()
		return nil, err
	}

	log.Info("nats connected", zap.String("url", nc.ConnectedUrl()))
	return c, nil
}

// JS returns the raw JetStream context for direct use.
func (c *Client) JS() jetstream.JetStream { return c.js }

// Conn returns the underlying NATS connection (e.g. for health checks).
func (c *Client) Conn() *nats.Conn { return c.nc }

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
			Description: "All alert events: raw, deduped, correlated, triaged, analyzed",
			Subjects: []string{
				SubjectAlertsRaw,
				SubjectAlertsDeduped,
				SubjectAlertsCorrelated,
				SubjectAlertsTriaged,
				SubjectAlertsAnalyzed,
			},
			Retention:  jetstream.LimitsPolicy,
			MaxAge:     72 * time.Hour, // 3 days
			MaxMsgs:    5_000_000,
			Storage:    jetstream.FileStorage,
			Replicas:   1,               // increase to 3 in production
			Duplicates: 5 * time.Minute, // NATS-level dedup window
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
		{
			Name:        StreamEvents,
			Description: "Agent token/event streaming → paladin-ws → browser (ephemeral)",
			Subjects:    []string{SubjectEvents},
			Retention:   jetstream.LimitsPolicy,
			MaxAge:      1 * time.Hour, // ephemeral; speed over durability
			MaxMsgs:     1_000_000,
			Storage:     jetstream.MemoryStorage,
			Replicas:    1,
		},
		{
			Name:        StreamMemory,
			Description: "Memory consolidation triggers (weekly background jobs)",
			Subjects:    []string{SubjectMemory},
			Retention:   jetstream.LimitsPolicy,
			MaxAge:      7 * 24 * time.Hour,
			MaxMsgs:     100_000,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
		},
		{
			Name:        StreamDLQ,
			Description: "Dead-letter queue: messages failing after 3 NAK/redelivery attempts",
			Subjects:    []string{SubjectDLQ},
			Retention:   jetstream.LimitsPolicy,
			MaxAge:      30 * 24 * time.Hour, // 30 days for forensic replay
			MaxMsgs:     1_000_000,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
		},
		{
			Name:        StreamEvals,
			Description: "Eval replay harness inputs and outputs",
			Subjects:    []string{SubjectEvals},
			Retention:   jetstream.LimitsPolicy,
			MaxAge:      7 * 24 * time.Hour,
			MaxMsgs:     500_000,
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
	msg := &nats.Msg{Subject: subject, Data: data, Header: nats.Header{}}
	telemetry.InjectTraceContext(ctx, msg.Header)
	ack, err := c.js.PublishMsg(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("publish to %s: %w", subject, err)
	}
	return ack, nil
}

// PublishDLQ forwards a failed message to the dead-letter queue.
//
// tenant is the originating tenant (used as a routing token); originalSubject
// is the subject the message was first published on (e.g.
// "paladin.alerts.correlated.tenant.alertmanager"). Consumers call this after
// MaxDeliveries NAK/redelivery attempts so failed messages can be inspected
// or replayed out of band.
//
// The resulting DLQ subject is `dlq.{tenant}.{original_subject_escaped}`
// where dots in the original subject are replaced with underscores so the
// whole original subject collapses into a single NATS token.
func (c *Client) PublishDLQ(ctx context.Context, tenant, originalSubject string, data []byte) (*jetstream.PubAck, error) {
	if err := validateToken(tenant); err != nil {
		return nil, fmt.Errorf("publish dlq tenant: %w", err)
	}
	if originalSubject == "" {
		return nil, fmt.Errorf("publish dlq: originalSubject must not be empty")
	}
	escaped := strings.NewReplacer(".", "_", "*", "_", ">", "_").Replace(originalSubject)
	subject := fmt.Sprintf("dlq.%s.%s", tenant, escaped)
	return c.Publish(ctx, subject, data)
}
