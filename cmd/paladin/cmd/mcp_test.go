package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paladinai/paladinai/cmd/paladin/client"
)

func skipIfNoNetwork(t *testing.T) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("network unavailable, skipping httptest-based test: %v", err)
	}
	ln.Close()
}

// newHubStub is a minimal fake paladin-hub for CLI command tests.
// It validates that requests carry either unauthenticated tenant context or a bearer token.
func newHubStub(t *testing.T) *httptest.Server {
	t.Helper()
	skipIfNoNetwork(t)
	mux := http.NewServeMux()

	requireAuthContext := func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("X-Tenant-ID") == "" && r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return false
		}
		return true
	}

	mux.HandleFunc("GET /api/v1/mcp/servers", func(w http.ResponseWriter, r *http.Request) {
		if !requireAuthContext(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{
				"id":           "srv-1",
				"name":         "Test Server",
				"endpoint":     "https://test.example.com",
				"healthy":      true,
				"capabilities": []string{"read", "write"},
			}},
			"meta": map[string]int{"total": 1},
		})
	})

	mux.HandleFunc("GET /api/v1/mcp/servers/{serverID}", func(w http.ResponseWriter, r *http.Request) {
		if !requireAuthContext(w, r) {
			return
		}
		id := r.PathValue("serverID")
		if id != "srv-1" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error":{"code":"NOT_FOUND","message":"server not found"}}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":           "srv-1",
				"name":         "Test Server",
				"endpoint":     "https://test.example.com",
				"healthy":      true,
				"capabilities": []string{"read", "write"},
			},
		})
	})

	mux.HandleFunc("POST /api/v1/mcp/servers", func(w http.ResponseWriter, r *http.Request) {
		if !requireAuthContext(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "new-srv"}})
	})

	mux.HandleFunc("DELETE /api/v1/mcp/servers/{serverID}", func(w http.ResponseWriter, r *http.Request) {
		if !requireAuthContext(w, r) {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /api/v1/mcp/servers/{serverID}/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		if !requireAuthContext(w, r) {
			return
		}
		id := r.PathValue("serverID")
		if id == "missing" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error":{"code":"NOT_FOUND","message":"server not found"}}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	// Swap the global HTTP client so CLI calls hit the stub.
	// Tests MUST remain serial because they mutate process-level globals
	// (client.HTTP, os.Stdout, rootCmd args).
	orig := client.HTTP
	client.HTTP = ts.Client()
	t.Cleanup(func() { client.HTTP = orig })
	return ts
}

// runMCP executes paladin mcp subcommand args and captures stdout.
// It resets rootCmd args after execution to avoid state leaking between tests.
// NOTE: this function is not goroutine-safe; do not call t.Parallel() in tests
// that use it. rootCmdMu is held for the duration to prevent concurrent
// rootCmd mutations from other tests.
func runMCP(t *testing.T, args ...string) (string, error) {
	t.Helper()
	rootCmdMu.Lock()
	defer rootCmdMu.Unlock()

	// Swap os.Stdout for a pipe so we can capture output.
	// Drain in a goroutine to avoid deadlock if output exceeds pipe buffer.
	r, w, err := os.Pipe()
	require.NoError(t, err)

	origOut := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origOut })

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.Copy(&buf, r)
	}()

	rootCmd.SetArgs(args)
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	runErr := rootCmd.Execute()

	// Reset persistent flags that bleed across Execute() calls.
	_ = rootCmd.PersistentFlags().Set("output", "table")
	_ = rootCmd.PersistentFlags().Set("ci", "false")

	w.Close()
	<-done // wait for drain goroutine to finish

	return buf.String(), runErr
}

func TestMCPGet_PrintsServer(t *testing.T) {
	hub := newHubStub(t)

	out, err := runMCP(t, "mcp", "get", "srv-1", "--tenant", "t1", "--api-url", hub.URL)
	require.NoError(t, err)
	assert.Contains(t, out, "srv-1")
	assert.Contains(t, out, "Test Server")
	assert.Contains(t, out, "read, write")
}

func TestMCPGet_JSONOutput(t *testing.T) {
	hub := newHubStub(t)

	out, err := runMCP(t, "mcp", "get", "srv-1", "--tenant", "t1", "--api-url", hub.URL, "--output", "json")
	require.NoError(t, err)
	assert.Contains(t, out, `"data"`)
	assert.Contains(t, out, `"srv-1"`)
}

func TestMCPGet_NotFound(t *testing.T) {
	hub := newHubStub(t)

	_, err := runMCP(t, "mcp", "get", "does-not-exist", "--tenant", "t1", "--api-url", hub.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestMCPGet_MissingTenant(t *testing.T) {
	newHubStub(t)
	_, err := runMCP(t, "mcp", "get", "srv-1", "--tenant", "", "--api-url", "http://localhost")
	require.Error(t, err)
}

func TestMCPHeartbeat_Success(t *testing.T) {
	hub := newHubStub(t)

	out, err := runMCP(t, "mcp", "heartbeat", "srv-1", "--tenant", "t1", "--api-url", hub.URL)
	require.NoError(t, err)
	assert.Contains(t, out, "Heartbeat sent")
}

func TestMCPHeartbeat_NotFound(t *testing.T) {
	hub := newHubStub(t)

	_, err := runMCP(t, "mcp", "heartbeat", "missing", "--tenant", "t1", "--api-url", hub.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestMCPHeartbeat_MissingTenant(t *testing.T) {
	newHubStub(t)
	_, err := runMCP(t, "mcp", "heartbeat", "srv-1", "--tenant", "", "--api-url", "http://localhost")
	require.Error(t, err)
}

func TestMCPList_TableOutput(t *testing.T) {
	hub := newHubStub(t)

	out, err := runMCP(t, "mcp", "list", "--tenant", "t1", "--api-url", hub.URL)
	require.NoError(t, err)
	assert.Contains(t, out, "srv-1")
	assert.Contains(t, out, "Test Server")
}

func TestMCPList_CIModeForcesJSONOutput(t *testing.T) {
	hub := newHubStub(t)

	out, err := runMCP(t, "--ci", "mcp", "list", "--tenant", "t1", "--api-url", hub.URL)
	require.NoError(t, err)
	assert.Contains(t, out, `"data"`)
	assert.Contains(t, out, `"srv-1"`)
	assert.NotContains(t, out, "NAME\tENDPOINT")
}
