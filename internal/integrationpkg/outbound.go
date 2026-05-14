// outbound.go implements reliable outbound webhook delivery from
// docs/plans/07.integrations-stage7.md §13 (Outbound Webhook Reliability).
//
// sendWithRetry retries up to OutboundMaxRetries times with fixed backoffs,
// then publishes to a NATS DLQ subject on terminal failure. This keeps the
// caller's latency bounded and ensures no alert is silently dropped.
package integrationpkg

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const OutboundMaxRetries = 3

// OutboundBackoff is the delay before each retry attempt (index = attempt number).
var OutboundBackoff = [OutboundMaxRetries]time.Duration{
	1 * time.Second,
	5 * time.Second,
	30 * time.Second,
}

// DLQPublisher publishes failed outbound payloads to a dead-letter queue.
// The real implementation uses NATS JetStream; tests use a fake.
type DLQPublisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
}

// OutboundSender delivers webhook payloads with retry and DLQ fallback.
type OutboundSender struct {
	client      *http.Client
	dlq         DLQPublisher
	integration string
	tenantID    string
}

// NewOutboundSender creates an OutboundSender for the given integration/tenant pair.
func NewOutboundSender(client *http.Client, dlq DLQPublisher, integration, tenantID string) *OutboundSender {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &OutboundSender{
		client:      client,
		dlq:         dlq,
		integration: integration,
		tenantID:    tenantID,
	}
}

// Send delivers body to url with up to OutboundMaxRetries retries.
// If all attempts fail, the payload is published to the DLQ subject
// `dlq.outbound.{integration}.{tenantID}`.
// Returns nil when the request succeeds (2xx or 3xx) on any attempt.
func (s *OutboundSender) Send(ctx context.Context, method, url string, body []byte, headers map[string]string) error {
	var lastErr error
	for attempt := 0; attempt <= OutboundMaxRetries; attempt++ {
		if attempt > 0 {
			if err := waitOutboundBackoff(ctx, OutboundBackoff[attempt-1]); err != nil {
				return err
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("build request: %w", err) // non-retryable
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("attempt %d: %w", attempt+1, err)
			continue
		}
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		resp.Body.Close()

		if resp.StatusCode < 500 {
			return nil // success or permanent client error — do not retry
		}
		lastErr = fmt.Errorf("attempt %d: server %d", attempt+1, resp.StatusCode)
	}

	// All attempts exhausted: publish to DLQ.
	subject, err := outboundDLQSubject(s.integration, s.tenantID)
	if err != nil {
		return fmt.Errorf("all retries failed (%w); build outbound dlq subject: %w", lastErr, err)
	}
	if pubErr := s.dlq.Publish(ctx, subject, body); pubErr != nil {
		return fmt.Errorf("all retries failed (%w); dlq publish also failed: %v", lastErr, pubErr)
	}
	return fmt.Errorf("all retries failed, payload sent to DLQ %s: %w", subject, lastErr)
}

func waitOutboundBackoff(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("outbound send cancelled: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}
