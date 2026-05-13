//go:build e2e

// E2E smoke test: sends an alert through the full paladin pipeline and verifies
// it is processed end-to-end.
//
// Requires all services running via `make up && make up-all`:
//   - PALADIN_INGEST_URL  (default: http://localhost:9001)
//   - PALADIN_AUTH_URL   (default: http://localhost:9003)
//   - PALADIN_HUB_URL    (default: http://localhost:8082)
//
// Run with: go test -tags e2e -timeout 120s ./test/e2e/...
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	defaultIngestURL = "http://localhost:9001"
	defaultAuthURL   = "http://localhost:9003"
	defaultHubURL    = "http://localhost:8082"
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func ingestURL() string   { return envOr("PALADIN_INGEST_URL", defaultIngestURL) }
func authURL() string     { return envOr("PALADIN_AUTH_URL", defaultAuthURL) }
func hubURL() string      { return envOr("PALADIN_HUB_URL", defaultHubURL) }
func adminSecret() string { return envOr("ADMIN_SECRET", "paladin-admin-secret") }
func jwtSecret() string   { return envOr("JWT_SECRET", "paladin-dev-jwt-secret-min-32-chars!!") }

// waitForHealthz polls /healthz until ready or timeout.
func waitForHealthz(t *testing.T, base string, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/healthz") //nolint:gosec,noctx
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close() //nolint:errcheck
			return
		}
		if resp != nil {
			resp.Body.Close() //nolint:errcheck
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("service at %s did not become healthy within %s", base, d)
}

func TestServicesHealthy(t *testing.T) {
	for _, svc := range []struct{ name, url string }{
		{"paladin-ingest", ingestURL()},
		{"paladin-auth", authURL()},
		{"paladin-hub", hubURL()},
	} {
		t.Run(svc.name, func(t *testing.T) {
			waitForHealthz(t, svc.url, 10*time.Second)
		})
	}
}

func TestAuthFlow_CreateTenantAndIssueToken(t *testing.T) {
	waitForHealthz(t, authURL(), 10*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	slug := fmt.Sprintf("e2e-tenant-%d", time.Now().UnixNano())

	// Create tenant
	body := fmt.Sprintf(`{"slug":%q,"name":"E2E Test Tenant"}`, slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authURL()+"/api/v1/tenants", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Secret", adminSecret())

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	require.Equal(t, http.StatusCreated, resp.StatusCode, "expected 201 Created")

	var createResp struct {
		Tenant struct {
			ID   string `json:"id"`
			Slug string `json:"slug"`
		} `json:"tenant"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&createResp))
	assert.NotEmpty(t, createResp.Tenant.ID)
	assert.Equal(t, slug, createResp.Tenant.Slug)

	// Issue token for the new tenant
	tokenBody := fmt.Sprintf(`{"tenant_id":%q,"user_id":"e2e-user"}`, createResp.Tenant.ID)
	req2, err := http.NewRequestWithContext(ctx, http.MethodPost, authURL()+"/api/v1/tokens", strings.NewReader(tokenBody))
	require.NoError(t, err)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Admin-Secret", adminSecret())

	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer resp2.Body.Close() //nolint:errcheck

	require.Equal(t, http.StatusOK, resp2.StatusCode)
	var tokenResp struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&tokenResp))
	assert.NotEmpty(t, tokenResp.Token, "expected a JWT in the response")
}

func TestIngestPipeline_AlertmanagerWebhook(t *testing.T) {
	waitForHealthz(t, ingestURL(), 10*time.Second)
	waitForHealthz(t, authURL(), 10*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create a tenant and get a token
	slug := fmt.Sprintf("e2e-ingest-%d", time.Now().UnixNano())
	body := fmt.Sprintf(`{"slug":%q,"name":"E2E Ingest Tenant"}`, slug)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, authURL()+"/api/v1/tenants", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Secret", adminSecret())
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var createResp struct {
		Tenant struct {
			ID string `json:"id"`
		} `json:"tenant"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&createResp))

	tokenBody := fmt.Sprintf(`{"tenant_id":%q,"user_id":"e2e-ingest-user"}`, createResp.Tenant.ID)
	req2, _ := http.NewRequestWithContext(ctx, http.MethodPost, authURL()+"/api/v1/tokens", strings.NewReader(tokenBody))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Admin-Secret", adminSecret())
	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer resp2.Body.Close() //nolint:errcheck

	var tokenResp struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&tokenResp))
	require.NotEmpty(t, tokenResp.Token)

	// Send an Alertmanager webhook to paladin-ingest
	alertPayload := map[string]any{
		"version":  "4",
		"groupKey": "{}:{alertname=\"PostgreSQLDown\"}",
		"status":   "firing",
		"receiver": "paladin",
		"groupLabels": map[string]string{
			"alertname": "PostgreSQLDown",
		},
		"commonLabels": map[string]string{
			"alertname": "PostgreSQLDown",
			"severity":  "P1",
			"namespace": "prod",
		},
		"commonAnnotations": map[string]string{
			"description": "Primary database is unreachable",
		},
		"externalURL": "http://alertmanager:9093",
		"alerts": []map[string]any{
			{
				"status": "firing",
				"labels": map[string]string{
					"alertname": "PostgreSQLDown",
					"severity":  "P1",
					"namespace": "prod",
					"job":       "postgres",
					"service":   "orders-db",
					"cluster":   "us-east-1",
				},
				"annotations": map[string]string{
					"description": "Primary database is unreachable for 5 minutes",
				},
				"startsAt": time.Now().UTC().Format(time.RFC3339),
				"endsAt":   "0001-01-01T00:00:00Z",
			},
		},
	}

	data, _ := json.Marshal(alertPayload)
	ingestReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		ingestURL()+"/api/v1/ingest/alertmanager",
		bytes.NewReader(data),
	)
	require.NoError(t, err)
	ingestReq.Header.Set("Content-Type", "application/json")
	ingestReq.Header.Set("Authorization", "Bearer "+tokenResp.Token)

	ingestResp, err := http.DefaultClient.Do(ingestReq)
	require.NoError(t, err)
	defer ingestResp.Body.Close() //nolint:errcheck

	body2, _ := io.ReadAll(ingestResp.Body)
	assert.Equal(t, http.StatusAccepted, ingestResp.StatusCode,
		"expected 202 Accepted from ingest; body=%s", string(body2))
}

func TestHubRegistry_ListServers(t *testing.T) {
	waitForHealthz(t, hubURL(), 10*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hubURL()+"/api/v1/servers", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
