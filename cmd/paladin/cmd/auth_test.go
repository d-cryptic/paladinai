package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
)

func newAuthLoginTestCmd(authURL string) *cobra.Command {
	c := &cobra.Command{Use: "login", RunE: runAuthLogin}
	c.Flags().String("auth-url", authURL, "")
	c.Flags().String("tenant", "", "")
	c.Flags().String("user-id", "", "")
	c.Flags().String("roles", "viewer", "")
	c.Flags().String("admin-secret", "", "")
	return c
}

func TestAuthLoginIssuesTokenAndStoresInKeychain(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("PALADIN_TOKEN", "")
	store := &fakeSecretStore{}
	withSecretStore(t, store)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tokens" {
			t.Fatalf("path = %q, want /api/v1/tokens", r.URL.Path)
		}
		if got := r.Header.Get("X-Admin-Secret"); got != "admin-secret" {
			t.Fatalf("admin secret header = %q", got)
		}
		var body struct {
			TenantID string   `json:"tenant_id"`
			UserID   string   `json:"user_id"`
			Roles    []string `json:"roles"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.TenantID != "tenant-abc" || body.UserID != "user-1" || len(body.Roles) != 2 {
			t.Fatalf("unexpected body: %+v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      "issued-token",
			"expires_in": 3600,
			"token_type": "Bearer",
		})
	}))
	defer srv.Close()

	cmd := newAuthLoginTestCmd(srv.URL)
	cmd.SetArgs([]string{
		"--tenant", "tenant-abc",
		"--user-id", "user-1",
		"--roles", "admin,viewer",
		"--admin-secret", "admin-secret",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Token != "" {
		t.Fatalf("config token = %q, want empty", cfg.Token)
	}
	if cfg.DefaultTenant != "tenant-abc" {
		t.Fatalf("default tenant = %q, want tenant-abc", cfg.DefaultTenant)
	}
	stored, err := loadStoredToken(cfg, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if stored != "issued-token" {
		t.Fatalf("stored token = %q, want issued-token", stored)
	}
}
