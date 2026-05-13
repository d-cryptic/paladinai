package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/auth"
)

// OPAClient is the narrow interface for querying the OPA REST API.
// Production: *http.Client pointing at the OPA sidecar.
// Tests: a fake that returns allow/deny directly.
type OPAClient interface {
	Allow(ctx context.Context, input OPAInput) (bool, error)
}

// OPAInput is the payload sent to OPA for a policy decision.
type OPAInput struct {
	Token    OPAToken    `json:"token"`
	Action   string      `json:"action"`
	Resource OPAResource `json:"resource"`
}

// OPAToken carries the JWT claims forwarded to OPA.
type OPAToken struct {
	TenantID string   `json:"tenant_id"`
	Roles    []string `json:"roles"`
}

// OPAResource is the resource being accessed.
type OPAResource struct {
	TenantID string `json:"tenant_id"`
}

// httpOPAClient calls the OPA REST API at /v1/data/paladin/authz/allow.
type httpOPAClient struct {
	endpoint string
	hc       *http.Client
}

// NewHTTPOPAClient creates a client pointing at the given OPA base URL
// (e.g. "http://opa:8181").
func NewHTTPOPAClient(baseURL string) OPAClient {
	return &httpOPAClient{
		endpoint: baseURL + "/v1/data/paladin/authz/allow",
		hc:       &http.Client{Timeout: 500 * time.Millisecond},
	}
}

type opaRequest struct {
	Input OPAInput `json:"input"`
}

type opaResponse struct {
	Result bool `json:"result"`
}

func (c *httpOPAClient) Allow(ctx context.Context, input OPAInput) (bool, error) {
	body, err := json.Marshal(opaRequest{Input: input})
	if err != nil {
		return false, fmt.Errorf("opa: marshal input: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("opa: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return false, fmt.Errorf("opa: query: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return false, fmt.Errorf("opa: unexpected status %d", resp.StatusCode)
	}

	var out opaResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, fmt.Errorf("opa: decode response: %w", err)
	}
	return out.Result, nil
}

// OPAMiddleware enforces the paladin.authz policy on every request.
// It extracts JWT claims from context (set by JWTMiddleware) and passes them
// to OPA along with the requested action derived from the HTTP method and path.
//
// When OPA is unreachable the middleware fails open with a warn log so a
// sidecar outage does not take down the API. Set failOpen=false to fail closed.
func OPAMiddleware(client OPAClient, failOpen bool, log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID, ok := auth.TenantIDFromContext(r.Context())
			if !ok || tenantID == "" {
				// JWTMiddleware must run before OPAMiddleware; reject unauthenticated requests.
				writeJSONError(w, http.StatusUnauthorized, "missing authentication context")
				return
			}
			roles := auth.RolesFromContext(r.Context())
			if roles == nil {
				roles = []string{}
			}

			action := httpMethodToAction(r.Method)
			input := OPAInput{
				Token:    OPAToken{TenantID: tenantID, Roles: roles},
				Action:   action,
				Resource: OPAResource{TenantID: tenantID},
			}

			allowed, err := client.Allow(r.Context(), input)
			if err != nil {
				log.Warn("opa: policy query failed", zap.Error(err),
					zap.String("tenant", tenantID),
					zap.String("action", action),
				)
				if !failOpen {
					writeJSONError(w, http.StatusForbidden, "authorization service unavailable")
					return
				}
				// fail-open: log and continue
			} else if !allowed {
				log.Info("opa: access denied",
					zap.String("tenant", tenantID),
					zap.String("action", action),
					zap.Strings("roles", roles),
				)
				writeJSONError(w, http.StatusForbidden, "access denied")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// httpMethodToAction maps HTTP verbs to paladin action strings.
func httpMethodToAction(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead:
		return "incidents:read"
	case http.MethodPost:
		return "incidents:update"
	case http.MethodPut, http.MethodPatch:
		return "incidents:update"
	case http.MethodDelete:
		return "incidents:delete"
	default:
		return "incidents:read"
	}
}
