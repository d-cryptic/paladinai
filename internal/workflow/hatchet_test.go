package workflow_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paladinai/paladinai/internal/workflow"
)

func skipIfNoNetwork(t *testing.T) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("network unavailable, skipping httptest-based test: %v", err)
	}
	_ = ln.Close()
}

func samplePayload() workflow.TriagePayload {
	return workflow.TriagePayload{
		TenantID:      "tenant-a",
		IncidentID:    "inc-1",
		Fingerprint:   "fp-1",
		Severity:      "P2",
		CorrelationID: "corr-1",
		Labels:        map[string]string{"env": "prod"},
	}
}

func TestNoopClient_TriggerTriage(t *testing.T) {
	t.Parallel()
	id, err := workflow.NoopClient{}.TriggerTriage(context.Background(), samplePayload())
	assert.NoError(t, err)
	assert.Equal(t, "", id)
}

func TestClient_TriggerTriage_Success(t *testing.T) {
	skipIfNoNetwork(t)

	var gotAuth, gotContentType, gotPath string
	var gotBody map[string]any

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		gotPath = r.URL.Path
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"workflow_run_id": "run-123"})
	}))
	t.Cleanup(ts.Close)

	c := workflow.NewClient(ts.URL, "test-key", nil)
	runID, err := c.TriggerTriage(context.Background(), samplePayload())

	require.NoError(t, err)
	assert.Equal(t, "run-123", runID)
	assert.Equal(t, "Bearer test-key", gotAuth)
	assert.Equal(t, "application/json", gotContentType)
	assert.Equal(t, "/api/v1/workflows/paladin-triage/trigger", gotPath)
	assert.Contains(t, gotBody, "input")
}

func TestClient_TriggerTriage_Created(t *testing.T) {
	skipIfNoNetwork(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"workflow_run_id": "run-456"})
	}))
	t.Cleanup(ts.Close)

	c := workflow.NewClient(ts.URL, "k", nil)
	runID, err := c.TriggerTriage(context.Background(), samplePayload())

	require.NoError(t, err)
	assert.Equal(t, "run-456", runID)
}

func TestClient_TriggerTriage_NonOKStatus(t *testing.T) {
	skipIfNoNetwork(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(ts.Close)

	c := workflow.NewClient(ts.URL, "k", nil)
	runID, err := c.TriggerTriage(context.Background(), samplePayload())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
	assert.Equal(t, "", runID)
}

func TestClient_TriggerTriage_UnparseableBody(t *testing.T) {
	skipIfNoNetwork(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	t.Cleanup(ts.Close)

	c := workflow.NewClient(ts.URL, "k", nil)
	runID, err := c.TriggerTriage(context.Background(), samplePayload())

	// Non-fatal: trigger succeeded, we just cannot report the run ID.
	require.NoError(t, err)
	assert.Equal(t, "", runID)
}

func TestClient_TriggerTriage_RequestError(t *testing.T) {
	t.Parallel()
	// Point at a closed port: dial must fail before any HTTP exchange.
	c := workflow.NewClient("http://127.0.0.1:1", "k", nil)
	_, err := c.TriggerTriage(context.Background(), samplePayload())
	require.Error(t, err)
}

func TestClient_TriggerTriage_BadURL(t *testing.T) {
	t.Parallel()
	c := workflow.NewClient("://bad-url", "k", nil)
	_, err := c.TriggerTriage(context.Background(), samplePayload())
	require.Error(t, err)
}
