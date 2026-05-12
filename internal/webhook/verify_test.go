package webhook_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/paladinai/paladinai/internal/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

func signHMAC(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

var testSecret = []byte("super-secret-webhook-key")
var testBody = []byte(`{"alert":"HighCPU","severity":"critical"}`)

// ─── PaladinVerifier ──────────────────────────────────────────────────────────

func TestPaladinVerifier_ValidSignature_NoError(t *testing.T) {
	v := webhook.NewPaladinVerifier(testSecret)
	h := http.Header{"X-Paladin-Signature": {"sha256=" + signHMAC(testSecret, testBody)}}
	require.NoError(t, v.Verify(h, testBody))
}

func TestPaladinVerifier_WrongSignature_Error(t *testing.T) {
	v := webhook.NewPaladinVerifier(testSecret)
	h := http.Header{"X-Paladin-Signature": {"sha256=deadbeef"}}
	assert.ErrorIs(t, v.Verify(h, testBody), webhook.ErrInvalidSignature)
}

func TestPaladinVerifier_MissingHeader_Error(t *testing.T) {
	v := webhook.NewPaladinVerifier(testSecret)
	assert.ErrorIs(t, v.Verify(http.Header{}, testBody), webhook.ErrMissingHeader)
}

func TestPaladinVerifier_WrongSecret_Error(t *testing.T) {
	v := webhook.NewPaladinVerifier([]byte("wrong-secret"))
	h := http.Header{"X-Paladin-Signature": {"sha256=" + signHMAC(testSecret, testBody)}}
	assert.ErrorIs(t, v.Verify(h, testBody), webhook.ErrInvalidSignature)
}

func TestPaladinVerifier_EmptyBody_StillVerifies(t *testing.T) {
	v := webhook.NewPaladinVerifier(testSecret)
	body := []byte{}
	h := http.Header{"X-Paladin-Signature": {"sha256=" + signHMAC(testSecret, body)}}
	require.NoError(t, v.Verify(h, body))
}

// ─── PagerDutyVerifier ────────────────────────────────────────────────────────

// pdHeader creates a properly canonicalized PagerDuty header.
// http.Header.Set canonicalizes the key (X-PagerDuty-Signature → X-Pagerduty-Signature).
func pdHeader(value string) http.Header {
	h := http.Header{}
	h.Set("X-Pagerduty-Signature", value)
	return h
}

func TestPagerDutyVerifier_ValidSignature_NoError(t *testing.T) {
	v := webhook.NewPagerDutyVerifier(testSecret)
	sig := signHMAC(testSecret, testBody)
	h := pdHeader(fmt.Sprintf("v1=%s", sig))
	require.NoError(t, v.Verify(h, testBody))
}

func TestPagerDutyVerifier_MultipleSignatures_FirstMatches(t *testing.T) {
	v := webhook.NewPagerDutyVerifier(testSecret)
	sig := signHMAC(testSecret, testBody)
	// PagerDuty may send multiple sigs
	h := pdHeader(fmt.Sprintf("v1=%s,v1=deadbeef", sig))
	require.NoError(t, v.Verify(h, testBody))
}

func TestPagerDutyVerifier_InvalidSignature_Error(t *testing.T) {
	v := webhook.NewPagerDutyVerifier(testSecret)
	h := pdHeader("v1=deadbeef")
	assert.ErrorIs(t, v.Verify(h, testBody), webhook.ErrInvalidSignature)
}

func TestPagerDutyVerifier_MissingHeader_Error(t *testing.T) {
	v := webhook.NewPagerDutyVerifier(testSecret)
	assert.ErrorIs(t, v.Verify(http.Header{}, testBody), webhook.ErrMissingHeader)
}

func TestPagerDutyVerifier_WrongVersion_Error(t *testing.T) {
	v := webhook.NewPagerDutyVerifier(testSecret)
	sig := signHMAC(testSecret, testBody)
	// v2 prefix — not handled
	h := pdHeader(fmt.Sprintf("v2=%s", sig))
	assert.ErrorIs(t, v.Verify(h, testBody), webhook.ErrInvalidSignature)
}

// ─── GitHubVerifier ───────────────────────────────────────────────────────────

func TestGitHubVerifier_ValidSignature_NoError(t *testing.T) {
	v := webhook.NewGitHubVerifier(testSecret)
	sig := "sha256=" + signHMAC(testSecret, testBody)
	h := http.Header{"X-Hub-Signature-256": {sig}}
	require.NoError(t, v.Verify(h, testBody))
}

func TestGitHubVerifier_InvalidSignature_Error(t *testing.T) {
	v := webhook.NewGitHubVerifier(testSecret)
	h := http.Header{"X-Hub-Signature-256": {"sha256=deadbeef00"}}
	assert.ErrorIs(t, v.Verify(h, testBody), webhook.ErrInvalidSignature)
}

func TestGitHubVerifier_MissingHeader_Error(t *testing.T) {
	v := webhook.NewGitHubVerifier(testSecret)
	assert.ErrorIs(t, v.Verify(http.Header{}, testBody), webhook.ErrMissingHeader)
}

func TestGitHubVerifier_WrongSecret_Error(t *testing.T) {
	v := webhook.NewGitHubVerifier([]byte("wrong"))
	sig := "sha256=" + signHMAC(testSecret, testBody)
	h := http.Header{"X-Hub-Signature-256": {sig}}
	assert.ErrorIs(t, v.Verify(h, testBody), webhook.ErrInvalidSignature)
}

// ─── SlackVerifier ────────────────────────────────────────────────────────────

func slackSig(secret []byte, tsStr string, body []byte) string {
	sigBase := fmt.Sprintf("v0:%s:%s", tsStr, string(body))
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(sigBase))
	return "v0=" + hex.EncodeToString(mac.Sum(nil))
}

