// Package webhook implements HMAC-SHA256 signature verification for incoming
// webhooks from external sources (Alertmanager, Datadog, PagerDuty, GitHub,
// Slack, custom).
//
// All verifiers implement the Verifier interface. Source-specific headers and
// signing logic are encapsulated per-verifier. Timing-safe comparison is used
// throughout to prevent timing attacks.
//
// See docs/plans/07.integrations-stage7.md §11 for the specification.
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Verifier validates a webhook request's authenticity.
type Verifier interface {
	// Verify checks the request signature. Returns nil on success.
	// Returns ErrInvalidSignature, ErrTimestampTooOld, or ErrReplayDetected on failure.
	Verify(headers http.Header, body []byte) error
}

// Sentinel errors.
var (
	ErrInvalidSignature = errors.New("webhook: invalid signature")
	ErrTimestampTooOld  = errors.New("webhook: timestamp too old (replay protection)")
	ErrMissingHeader    = errors.New("webhook: required header missing")
)

// maxAge is the maximum age of a webhook timestamp before it is rejected.
const maxAge = 5 * time.Minute

// ─── HMAC helpers ─────────────────────────────────────────────────────────────

// computeHMAC computes HMAC-SHA256 of payload using secret.
func computeHMAC(secret, payload []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	return mac.Sum(nil)
}

// timingSafeEqual compares two hex-encoded HMAC strings without leaking timing info.
// Returns false if either string is empty or lengths differ.
func timingSafeEqual(a, b string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	aBytes, err := hex.DecodeString(strings.TrimPrefix(a, "sha256="))
	if err != nil {
		return false
	}
	bBytes, err := hex.DecodeString(strings.TrimPrefix(b, "sha256="))
	if err != nil {
		return false
	}
	return hmac.Equal(aBytes, bBytes)
}

// ─── Paladin / Alertmanager / Datadog verifier ────────────────────────────────

// PaladinVerifier verifies HMAC-SHA256 signatures using the Paladin-issued
// secret. Used for: Alertmanager, Datadog webhooks, custom webhooks.
// Header: X-Paladin-Signature: sha256=<hex>
type PaladinVerifier struct {
	secret []byte
}

// NewPaladinVerifier creates a verifier for the Paladin-issued HMAC secret.
func NewPaladinVerifier(secret []byte) *PaladinVerifier {
	return &PaladinVerifier{secret: secret}
}

// Verify checks the X-Paladin-Signature header.
func (v *PaladinVerifier) Verify(headers http.Header, body []byte) error {
	sig := headers.Get("X-Paladin-Signature")
	if sig == "" {
		return fmt.Errorf("%w: X-Paladin-Signature", ErrMissingHeader)
	}
	expected := "sha256=" + hex.EncodeToString(computeHMAC(v.secret, body))
	if !timingSafeEqual(sig, expected) {
		return ErrInvalidSignature
	}
	return nil
}

// ─── PagerDuty verifier ───────────────────────────────────────────────────────

// PagerDutyVerifier verifies PagerDuty webhook signatures.
// Header: X-PagerDuty-Signature: v1=<hex>
type PagerDutyVerifier struct {
	secret []byte
}

// NewPagerDutyVerifier creates a PagerDuty webhook verifier.
func NewPagerDutyVerifier(secret []byte) *PagerDutyVerifier {
	return &PagerDutyVerifier{secret: secret}
}

// Verify checks the X-Pagerduty-Signature header (v1=<hex> format).
// Note: Go's http.Header canonicalizes "X-PagerDuty-Signature" to
// "X-Pagerduty-Signature" — use h.Set() to set this header in tests.
func (v *PagerDutyVerifier) Verify(headers http.Header, body []byte) error {
	// Try both canonical and non-canonical forms for defensive compatibility.
	raw := headers.Get("X-Pagerduty-Signature")
	if raw == "" {
		raw = headers.Get("X-PagerDuty-Signature")
	}
	if raw == "" {
		return fmt.Errorf("%w: X-Pagerduty-Signature", ErrMissingHeader)
	}
	// PagerDuty sends: "v1=<hex>" or "v1=<hex>,v1=<hex>" (multiple versions).
	// We verify any matching v1 signature.
	expected := hex.EncodeToString(computeHMAC(v.secret, body))
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "v1=") {
			continue
		}
		if timingSafeEqual(strings.TrimPrefix(part, "v1="), expected) {
			return nil
		}
	}
	return ErrInvalidSignature
}

// ─── GitHub verifier ──────────────────────────────────────────────────────────

// GitHubVerifier verifies GitHub webhook signatures using timing-safe comparison.
// Header: X-Hub-Signature-256: sha256=<hex>
type GitHubVerifier struct {
	secret []byte
}

// NewGitHubVerifier creates a GitHub webhook verifier.
func NewGitHubVerifier(secret []byte) *GitHubVerifier {
	return &GitHubVerifier{secret: secret}
}

// Verify checks the X-Hub-Signature-256 header using timing-safe comparison.
func (v *GitHubVerifier) Verify(headers http.Header, body []byte) error {
	sig := headers.Get("X-Hub-Signature-256")
	if sig == "" {
		return fmt.Errorf("%w: X-Hub-Signature-256", ErrMissingHeader)
	}
	expected := "sha256=" + hex.EncodeToString(computeHMAC(v.secret, body))
	if !timingSafeEqual(sig, expected) {
		return ErrInvalidSignature
	}
	return nil
}

// ─── Slack verifier ───────────────────────────────────────────────────────────

// SlackVerifier verifies Slack webhook signatures with timestamp check.
// Headers: X-Slack-Request-Timestamp (Unix epoch), X-Slack-Signature (v0=<hex>)
// Signing message: "v0:{timestamp}:{body}"
type SlackVerifier struct {
	secret []byte
}

// NewSlackVerifier creates a Slack webhook verifier.
func NewSlackVerifier(secret []byte) *SlackVerifier {
	return &SlackVerifier{secret: secret}
}

// Verify checks the Slack signature and timestamp headers.
// Rejects requests older than maxAge to prevent replay attacks.
func (v *SlackVerifier) Verify(headers http.Header, body []byte) error {
	tsStr := headers.Get("X-Slack-Request-Timestamp")
	if tsStr == "" {
		return fmt.Errorf("%w: X-Slack-Request-Timestamp", ErrMissingHeader)
	}
	sig := headers.Get("X-Slack-Signature")
	if sig == "" {
		return fmt.Errorf("%w: X-Slack-Signature", ErrMissingHeader)
	}

	tsUnix, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return fmt.Errorf("webhook: invalid Slack timestamp: %w", err)
	}
	ts := time.Unix(tsUnix, 0)
	delta := time.Since(ts)
	if delta > maxAge || delta < -maxAge {
		return ErrTimestampTooOld
	}

	// Slack signing message: "v0:{timestamp}:{body}"
	sigBase := fmt.Sprintf("v0:%s:%s", tsStr, string(body))
	expectedHex := hex.EncodeToString(computeHMAC(v.secret, []byte(sigBase)))

	// Use hmac.Equal after stripping the "v0=" prefix for timing safety.
	sigHex := strings.TrimPrefix(sig, "v0=")
	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		return ErrInvalidSignature
	}
	expectedBytes, _ := hex.DecodeString(expectedHex)
	if !hmac.Equal(sigBytes, expectedBytes) {
		return ErrInvalidSignature
	}
	return nil
}
