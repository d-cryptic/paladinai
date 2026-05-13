package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/paladinai/paladinai/cmd/paladin/client"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication (login, logout, status)",
}

// paladin auth login
var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate and store credentials",
	Long: `Authenticate with the PaladinAI API and save the token.

The token is stored in the OS keychain. In CI/CD environments, set
PALADIN_TOKEN instead to avoid interactive keychain access.`,
	RunE: runAuthLogin,
}

func runAuthLogin(cmd *cobra.Command, _ []string) error {
	tenantID, _ := cmd.Flags().GetString("tenant")
	if tenantID == "" {
		if cfg, _ := loadConfig(); cfg != nil {
			tenantID = cfg.DefaultTenant
		}
	}
	if tenantID == "" {
		return fmt.Errorf("tenant ID is required — set --tenant or PALADIN_TENANT")
	}

	userID, _ := cmd.Flags().GetString("user-id")
	if userID == "" {
		if isCIMode(cmd) {
			return fmt.Errorf("user-id is required in CI mode — set --user-id")
		}
		r := bufio.NewReader(os.Stdin)
		fmt.Print("User ID: ")
		read, err := readLine(r)
		if err != nil {
			return fmt.Errorf("read user-id: %w", err)
		}
		userID = read
	}
	if userID == "" {
		return fmt.Errorf("user-id is required")
	}

	secret := adminSecret(cmd)
	if secret == "" {
		return fmt.Errorf("admin secret is required — set --admin-secret or PALADIN_ADMIN_SECRET")
	}
	roles := parseRoles(cmd)

	authBase := authURL(cmd)
	u, err := url.Parse(authBase)
	if err != nil {
		return fmt.Errorf("invalid auth-url: %w", err)
	}
	u.Path = "/api/v1/tokens"

	payload := map[string]any{
		"tenant_id": tenantID,
		"user_id":   userID,
		"roles":     roles,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	body, status, err := client.DoJSON(cmd.Context(), http.MethodPost, u.String(),
		client.Options{AdminSecret: secret}, bytes.NewReader(data))
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("login failed (%d): %s", status, string(body))
	}

	var resp authLoginResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if resp.Token == "" {
		return fmt.Errorf("server returned no token")
	}

	cfg, _ := loadConfig()
	if cfg == nil {
		cfg = &PaladinConfig{}
	}
	cfg.AuthEndpoint = strings.TrimRight(authBase, "/")
	cfg.DefaultTenant = tenantID
	cfg.Token = ""

	if err := saveStoredToken(authBase, resp.Token); err != nil {
		return err
	}
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	return writeAuthLoginResult(cmd, tenantID, resp)
}

func parseRoles(cmd *cobra.Command) []string {
	raw, _ := cmd.Flags().GetString("roles")
	if raw == "" {
		return []string{"viewer"}
	}
	parts := strings.Split(raw, ",")
	roles := make([]string, 0, len(parts))
	for _, part := range parts {
		role := strings.TrimSpace(part)
		if role != "" {
			roles = append(roles, role)
		}
	}
	if len(roles) == 0 {
		return []string{"viewer"}
	}
	return roles
}

// paladin auth logout
var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Revoke the stored token",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		authBase := authURL(cmd)
		token := ""
		if err == nil {
			token, err = loadStoredToken(cfg, authBase)
			if err != nil {
				return err
			}
		}
		if err != nil || cfg == nil || token == "" {
			return writeAuthLogoutResult(cmd, false)
		}

		// Attempt server-side revocation (best-effort; don't fail if unavailable).
		u, err := url.Parse(authBase)
		if err == nil {
			u.Path = "/api/v1/auth/logout"
			body, status, rErr := client.DoJSON(cmd.Context(), http.MethodPost, u.String(),
				client.Options{Token: token}, nil)
			if rErr == nil && (status < 200 || status >= 300) {
				fmt.Fprintf(os.Stderr, "server revocation failed (%d): %s\n", status, string(body))
			}
		}

		// Clear local token.
		cfg.Token = ""
		if err := deleteStoredToken(authBase); err != nil {
			return err
		}
		if err := saveConfig(cfg); err != nil {
			return fmt.Errorf("clear config: %w", err)
		}
		return writeAuthLogoutResult(cmd, true)
	},
}

// paladin auth status
var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current authentication state and token expiry",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve token: flag > env > config file.
		token := optToken(cmd)
		if token == "" {
			token = os.Getenv("PALADIN_TOKEN")
		}
		if token == "" {
			if cfg, _ := loadConfig(); cfg != nil {
				stored, err := loadStoredToken(cfg, authURL(cmd))
				if err != nil {
					return err
				}
				token = stored
			}
		}

		if token == "" {
			return writeAuthStatusResult(cmd, authStatusResult{Authenticated: false, Status: "not_authenticated"})
		}

		// Try to verify the token against the auth service.
		authBase := authURL(cmd)
		u, err := url.Parse(authBase)
		if err != nil {
			return fmt.Errorf("invalid auth-url: %w", err)
		}
		u.Path = "/api/v1/auth/me"

		body, err := client.Get(cmd.Context(), u.String(), client.Options{Token: token})
		if err != nil {
			// Network/server unavailable — show local token info only.
			return writeAuthStatusResult(cmd, authStatusResult{Authenticated: true, Status: "server_unavailable"})
		}

		var me struct {
			Email     string   `json:"email"`
			TenantID  string   `json:"tenant_id"`
			Roles     []string `json:"roles"`
			ExpiresAt string   `json:"expires_at"`
		}
		if err := json.Unmarshal(body, &me); err != nil {
			return writeAuthStatusResult(cmd, authStatusResult{Authenticated: true, Status: "parse_error"})
		}

		return writeAuthStatusResult(cmd, authStatusResult{
			Authenticated: true,
			Status:        "authenticated",
			Email:         me.Email,
			TenantID:      me.TenantID,
			Roles:         me.Roles,
			ExpiresAt:     me.ExpiresAt,
		})
	},
}