func TestSlackVerifier_ValidSignature_NoError(t *testing.T) {
	v := webhook.NewSlackVerifier(testSecret)
	tsStr := strconv.FormatInt(time.Now().Unix(), 10)
	sig := slackSig(testSecret, tsStr, testBody)
	h := http.Header{
		"X-Slack-Request-Timestamp": {tsStr},
		"X-Slack-Signature":         {sig},
	}
	require.NoError(t, v.Verify(h, testBody))
}

func TestSlackVerifier_OldTimestamp_ReturnsTimestampError(t *testing.T) {
	v := webhook.NewSlackVerifier(testSecret)
	old := time.Now().Add(-10 * time.Minute)
	tsStr := strconv.FormatInt(old.Unix(), 10)
	sig := slackSig(testSecret, tsStr, testBody)
	h := http.Header{
		"X-Slack-Request-Timestamp": {tsStr},
		"X-Slack-Signature":         {sig},
	}
	assert.ErrorIs(t, v.Verify(h, testBody), webhook.ErrTimestampTooOld)
}

func TestSlackVerifier_InvalidSignature_Error(t *testing.T) {
	v := webhook.NewSlackVerifier(testSecret)
	tsStr := strconv.FormatInt(time.Now().Unix(), 10)
	h := http.Header{
		"X-Slack-Request-Timestamp": {tsStr},
		"X-Slack-Signature":         {"v0=deadbeef"},
	}
	assert.ErrorIs(t, v.Verify(h, testBody), webhook.ErrInvalidSignature)
}

func TestSlackVerifier_MissingTimestampHeader_Error(t *testing.T) {
	v := webhook.NewSlackVerifier(testSecret)
	h := http.Header{"X-Slack-Signature": {"v0=something"}}
	assert.ErrorIs(t, v.Verify(h, testBody), webhook.ErrMissingHeader)
}

func TestSlackVerifier_MissingSignatureHeader_Error(t *testing.T) {
	v := webhook.NewSlackVerifier(testSecret)
	tsStr := strconv.FormatInt(time.Now().Unix(), 10)
	h := http.Header{"X-Slack-Request-Timestamp": {tsStr}}
	assert.ErrorIs(t, v.Verify(h, testBody), webhook.ErrMissingHeader)
}

func TestSlackVerifier_InvalidTimestamp_Error(t *testing.T) {
	v := webhook.NewSlackVerifier(testSecret)
	h := http.Header{
		"X-Slack-Request-Timestamp": {"not-a-number"},
		"X-Slack-Signature":         {"v0=something"},
	}
	err := v.Verify(h, testBody)
	require.Error(t, err)
	assert.NotErrorIs(t, err, webhook.ErrInvalidSignature) // it's a parse error
}

// ─── Interface compliance ─────────────────────────────────────────────────────

func TestVerifierInterface_AllImplementations(t *testing.T) {
	var _ webhook.Verifier = webhook.NewPaladinVerifier(testSecret)
	var _ webhook.Verifier = webhook.NewPagerDutyVerifier(testSecret)
	var _ webhook.Verifier = webhook.NewGitHubVerifier(testSecret)
	var _ webhook.Verifier = webhook.NewSlackVerifier(testSecret)
}
