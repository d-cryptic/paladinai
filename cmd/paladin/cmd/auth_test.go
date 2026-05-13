package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	c.Flags().Bool("ci", false, "")
	return c
}

func TestAuthLoginCIModeRequiresUserID(t *testing.T) {
	cmd := newAuthLoginTestCmd("http://auth.example.test")
	cmd.SetArgs([]string{
		"--tenant", "tenant-abc",
		"--admin-secret", "admin-secret",
		"--ci",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got != "user-id is required in CI mode — set --user-id" {
		t.Fatalf("error = %q", got)
	}
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

func TestWriteAuthLoginResult_CIModeJSONDoesNotExposeToken(t *testing.T) {
	cmd := newAuthLoginTestCmd("http://auth.example.test")
	if err := cmd.Flags().Set("ci", "true"); err != nil {
		t.Fatal(err)
	}

	stdout := captureStdout(t, func() {
		err := writeAuthLoginResult(cmd, "tenant-abc", authLoginResponse{
			Token:     "issued-token",
			ExpiresIn: 3600,
			TokenType: "Bearer",
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	var result authLoginResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Status != "authenticated" || result.TenantID != "tenant-abc" {
		t.Fatalf("unexpected login result: %+v", result)
	}
	if result.ExpiresIn != 3600 || result.TokenType != "Bearer" {
		t.Fatalf("unexpected token metadata: %+v", result)
	}
	if strings.Contains(stdout, "issued-token") {
		t.Fatalf("login output exposed token: %s", stdout)
	}
}

func TestWriteAuthLoginResult_HumanOutput(t *testing.T) {
	cmd := newAuthLoginTestCmd("http://auth.example.test")

	stdout := captureStdout(t, func() {
		if err := writeAuthLoginResult(cmd, "tenant-abc", authLoginResponse{ExpiresIn: 3600}); err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(stdout, "Logged in successfully.") || !strings.Contains(stdout, "Tenant: tenant-abc") {
		t.Fatalf("stdout = %q, want human login summary", stdout)
	}
}

func TestWriteAuthLogoutResult_CIModeJSON(t *testing.T) {
	cmd := newAuthLoginTestCmd("http://auth.example.test")
	if err := cmd.Flags().Set("ci", "true"); err != nil {
		t.Fatal(err)
	}

	stdout := captureStdout(t, func() {
		if err := writeAuthLogoutResult(cmd, true); err != nil {
			t.Fatal(err)
		}
	})

	var result authLogoutResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || !result.LoggedOut || result.Status != "logged_out" {
		t.Fatalf("unexpected logout result: %+v", result)
	}
	if strings.Contains(stdout, "Logged out.") {
		t.Fatalf("stdout = %q, want json only", stdout)
	}
}

func TestWriteAuthLogoutResult_CIModeJSONWhenNotAuthenticated(t *testing.T) {
	cmd := newAuthLoginTestCmd("http://auth.example.test")
	if err := cmd.Flags().Set("ci", "true"); err != nil {
		t.Fatal(err)
	}

	stdout := captureStdout(t, func() {
		if err := writeAuthLogoutResult(cmd, false); err != nil {
			t.Fatal(err)
		}
	})

	var result authLogoutResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || result.LoggedOut || result.Status != "not_authenticated" {
		t.Fatalf("unexpected logout result: %+v", result)
	}
}

func TestWriteAuthStatusResult_CIModeJSON(t *testing.T) {
	cmd := newAuthLoginTestCmd("http://auth.example.test")
	if err := cmd.Flags().Set("ci", "true"); err != nil {
		t.Fatal(err)
	}

	stdout := captureStdout(t, func() {
		err := writeAuthStatusResult(cmd, authStatusResult{
			Authenticated: true,
			Status:        "authenticated",
			Email:         "user@example.com",
			TenantID:      "tenant-abc",
			Roles:         []string{"admin"},
			ExpiresAt:     "2026-05-13T16:00:00Z",
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	var result authStatusResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Status != "authenticated" {
		t.Fatalf("unexpected status result: %+v", result)
	}
	if result.Email != "user@example.com" || result.TenantID != "tenant-abc" {
		t.Fatalf("unexpected identity fields: %+v", result)
	}
}

func TestWriteAuthStatusResult_HumanUnauthenticated(t *testing.T) {
	cmd := newAuthLoginTestCmd("http://auth.example.test")

	stdout := captureStdout(t, func() {
		if err := writeAuthStatusResult(cmd, authStatusResult{Authenticated: false, Status: "not_authenticated"}); err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(stdout, "not authenticated") {
		t.Fatalf("stdout = %q, want unauthenticated status", stdout)
	}
}