type authStatusResult struct {
	Authenticated bool     `json:"authenticated"`
	Status        string   `json:"status"`
	Email         string   `json:"email,omitempty"`
	TenantID      string   `json:"tenant_id,omitempty"`
	Roles         []string `json:"roles,omitempty"`
	ExpiresAt     string   `json:"expires_at,omitempty"`
}

type authLoginResponse struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
	TokenType string `json:"token_type"`
}

type authLoginResult struct {
	Authenticated bool   `json:"authenticated"`
	Status        string `json:"status"`
	TenantID      string `json:"tenant_id"`
	ExpiresIn     int    `json:"expires_in,omitempty"`
	TokenType     string `json:"token_type,omitempty"`
}

type authLogoutResult struct {
	Authenticated bool   `json:"authenticated"`
	Status        string `json:"status"`
	LoggedOut     bool   `json:"logged_out"`
}

func writeAuthLoginResult(cmd *cobra.Command, tenantID string, resp authLoginResponse) error {
	if outputFormat(cmd) == "json" {
		result := authLoginResult{
			Authenticated: true,
			Status:        "authenticated",
			TenantID:      tenantID,
			ExpiresIn:     resp.ExpiresIn,
			TokenType:     resp.TokenType,
		}
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			return fmt.Errorf("write auth login: %w", err)
		}
		return nil
	}

	fmt.Fprintln(os.Stdout, "Logged in successfully.")
	if resp.ExpiresIn > 0 {
		fmt.Fprintf(os.Stdout, "Token expires in: %ds\n", resp.ExpiresIn)
	}
	fmt.Fprintf(os.Stdout, "Tenant: %s\n", tenantID)
	return nil
}

func writeAuthLogoutResult(cmd *cobra.Command, loggedOut bool) error {
	if outputFormat(cmd) == "json" {
		status := "not_authenticated"
		if loggedOut {
			status = "logged_out"
		}
		result := authLogoutResult{
			Authenticated: false,
			Status:        status,
			LoggedOut:     loggedOut,
		}
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			return fmt.Errorf("write auth logout: %w", err)
		}
		return nil
	}
	if !loggedOut {
		fmt.Fprintln(os.Stdout, "Not currently logged in.")
		return nil
	}
	fmt.Fprintln(os.Stdout, "Logged out.")
	return nil
}

func writeAuthStatusResult(cmd *cobra.Command, result authStatusResult) error {
	if outputFormat(cmd) == "json" {
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			return fmt.Errorf("write auth status: %w", err)
		}
		return nil
	}

	switch result.Status {
	case "not_authenticated":
		fmt.Fprintln(os.Stdout, "Status:  not authenticated")
		fmt.Fprintln(os.Stdout, "Run `paladin auth login` to authenticate.")
	case "server_unavailable":
		fmt.Fprintln(os.Stdout, "Status:  token present (server unavailable)")
	case "parse_error":
		fmt.Fprintf(os.Stdout, "Status:  authenticated (could not parse details)\n")
	default:
		fmt.Fprintf(os.Stdout, "Status:    authenticated\n")
		if result.Email != "" {
			fmt.Fprintf(os.Stdout, "Email:     %s\n", result.Email)
		}
		if result.TenantID != "" {
			fmt.Fprintf(os.Stdout, "Tenant:    %s\n", result.TenantID)
		}
		if len(result.Roles) > 0 {
			fmt.Fprintf(os.Stdout, "Roles:     %v\n", result.Roles)
		}
		if result.ExpiresAt != "" {
			if t, err := time.Parse(time.RFC3339, result.ExpiresAt); err == nil {
				remaining := time.Until(t).Round(time.Minute)
				if remaining < 0 {
					fmt.Fprintf(os.Stdout, "Expires:   EXPIRED (%s ago)\n", (-remaining).String())
				} else {
					fmt.Fprintf(os.Stdout, "Expires:   %s (%s remaining)\n", result.ExpiresAt[:19], remaining.String())
				}
			} else {
				fmt.Fprintf(os.Stdout, "Expires:   %s\n", result.ExpiresAt)
			}
		}
	}
	return nil
}

func init() {
	authLoginCmd.Flags().String("user-id", "", "User ID to encode in the issued token")
	authLoginCmd.Flags().String("roles", "viewer", "Comma-separated roles for the issued token")
	authLoginCmd.Flags().String("tenant", os.Getenv("PALADIN_TENANT"), "Tenant ID for the issued token")
	authLoginCmd.Flags().String("admin-secret", os.Getenv("PALADIN_ADMIN_SECRET"), "Admin secret for token issuance")
	authLoginCmd.Flags().String("auth-url", envStr("PALADIN_AUTH_URL", "http://localhost:9003"), "PaladinAI auth service URL")

	authLogoutCmd.Flags().String("auth-url", envStr("PALADIN_AUTH_URL", "http://localhost:9003"), "PaladinAI auth service URL")
	authStatusCmd.Flags().String("auth-url", envStr("PALADIN_AUTH_URL", "http://localhost:9003"), "PaladinAI auth service URL")

	authCmd.AddCommand(authLoginCmd, authLogoutCmd, authStatusCmd)
	rootCmd.AddCommand(authCmd)
}
