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

// PublishResult carries the confirmation returned by the broker after a
// successful publish. Using a local value type keeps this package decoupled
// from the nats.go/jetstream concrete types.
type PublishResult struct {
	Sequence uint64
}

// JetStream is the minimum surface required by NATSPublisher.
// *inats.Client does not implement this directly (its Publish returns
// *jetstream.PubAck); use New to obtain a correctly adapted publisher.
type JetStream interface {
	Publish(ctx context.Context, subject string, data []byte) (PublishResult, error)
}

// natsAdapter adapts *inats.Client to the JetStream interface.
type natsAdapter struct {
	client *inats.Client
}

func (a *natsAdapter) Publish(ctx context.Context, subject string, data []byte) (PublishResult, error) {
	ack, err := a.client.Publish(ctx, subject, data)
	if err != nil {
		return PublishResult{}, err
	}
	return PublishResult{Sequence: ack.Sequence}, nil
}

// NATSPublisher publishes AlertEnvelopes to NATS JetStream.
type NATSPublisher struct {
	client JetStream
	log    *zap.Logger
}

// New creates a NATSPublisher backed by a real *inats.Client.
// The adapter is wired here so callers in cmd/ do not need to know about it.
func New(client *inats.Client, log *zap.Logger) *NATSPublisher {
	return &NATSPublisher{client: &natsAdapter{client}, log: log}
}

// NewFromJS creates a NATSPublisher with an explicit JetStream implementation.
// Use this in tests or when supplying a custom transport.
func NewFromJS(js JetStream, log *zap.Logger) *NATSPublisher {
	return &NATSPublisher{client: js, log: log}
}

// PublishAlert serialises and publishes an AlertEnvelope to the raw alerts subject.
func (p *NATSPublisher) PublishAlert(ctx context.Context, env alert.AlertEnvelope) error {
	subject, err := env.NATSSubjectE()
	if err != nil {
		return fmt.Errorf("invalid alert subject: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("publish cancelled: %w", err)
	}

	data, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal alert: %w", err)
	}

	result, err := p.client.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("nats publish: %w", err)
	}

	p.log.Debug("alert published",
		zap.String("subject", subject),
		zap.String("fingerprint", env.Fingerprint),
		zap.Uint64("seq", result.Sequence),
	)
	return nil
}
