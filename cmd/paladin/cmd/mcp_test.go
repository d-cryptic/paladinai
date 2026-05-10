package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paladinai/paladinai/cmd/paladin/client"
)

// newHubStub is a minimal fake paladin-hub for CLI command tests.
func newHubStub(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/mcp/servers", func(w http.ResponseWriter, _ *http.Request) {
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

	mux.HandleFunc("POST /api/v1/mcp/servers", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "new-srv"}})
	})

	mux.HandleFunc("DELETE /api/v1/mcp/servers/{serverID}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /api/v1/mcp/servers/{serverID}/heartbeat", func(w http.ResponseWriter, r *http.Request) {
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
	orig := client.HTTP
	client.HTTP = ts.Client()
	t.Cleanup(func() { client.HTTP = orig })
	return ts
}

// runMCP executes a paladin mcp subcommand and returns stdout.
func runMCP(t *testing.T, args ...string) (string, error) {
	t.Helper()
	// Capture stdout by swapping os.Stdout.
	r, w, err := os.Pipe()
	require.NoError(t, err)
	origOut := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origOut })

	rootCmd.SetArgs(args)
	runErr := rootCmd.Execute()

	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String(), runErr
}

func TestMCPGet_PrintsServer(t *testing.T) {
	hub := newHubStub(t)

	out, err := runMCP(t, "mcp", "get", "srv-1", "--tenant", "t1", "--api-url", hub.URL)
	require.NoError(t, err)
	assert.Contains(t, out, "srv-1")
	assert.Contains(t, out, "Test Server")
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

	_, err := runMCP(t, "mcp", "heartbeat", "srv-1", "--tenant", "t1", "--api-url", hub.URL)
	require.NoError(t, err)
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
