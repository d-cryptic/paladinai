package integrationpkg

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// ── fakes ─────────────────────────────────────────────────────────────────────

type fakeTokenStore struct {
	tokens     []OAuthToken
	updateErr  error
	updatePath string
	updated    *OAuthToken
}

func (f *fakeTokenStore) ListExpiring(_ context.Context, lookahead time.Duration) ([]OAuthToken, error) {
	var out []OAuthToken
	for _, tok := range f.tokens {
		if tok.IsExpiringSoon(lookahead) {
			out = append(out, tok)
		}
	}
	return out, nil
}

func (f *fakeTokenStore) UpdateToken(_ context.Context, path string, tok OAuthToken) error {
	f.updatePath = path
	f.updated = &tok
	return f.updateErr
}

type fakeHTTPRefresher struct {
	accessToken  string
	refreshToken string
	expiresAt    time.Time
	err          error
	callCount    int32
}

func (f *fakeHTTPRefresher) Refresh(_ context.Context, _, _ string) (string, string, time.Time, error) {
	atomic.AddInt32(&f.callCount, 1)
	return f.accessToken, f.refreshToken, f.expiresAt, f.err
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestOAuthToken_IsExpiringSoon(t *testing.T) {
	t.Run("expiring_soon", func(t *testing.T) {
		tok := OAuthToken{ExpiresAt: time.Now().Add(10 * time.Minute)}
		if !tok.IsExpiringSoon(30 * time.Minute) {
			t.Error("expected IsExpiringSoon=true for token expiring in 10min with 30min lookahead")
		}
	})
	t.Run("not_expiring_soon", func(t *testing.T) {
		tok := OAuthToken{ExpiresAt: time.Now().Add(2 * time.Hour)}
		if tok.IsExpiringSoon(30 * time.Minute) {
			t.Error("expected IsExpiringSoon=false for token expiring in 2h with 30min lookahead")
		}
	})
}

func TestRunCycle_RefreshesExpiringToken(t *testing.T) {
	expiresAt := time.Now().Add(5 * time.Minute)
	store := &fakeTokenStore{
		tokens: []OAuthToken{
			{TenantID: "acme", Provider: "slack", VaultPath: "secret/tenants/acme/slack",
				RefreshToken: "old-refresh", ExpiresAt: expiresAt},
		},
	}
	refresher := &fakeHTTPRefresher{
		accessToken:  "new-access",
		refreshToken: "new-refresh",
		expiresAt:    time.Now().Add(1 * time.Hour),
	}

	var alerted bool
	alertFn := func(_, _, _ string) { alerted = true }

	r := NewOAuthRefresher(store, refresher, alertFn, nil)
	r.runCycle(context.Background())

	if atomic.LoadInt32(&refresher.callCount) != 1 {
		t.Errorf("expected 1 refresh call, got %d", refresher.callCount)
	}
	if store.updated == nil {
		t.Fatal("expected UpdateToken to be called")
	}
	if store.updated.AccessToken != "new-access" {
		t.Errorf("updated access token = %q, want %q", store.updated.AccessToken, "new-access")
	}
	if alerted {
		t.Error("did not expect alert on successful refresh")
	}
}

func TestRunCycle_SkipsNonExpiringToken(t *testing.T) {
	store := &fakeTokenStore{
		tokens: []OAuthToken{
			{TenantID: "acme", Provider: "github", VaultPath: "secret/tenants/acme/github",
				ExpiresAt: time.Now().Add(2 * time.Hour)},
		},
	}
	refresher := &fakeHTTPRefresher{}

	r := NewOAuthRefresher(store, refresher, nil, nil)
	r.runCycle(context.Background())

	if atomic.LoadInt32(&refresher.callCount) != 0 {
		t.Error("expected no refresh call for non-expiring token")
	}
}

func TestRunCycle_AlertsOnRefreshFailure(t *testing.T) {
	store := &fakeTokenStore{
		tokens: []OAuthToken{
			{TenantID: "acme", Provider: "github", VaultPath: "secret/tenants/acme/github",
				ExpiresAt: time.Now().Add(5 * time.Minute)},
		},
	}
	refresher := &fakeHTTPRefresher{err: errors.New("provider 500")}

	var alertedTenant, alertedProvider string
	alertFn := func(tid, prov, _ string) {
		alertedTenant = tid
		alertedProvider = prov
	}

	r := NewOAuthRefresher(store, refresher, alertFn, nil)
	r.runCycle(context.Background())

	if alertedTenant != "acme" {
		t.Errorf("alerted tenant = %q, want %q", alertedTenant, "acme")
	}
	if alertedProvider != "github" {
		t.Errorf("alerted provider = %q, want %q", alertedProvider, "github")
	}
}

func TestRunCycle_AlertsOnVaultUpdateFailure(t *testing.T) {
	store := &fakeTokenStore{
		tokens: []OAuthToken{
			{TenantID: "acme", Provider: "slack", VaultPath: "secret/tenants/acme/slack",
				ExpiresAt: time.Now().Add(5 * time.Minute)},
		},
		updateErr: errors.New("vault unavailable"),
	}
	refresher := &fakeHTTPRefresher{
		accessToken: "ok", refreshToken: "ok", expiresAt: time.Now().Add(1 * time.Hour),
	}

	var alerted bool
	r := NewOAuthRefresher(store, refresher, func(_, _, _ string) { alerted = true }, nil)
	r.runCycle(context.Background())

	if !alerted {
		t.Error("expected alert on vault update failure")
	}
}

func TestRun_StopsOnContextCancel(t *testing.T) {
	store := &fakeTokenStore{}
	refresher := &fakeHTTPRefresher{}

	r := NewOAuthRefresher(store, refresher, nil, nil).WithInterval(50 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.Run(ctx)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("Run did not stop after context cancel within 2s")
	}
}
