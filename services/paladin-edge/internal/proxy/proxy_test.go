package proxy_test

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/auth"
	"github.com/paladinai/paladinai/services/paladin-edge/internal/proxy"
)

func skipIfNoNetwork(t *testing.T) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("network unavailable, skipping httptest-based test: %v", err)
	}
	ln.Close()
}

// newFakeBackend returns a test server that records the last received request.
func newFakeBackend(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	skipIfNoNetwork(t)
	ts := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(ts.Close)
	return ts
}

func TestProxy_ForwardsRequest(t *testing.T) {
	var got *http.Request
	backend := newFakeBackend(t, func(w http.ResponseWriter, r *http.Request) {
		got = r
		w.WriteHeader(http.StatusOK)
	})

	target, err := url.Parse(backend.URL)
	require.NoError(t, err)

	h := proxy.New(target, "/api/v1/mcp", zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, got)
	assert.Equal(t, "/servers", got.URL.Path)
}

func TestProxy_InjectsTenantHeader(t *testing.T) {
	var tenantHdr string
	backend := newFakeBackend(t, func(w http.ResponseWriter, r *http.Request) {
		tenantHdr = r.Header.Get("X-Tenant-ID")
		w.WriteHeader(http.StatusOK)
	})

	target, err := url.Parse(backend.URL)
	require.NoError(t, err)

	h := proxy.New(target, "/api/v1/mcp", zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers", nil)
	ctx := auth.WithTenantID(req.Context(), "acme-corp")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "acme-corp", tenantHdr)
}

func TestProxy_StripsAuthorizationHeader(t *testing.T) {
	var authHdr string
	backend := newFakeBackend(t, func(w http.ResponseWriter, r *http.Request) {
		authHdr = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	})

	target, err := url.Parse(backend.URL)
	require.NoError(t, err)

	h := proxy.New(target, "/api/v1/mcp", zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers", nil)
	req.Header.Set("Authorization", "Bearer supersecret.jwt.token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Empty(t, authHdr, "Authorization must not be forwarded to backend")
}

func TestProxy_InjectsUserIDHeader(t *testing.T) {
	var userHdr string
	backend := newFakeBackend(t, func(w http.ResponseWriter, r *http.Request) {
		userHdr = r.Header.Get("X-User-ID")
		w.WriteHeader(http.StatusOK)
	})

	target, err := url.Parse(backend.URL)
	require.NoError(t, err)

	h := proxy.New(target, "", zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	ctx := auth.WithUserID(req.Context(), "user-42")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, "user-42", userHdr)
}

func TestProxy_NoPrefixPassesPathThrough(t *testing.T) {
	var gotPath string
	backend := newFakeBackend(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})

	target, err := url.Parse(backend.URL)
	require.NoError(t, err)

	h := proxy.New(target, "", zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, "/api/v1/servers", gotPath)
}

func TestProxy_BackendUnavailableReturns502(t *testing.T) {
	// Point at a port that is definitely not listening.
	target, err := url.Parse("http://127.0.0.1:19999")
	require.NoError(t, err)

	h := proxy.New(target, "/api/v1/mcp", zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadGateway, rr.Code)
}

// TestProxy_ClientTenantHeaderOverwritten verifies that a client-supplied
// X-Tenant-ID cannot spoof a different tenant even if the context has no tenant.
func TestProxy_ClientTenantHeaderOverwritten(t *testing.T) {
	var tenantHdr string
	backend := newFakeBackend(t, func(w http.ResponseWriter, r *http.Request) {
		tenantHdr = r.Header.Get("X-Tenant-ID")
		w.WriteHeader(http.StatusOK)
	})

	target, err := url.Parse(backend.URL)
	require.NoError(t, err)

	h := proxy.New(target, "", zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	req.Header.Set("X-Tenant-ID", "evil-tenant") // client-supplied
	// No tenant injected in context — simulates a route not behind JWTMiddleware.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Empty(t, tenantHdr, "client-supplied X-Tenant-ID must be stripped when context has no tenant")
}

// TestProxy_PartialPrefixNotStripped guards against /api/v1/mcpevil being
// mis-treated as if the prefix was "/api/v1/mcp".
func TestProxy_PartialPrefixNotStripped(t *testing.T) {
	var gotPath string
	backend := newFakeBackend(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})

	target, err := url.Parse(backend.URL)
	require.NoError(t, err)

	h := proxy.New(target, "/api/v1/mcp", zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcpevil", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, "/api/v1/mcpevil", gotPath, "partial prefix match must not strip path")
}

// TestProxy_ExactPrefixBecomesSlash verifies that a path that exactly equals
// the prefix (with no trailing content) is forwarded as "/".
func TestProxy_ExactPrefixBecomesSlash(t *testing.T) {
	var gotPath string
	backend := newFakeBackend(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})

	target, err := url.Parse(backend.URL)
	require.NoError(t, err)

	h := proxy.New(target, "/api/v1/mcp", zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, "/", gotPath)
}

// TestProxy_QueryStringPreserved verifies that query parameters survive prefix stripping.
func TestProxy_QueryStringPreserved(t *testing.T) {
	var gotQuery string
	backend := newFakeBackend(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	})

	target, err := url.Parse(backend.URL)
	require.NoError(t, err)

	h := proxy.New(target, "/api/v1/mcp", zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers?limit=10&offset=0", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, "limit=10&offset=0", gotQuery)
}

// TestProxy_ThroughChiRouter verifies that chi's Mount does not rewrite
// r.URL.Path before our Director sees it, so prefix-stripping works end-to-end.
func TestProxy_ThroughChiRouter(t *testing.T) {
	var gotPath string
	var gotTenantHdr string
	backend := newFakeBackend(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotTenantHdr = r.Header.Get("X-Tenant-ID")
		w.WriteHeader(http.StatusOK)
	})

	target, err := url.Parse(backend.URL)
	require.NoError(t, err)

	h := proxy.New(target, "/api/v1/mcp", zap.NewNop())

	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		r.Mount("/mcp", h)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers", nil)
	ctx := auth.WithTenantID(req.Context(), "chi-tenant")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "/servers", gotPath, "chi.Mount must not hide the full path from the proxy Director")
	assert.Equal(t, "chi-tenant", gotTenantHdr)
}

func TestValidate_RejectsNonHTTP(t *testing.T) {
	u, _ := url.Parse("file:///etc/passwd")
	assert.Error(t, proxy.Validate(u))
}

func TestValidate_RejectsMissingHost(t *testing.T) {
	u, _ := url.Parse("http://")
	assert.Error(t, proxy.Validate(u))
}

func TestValidate_AcceptsHTTP(t *testing.T) {
	u, _ := url.Parse("http://paladin-hub:8082")
	assert.NoError(t, proxy.Validate(u))
}
