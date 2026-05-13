package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	rr := httptest.NewRecorder()

	healthz(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Body.String(); got != `{"status":"ok"}` {
		t.Fatalf("healthz body = %q, want ok status JSON", got)
	}
}

func TestReadyz(t *testing.T) {
	rr := httptest.NewRecorder()

	readyz(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("readyz status = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Body.String(); got != `{"status":"ready"}` {
		t.Fatalf("readyz body = %q, want ready status JSON", got)
	}
}
