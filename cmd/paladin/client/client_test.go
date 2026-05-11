// Tests in this file are intentionally NOT parallel because they mutate the
// package-level client.HTTP global. Run sequentially to avoid races.
package client_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
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

// setupStub creates a test server and swaps client.HTTP so requests go to it.
// Tests MUST remain serial (no t.Parallel) because they mutate client.HTTP.
func setupStub(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	skipIfNoNetwork(t)
	ts := httptest.NewServer(handler)
	orig := client.HTTP
	client.HTTP = ts.Client()
	t.Cleanup(func() {
		client.HTTP = orig
		ts.Close()
	})
	return ts
}

// ── Get ───────────────────────────────────────────────────────────────────────

func TestGet_ReturnsBody(t *testing.T) {
	ts := setupStub(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))

	body, err := client.Get(context.Background(), ts.URL, client.Options{})
	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, string(body))
}

func TestGet_SetsAllHeaders(t *testing.T) {
	var got http.Header
	ts := setupStub(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))

	opts := client.Options{
		TenantID:    "acme-corp",
		Token:       "tok-abc",
		AdminSecret: "s3cr3t",
	}
	_, err := client.Get(context.Background(), ts.URL, opts)
	require.NoError(t, err)
	assert.Equal(t, "acme-corp", got.Get("X-Tenant-ID"))
	assert.Equal(t, "Bearer tok-abc", got.Get("Authorization"))
	assert.Equal(t, "s3cr3t", got.Get("X-Admin-Secret"))
}

func TestGet_EmptyOptionsOmitsHeaders(t *testing.T) {
	var got http.Header
	ts := setupStub(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))

	_, err := client.Get(context.Background(), ts.URL, client.Options{})
	require.NoError(t, err)
	assert.Empty(t, got.Get("X-Tenant-ID"))
	assert.Empty(t, got.Get("Authorization"))
	assert.Empty(t, got.Get("X-Admin-Secret"))
}

func TestGet_Non200ReturnsError(t *testing.T) {
	for _, code := range []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			ts := setupStub(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(code)
				_, _ = io.WriteString(w, "error body")
			}))

			_, err := client.Get(context.Background(), ts.URL, client.Options{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), strconv.Itoa(code), "error should include HTTP status code")
		})
	}
}

func TestGet_ContextCancellationReturnsError(t *testing.T) {
	ts := setupStub(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before even sending

	_, err := client.Get(ctx, ts.URL, client.Options{})
	require.Error(t, err)
	// net/http wraps context errors inside *url.Error; unwrap to check the cause.
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		assert.True(t, errors.Is(urlErr.Err, context.Canceled),
			"underlying error should be context.Canceled, got: %v", urlErr.Err)
	}
}

// TestGet_BodyIsCappedAtMaxResponseBytes verifies that LimitReader silently
// truncates oversized responses to exactly MaxResponseBytes and returns no error.
func TestGet_BodyIsCappedAtMaxResponseBytes(t *testing.T) {
	oversized := strings.Repeat("x", int(client.MaxResponseBytes)+1)
	ts := setupStub(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, oversized)
	}))

	body, err := client.Get(context.Background(), ts.URL, client.Options{})
	require.NoError(t, err, "truncation should not return an error")
	assert.Equal(t, int(client.MaxResponseBytes), len(body),
		"body should be truncated to exactly MaxResponseBytes")
}

// ── DoJSON ────────────────────────────────────────────────────────────────────

func TestDoJSON_SetsContentTypeWhenBodyPresent(t *testing.T) {
	var got http.Header
	ts := setupStub(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))

	_, _, err := client.DoJSON(context.Background(), http.MethodPost, ts.URL,
		client.Options{}, bytes.NewBufferString(`{"x":1}`))
	require.NoError(t, err)
	assert.Equal(t, "application/json", got.Get("Content-Type"))
}

func TestDoJSON_NilBodyOmitsContentType(t *testing.T) {
	var got http.Header
	ts := setupStub(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))

	_, _, err := client.DoJSON(context.Background(), http.MethodPost, ts.URL,
		client.Options{}, nil)
	require.NoError(t, err)
	assert.Empty(t, got.Get("Content-Type"))
}

// TestDoJSON_BodyRoundTrip asserts that the request body reaches the server intact.
func TestDoJSON_BodyRoundTrip(t *testing.T) {
	const payload = `{"name":"paladin","value":42}`
	var received string
	ts := setupStub(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		received = string(b)
		w.WriteHeader(http.StatusOK)
	}))

	_, _, err := client.DoJSON(context.Background(), http.MethodPost, ts.URL,
		client.Options{}, strings.NewReader(payload))
	require.NoError(t, err)
	assert.Equal(t, payload, received)
}

func TestDoJSON_ReturnsStatusCode(t *testing.T) {
	for _, want := range []int{http.StatusOK, http.StatusCreated, http.StatusNoContent, http.StatusNotFound} {
		t.Run(http.StatusText(want), func(t *testing.T) {
			ts := setupStub(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(want)
			}))

			_, got, err := client.DoJSON(context.Background(), http.MethodGet, ts.URL,
				client.Options{}, nil)
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func TestDoJSON_SetsAllHeaders(t *testing.T) {
	var got http.Header
	ts := setupStub(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))

	opts := client.Options{TenantID: "t1", Token: "tok", AdminSecret: "sec"}
	_, _, err := client.DoJSON(context.Background(), http.MethodDelete, ts.URL, opts, nil)
	require.NoError(t, err)
	assert.Equal(t, "t1", got.Get("X-Tenant-ID"))
	assert.Equal(t, "Bearer tok", got.Get("Authorization"))
	assert.Equal(t, "sec", got.Get("X-Admin-Secret"))
}

// TestDoJSON_ReturnsBodyOnNon2xx asserts DoJSON returns body+status on non-2xx
// without itself erroring — callers inspect the status and decide.
func TestDoJSON_ReturnsBodyOnNon2xx(t *testing.T) {
	ts := setupStub(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"not found"}`)
	}))

	body, status, err := client.DoJSON(context.Background(), http.MethodGet, ts.URL,
		client.Options{}, nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, status)
	assert.Contains(t, string(body), "not found")
}

// TestGet_TransportErrorReturnsError covers the path where the server is unreachable.
func TestGet_TransportErrorReturnsError(t *testing.T) {
	// Port 1 is always refused on loopback.
	_, err := client.Get(context.Background(), "http://127.0.0.1:1", client.Options{})
	require.Error(t, err)
}
