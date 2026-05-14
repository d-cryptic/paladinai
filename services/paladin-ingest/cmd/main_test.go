package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookVerifiers_ConfiguresSupportedIntegrations(t *testing.T) {
	verifiers := webhookVerifiers([]byte("shared-secret"))

	for _, integration := range []string{"alertmanager", "datadog", "cloudwatch", "pagerduty", "slack", "github"} {
		assert.NotNil(t, verifiers[integration], "missing verifier for %s", integration)
	}
}

func TestWebhookVerifiers_PaladinSignedSource(t *testing.T) {
	secret := []byte("shared-secret")
	body := []byte(`{"status":"firing"}`)
	verifiers := webhookVerifiers(secret)

	header := http.Header{}
	header.Set("X-Paladin-Signature", paladinTestSignature(secret, body))

	require.NoError(t, verifiers["alertmanager"].Verify(header, body))
}

func paladinTestSignature(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
