package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"

	base "github.com/paladinai/paladinai/internal/config"
	cfg "github.com/paladinai/paladinai/services/paladin-ws/config"
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

func TestPublicRouterIncludesAdminRoutesByDefault(t *testing.T) {
	conf := testConfig(0)
	router := newPublicRouter(conf, zap.NewNop(), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusSwitchingProtocols)
	}))

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestPublicRouterOmitsAdminRoutesWhenAdminPortDiffers(t *testing.T) {
	conf := testConfig(9107)
	router := newPublicRouter(conf, zap.NewNop(), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusSwitchingProtocols)
	}))

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rr.Code != http.StatusNotFound {
		t.Fatalf("healthz status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestNewAdminServerOnlyWhenAdminPortDiffers(t *testing.T) {
	if srv := newAdminServer(testConfig(0)); srv != nil {
		t.Fatal("newAdminServer returned server for default admin port")
	}

	srv := newAdminServer(testConfig(9107))
	if srv == nil {
		t.Fatal("newAdminServer returned nil for separate admin port")
	}
	if srv.Addr != ":9107" {
		t.Fatalf("admin server addr = %q, want :9107", srv.Addr)
	}

	rr := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("readyz status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func testConfig(adminPort int) cfg.Config {
	return cfg.Config{
		Server: base.Server{
			Port:         9007,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		AdminPort: adminPort,
		JWTSecret: []byte("ws-secret-at-least-32-bytes!!!!!"),
	}
}
