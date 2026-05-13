package qdrant

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func skipIfNoNetwork(t *testing.T) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("network unavailable: %v", err)
	}
	ln.Close() //nolint:errcheck
}

func TestClient_EnsureCollection(t *testing.T) {
	t.Parallel()
	skipIfNoNetwork(t)

	var gotMethod, gotPath, gotAPIKey string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("api-key")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result": true, "status": "ok"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "secret", nil)
	if err := c.EnsureCollection(context.Background(), "rb", 8); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("method: %s", gotMethod)
	}
	if gotPath != "/collections/rb" {
		t.Fatalf("path: %s", gotPath)
	}
	if gotAPIKey != "secret" {
		t.Fatalf("api-key not sent: %q", gotAPIKey)
	}
	vec, ok := gotBody["vectors"].(map[string]any)
	if !ok {
		t.Fatalf("missing vectors field: %v", gotBody)
	}
	if vec["distance"] != "Cosine" {
		t.Fatalf("distance: %v", vec["distance"])
	}
}

func TestClient_Upsert(t *testing.T) {
	t.Parallel()
	skipIfNoNetwork(t)

	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "", nil)
	pts := []Point{{ID: "id1", Vector: []float32{1, 2}, Payload: map[string]any{"k": "v"}}}
	if err := c.Upsert(context.Background(), "rb", pts); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	points, ok := gotBody["points"].([]any)
	if !ok || len(points) != 1 {
		t.Fatalf("expected 1 point, got %v", gotBody["points"])
	}
}

func TestClient_Upsert_Empty(t *testing.T) {
	t.Parallel()
	c := New("http://invalid.example.invalid", "", nil)
	if err := c.Upsert(context.Background(), "rb", nil); err != nil {
		t.Fatalf("empty Upsert should be no-op: %v", err)
	}
}

func TestClient_Search(t *testing.T) {
	t.Parallel()
	skipIfNoNetwork(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/points/search") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":[{"id":"a","score":0.9,"payload":{"content":"x"}},{"id":"b","score":0.5,"payload":{"content":"y"}}]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "", nil)
	res, err := c.Search(context.Background(), "rb", []float32{0.1, 0.2}, 5, nil)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}
	if res[0].ID != "a" || res[0].Score != 0.9 {
		t.Fatalf("unexpected first result: %+v", res[0])
	}
	if res[0].Payload["content"] != "x" {
		t.Fatalf("payload missing: %v", res[0].Payload)
	}
}

func TestClient_Delete(t *testing.T) {
	t.Parallel()
	skipIfNoNetwork(t)

	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "", nil)
	if err := c.Delete(context.Background(), "rb", []string{"x", "y"}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	ids, ok := gotBody["points"].([]any)
	if !ok || len(ids) != 2 {
		t.Fatalf("expected 2 ids, got %v", gotBody["points"])
	}
}

func TestClient_ErrorStatus(t *testing.T) {
	t.Parallel()
	skipIfNoNetwork(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"status":"err"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "", nil)
	if err := c.EnsureCollection(context.Background(), "rb", 8); err == nil {
		t.Fatalf("expected error on 400 status")
	}
}
