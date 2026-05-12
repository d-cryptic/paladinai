// oauth.go implements the background OAuth token refresh job from
// docs/plans/07.integrations-stage7.md §12 (Token Refresh).
//
// OAuthRefresher polls a TokenStore every interval (default 15min) and
// proactively refreshes any token expiring within a lookahead window
// (default 30min). On refresh failure it calls the configured AlertFunc
// so the tenant can be notified via Slack or on-call paging.
//
// The TokenStore and HTTPRefresher interfaces are narrow and easy to fake
// in tests. The real implementations connect to Vault and provider APIs.
package integrationpkg

import (
	"context"
	"fmt"
	"time"
)

// OAuthToken represents a stored OAuth credential set for one tenant+provider.
type OAuthToken struct {
	TenantID     string
	Provider     string // "github" | "slack" | "confluence" | "jira"
	VaultPath    string
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	Scopes       []string
}

// IsExpiringSoon reports whether the token expires within the given lookahead.
func (t OAuthToken) IsExpiringSoon(lookahead time.Duration) bool {
	return time.Until(t.ExpiresAt) < lookahead
}

// TokenStore is the narrow Vault interface used by OAuthRefresher.
// The real implementation reads from secret/tenants/{tenantID}/{provider}.
type TokenStore interface {
	// ListExpiring returns all tokens that expire within lookahead.
	ListExpiring(ctx context.Context, lookahead time.Duration) ([]OAuthToken, error)
	// UpdateToken writes refreshed credentials back to Vault.
	UpdateToken(ctx context.Context, path string, token OAuthToken) error
}

// HTTPRefresher exchanges a refresh_token for new credentials via the provider's API.
type HTTPRefresher interface {
	// Refresh calls the provider's token endpoint and returns updated credentials.
	Refresh(ctx context.Context, provider, refreshToken string) (accessToken, newRefreshToken string, expiresAt time.Time, err error)
}

// AlertFunc is called when a token refresh fails so the tenant can be notified.
// The real implementation posts to a Slack channel or pages the on-call.
type AlertFunc func(tenantID, provider, message string)

// OAuthRefresher runs a background loop that refreshes expiring OAuth tokens.
type OAuthRefresher struct {
	store     TokenStore
	refresher HTTPRefresher
	alertFn   AlertFunc
	interval  time.Duration // how often to scan (default: 15min)
	lookahead time.Duration // refresh tokens expiring within this window (default: 30min)
}

// NewOAuthRefresher creates a refresher with sensible defaults.
func NewOAuthRefresher(store TokenStore, refresher HTTPRefresher, alertFn AlertFunc) *OAuthRefresher {
	return &OAuthRefresher{
		store:     store,
		refresher: refresher,
		alertFn:   alertFn,
		interval:  15 * time.Minute,
		lookahead: 30 * time.Minute,
	}
}

// WithInterval overrides the scan interval (useful for tests / faster refresh in prod).
func (r *OAuthRefresher) WithInterval(d time.Duration) *OAuthRefresher {
	r.interval = d
	return r
}

// WithLookahead overrides the expiry lookahead window.
func (r *OAuthRefresher) WithLookahead(d time.Duration) *OAuthRefresher {
	r.lookahead = d
	return r
}

// Run starts the refresh loop. It blocks until ctx is cancelled.
func (r *OAuthRefresher) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	// Run one cycle immediately on startup.
	r.runCycle(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.runCycle(ctx)
		}
	}
}

func (r *OAuthRefresher) runCycle(ctx context.Context) {
	tokens, err := r.store.ListExpiring(ctx, r.lookahead)
	if err != nil {
		// Non-fatal: log would happen at the call site via injected logger.
		return
	}

	for _, tok := range tokens {
		if err := r.refreshOne(ctx, tok); err != nil {
			if r.alertFn != nil {
				r.alertFn(tok.TenantID, tok.Provider,
					fmt.Sprintf("OAuth refresh failed: %v", err))
			}
		}
	}
}

func (r *OAuthRefresher) refreshOne(ctx context.Context, tok OAuthToken) error {
	accessToken, newRefresh, expiresAt, err := r.refresher.Refresh(ctx, tok.Provider, tok.RefreshToken)
	if err != nil {
		return fmt.Errorf("provider %s refresh for tenant %s: %w", tok.Provider, tok.TenantID, err)
	}

	updated := OAuthToken{
		TenantID:     tok.TenantID,
		Provider:     tok.Provider,
		VaultPath:    tok.VaultPath,
		AccessToken:  accessToken,
		RefreshToken: newRefresh,
		ExpiresAt:    expiresAt,
		Scopes:       tok.Scopes,
	}
	if err := r.store.UpdateToken(ctx, tok.VaultPath, updated); err != nil {
		return fmt.Errorf("update vault path %s: %w", tok.VaultPath, err)
	}
	return nil
}
