package client_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paladinai/paladinai/cmd/paladin/client"
)

// setupStub creates a test server and swaps client.HTTP so requests go to it.
// The returned cleanup restores the original client.
func setupStub(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
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
		code := code
		t.Run(http.StatusText(code), func(t *testing.T) {
			ts := setupStub(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(code)
				_, _ = io.WriteString(w, "error body")
			}))

			_, err := client.Get(context.Background(), ts.URL, client.Options{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), fmt.Sprintf("%d", code), "error should include HTTP status code")
		})
	}
}

func TestGet_ContextCancellationReturnsError(t *testing.T) {
	// Server that blocks until the client cancels.
	ts := setupStub(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before even sending

	_, err := client.Get(ctx, ts.URL, client.Options{})
	require.Error(t, err)
}

func TestGet_BodyIsCappedAtMaxResponseBytes(t *testing.T) {
	// Send MaxResponseBytes+1 worth of data; LimitReader should silently truncate.
	oversized := strings.Repeat("x", int(client.MaxResponseBytes)+1)
	ts := setupStub(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, oversized)
	}))

	body, err := client.Get(context.Background(), ts.URL, client.Options{})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(body), int(client.MaxResponseBytes),
		"response body should be capped at MaxResponseBytes")
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

func TestDoJSON_ReturnsStatusCode(t *testing.T) {
	for _, want := range []int{http.StatusOK, http.StatusCreated, http.StatusNoContent, http.StatusNotFound} {
		want := want
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

func TestDoJSON_ReturnsBodyOnError(t *testing.T) {
	ts := setupStub(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"not found"}`)
	}))

	body, status, err := client.DoJSON(context.Background(), http.MethodGet, ts.URL,
		client.Options{}, nil)
	require.NoError(t, err, "DoJSON itself should not error on non-2xx — callers decide")
	assert.Equal(t, http.StatusNotFound, status)
	assert.Contains(t, string(body), "not found")
}
