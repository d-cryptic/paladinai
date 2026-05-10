// Package publisher wraps NATS JetStream publishing for alert envelopes.
package publisher

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/paladinai/paladinai/internal/alert"
	inats "github.com/paladinai/paladinai/internal/nats"
	"go.uber.org/zap"
)

// NATSPublisher publishes AlertEnvelopes to NATS JetStream.
type NATSPublisher struct {
	client *inats.Client
	log    *zap.Logger
}

// New creates a NATSPublisher.
func New(client *inats.Client, log *zap.Logger) *NATSPublisher {
	return &NATSPublisher{client: client, log: log}
}

// PublishAlert serialises and publishes an AlertEnvelope to the raw alerts subject.
func (p *NATSPublisher) PublishAlert(ctx context.Context, env alert.AlertEnvelope) error {
	data, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal alert: %w", err)
	}

	ack, err := p.client.Publish(ctx, env.NATSSubject(), data)
	if err != nil {
		return fmt.Errorf("nats publish: %w", err)
	}

	p.log.Debug("alert published",
		zap.String("subject", env.NATSSubject()),
		zap.String("fingerprint", env.Fingerprint),
		zap.Uint64("seq", ack.Sequence),
	)
	return nil
}
