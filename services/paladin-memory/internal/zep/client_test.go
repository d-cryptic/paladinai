package zep_test

import (
	"context"
	"encoding/json"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/paladinai/paladinai/services/paladin-memory/internal/zep"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skipIfNoNetwork skips tests that require TCP listening (unavailable in sandbox mode).
func skipIfNoNetwork(t *testing.T) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("network unavailable: %v", err)
	}
	ln.Close()
}

// newTestServer creates an httptest.Server bound to 127.0.0.1 (avoids IPv6 sandbox issues).
func newTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	skipIfNoNetwork(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ts := httptest.NewUnstartedServer(handler)
	ts.Listener = ln
	ts.Start()
	t.Cleanup(ts.Close)
	return ts
}

// ─── SessionID ────────────────────────────────────────────────────────────────

func TestSessionID_Format(t *testing.T) {
	assert.Equal(t, "paladin-tenant-1", zep.SessionID("tenant-1"))
	assert.Equal(t, "paladin-acme", zep.SessionID("acme"))
}

func TestSessionID_Empty(t *testing.T) {
	assert.Equal(t, "paladin-", zep.SessionID(""))
}

// ─── ApplyDecay ───────────────────────────────────────────────────────────────

func TestApplyDecay_JustNow_NoDecay(t *testing.T) {
	// validAt = now → t_days = 0 → exp(0) = 1 → score unchanged
	result := zep.ApplyDecay(0.9, time.Now(), "infra")
	assert.InDelta(t, 0.9, float64(result), 0.01)
}

func TestApplyDecay_InfraDomain_SlowDecay(t *testing.T) {
	// infra λ=0.05, 10 days → exp(-0.05 * 10) = exp(-0.5) ≈ 0.6065
	validAt := time.Now().Add(-10 * 24 * time.Hour)
	result := zep.ApplyDecay(1.0, validAt, "infra")
	expected := float32(math.Exp(-0.05 * 10))
	assert.InDelta(t, float64(expected), float64(result), 0.01)
}

func TestApplyDecay_TrafficDomain_MediumDecay(t *testing.T) {
	// traffic λ=0.15, 10 days → exp(-1.5) ≈ 0.2231
	validAt := time.Now().Add(-10 * 24 * time.Hour)
	result := zep.ApplyDecay(1.0, validAt, "traffic")
	expected := float32(math.Exp(-0.15 * 10))
	assert.InDelta(t, float64(expected), float64(result), 0.01)
}

func TestApplyDecay_UnknownDomain_FastDecay(t *testing.T) {
	// unknown λ=0.30, 10 days → exp(-3.0) ≈ 0.0498
	validAt := time.Now().Add(-10 * 24 * time.Hour)
	result := zep.ApplyDecay(1.0, validAt, "unknown")
	expected := float32(math.Exp(-0.30 * 10))
	assert.InDelta(t, float64(expected), float64(result), 0.01)
}

func TestApplyDecay_EmptyDomain_UsesDefault(t *testing.T) {
	validAt := time.Now().Add(-10 * 24 * time.Hour)
	resultEmpty := zep.ApplyDecay(1.0, validAt, "")
	resultUnknown := zep.ApplyDecay(1.0, validAt, "unknown")
	assert.InDelta(t, float64(resultUnknown), float64(resultEmpty), 0.001, "empty domain should use default λ same as unknown")
}

func TestApplyDecay_FutureTimestamp_Clampedto0(t *testing.T) {
	// validAt in the future → t_days < 0 → clamped to 0 → no decay
	futureAt := time.Now().Add(24 * time.Hour)
	result := zep.ApplyDecay(0.8, futureAt, "infra")
	assert.InDelta(t, 0.8, float64(result), 0.01)
}

func TestApplyDecay_InfraDomain_SlowerThanTraffic(t *testing.T) {
	validAt := time.Now().Add(-7 * 24 * time.Hour)
	infraScore := zep.ApplyDecay(1.0, validAt, "infra")
	trafficScore := zep.ApplyDecay(1.0, validAt, "traffic")
	assert.Greater(t, float64(infraScore), float64(trafficScore), "infra decays slower than traffic")
}

func TestApplyDecay_TrafficSlowerThanUnknown(t *testing.T) {
	validAt := time.Now().Add(-7 * 24 * time.Hour)
	trafficScore := zep.ApplyDecay(1.0, validAt, "traffic")
	unknownScore := zep.ApplyDecay(1.0, validAt, "unknown")
	assert.Greater(t, float64(trafficScore), float64(unknownScore), "traffic decays slower than unknown")
}

// ─── AddEpisode validation ─────────────────────────────────────────────────────

func TestAddEpisode_EmptyTenantID_Error(t *testing.T) {
	c := zep.NewClient("http://localhost:8080", "", nil)
	err := c.AddEpisode(context.Background(), zep.Episode{ID: "ep-1", TenantID: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant_id")
}

func TestAddEpisode_EmptyID_Error(t *testing.T) {
	c := zep.NewClient("http://localhost:8080", "", nil)
	err := c.AddEpisode(context.Background(), zep.Episode{ID: "", TenantID: "t1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "id")
}

// ─── AddEpisode HTTP integration ──────────────────────────────────────────────

func TestAddEpisode_PostsToCorrectEndpoint(t *testing.T) {
	var gotPath string
	var gotBody []byte
	ts := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotBody, _ = readAll(r)
		w.WriteHeader(http.StatusOK)
	}))

	c := zep.NewClient(ts.URL, "test-key", nil)
	ep := zep.Episode{
		ID:         "ep-abc",
		TenantID:   "tenant-1",
		IncidentID: "inc-1",
		Content:    "DB connection pool exhausted",
		ValidAt:    time.Now().Add(-time.Hour),
		RecordedAt: time.Now(),
		Domain:     "infra",
	}
	err := c.AddEpisode(context.Background(), ep)
	require.NoError(t, err)

	assert.Equal(t, "/sessions/paladin-tenant-1/messages", gotPath)

	var reqBody map[string]any
	require.NoError(t, json.Unmarshal(gotBody, &reqBody))
	messages, ok := reqBody["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 1)
	msg := messages[0].(map[string]any)
	assert.Equal(t, "ep-abc", msg["uuid"])
	assert.Equal(t, "system", msg["role_type"])
}

func TestAddEpisode_ServerError_ReturnsError(t *testing.T) {
	ts := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal"}`))
	}))

	c := zep.NewClient(ts.URL, "", nil)
	err := c.AddEpisode(context.Background(), zep.Episode{ID: "ep-1", TenantID: "t1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestAddEpisode_ServerError_TruncatesBody(t *testing.T) {
	ts := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(strings.Repeat("x", 70*1024)))
	}))

	c := zep.NewClient(ts.URL, "", nil)
	err := c.AddEpisode(context.Background(), zep.Episode{ID: "ep-1", TenantID: "t1"})
	require.Error(t, err)
	if strings.Count(err.Error(), "x") > 64*1024 {
		t.Fatalf("error body was not capped: %d x chars", strings.Count(err.Error(), "x"))
	}
}

// ─── SearchEpisodes ───────────────────────────────────────────────────────────

func TestSearchEpisodes_EmptyTenantID_Error(t *testing.T) {
	c := zep.NewClient("http://localhost:8080", "", nil)
	_, err := c.SearchEpisodes(context.Background(), "", "query", 5)
	require.Error(t, err)
}

func TestSearchEpisodes_DecayAppliedToResults(t *testing.T) {
	validAt := time.Now().Add(-10 * 24 * time.Hour)

	ts := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"results": []map[string]any{
				{
					"message": map[string]any{
						"uuid":       "ep-1",
						"content":    "DB pool exhausted",
						"created_at": validAt.Format(time.RFC3339),
						"metadata":   map[string]any{"domain": "infra"},
						"role_type":  "system",
					},
					"score": 0.85,
				},
			},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))

	c := zep.NewClient(ts.URL, "", nil)
	results, err := c.SearchEpisodes(context.Background(), "tenant-1", "connection pool", 5)
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.Equal(t, float32(0.85), results[0].Score)
	// Decay score should be less than raw score for a 10-day old episode
	assert.Less(t, results[0].DecayScore, results[0].Score)
	assert.Greater(t, results[0].DecayScore, float32(0))
}

func TestSearchEpisodes_RejectsOversizedResponse(t *testing.T) {
	ts := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results":[],"padding":"` + strings.Repeat("x", 4<<20) + `"}`))
	}))

	c := zep.NewClient(ts.URL, "", nil)
	_, err := c.SearchEpisodes(context.Background(), "tenant-1", "connection pool", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "response body exceeds")
}

func TestSearchEpisodes_RejectsMultipleJSONDocuments(t *testing.T) {
	ts := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results":[]} {}`))
	}))

	c := zep.NewClient(ts.URL, "", nil)
	_, err := c.SearchEpisodes(context.Background(), "tenant-1", "connection pool", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "single JSON document")
}

// ─── DeleteSession ─────────────────────────────────────────────────────────────

func TestDeleteSession_EmptyTenantID_Error(t *testing.T) {
	c := zep.NewClient("http://localhost:8080", "", nil)
	err := c.DeleteSession(context.Background(), "")
	require.Error(t, err)
}

func TestDeleteSession_SendsDELETERequest(t *testing.T) {
	var gotMethod, gotPath string
	ts := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))

	c := zep.NewClient(ts.URL, "", nil)
	err := c.DeleteSession(context.Background(), "tenant-2")
	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, gotMethod)
	assert.Equal(t, "/sessions/paladin-tenant-2", gotPath)
}

func readAll(r *http.Request) ([]byte, error) {
	b := make([]byte, 0, 512)
	buf := make([]byte, 512)
	for {
		n, err := r.Body.Read(buf)
		b = append(b, buf[:n]...)
		if err != nil {
			break
		}
	}
	return b, nil
}
