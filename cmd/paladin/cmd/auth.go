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

	var resp struct {
		Token     string `json:"token"`
		ExpiresIn int    `json:"expires_in"`
		TokenType string `json:"token_type"`
	}
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

	fmt.Fprintln(os.Stdout, "Logged in successfully.")
	if resp.ExpiresIn > 0 {
		fmt.Fprintf(os.Stdout, "Token expires in: %ds\n", resp.ExpiresIn)
	}
	fmt.Fprintf(os.Stdout, "Tenant: %s\n", tenantID)
	return nil
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
			fmt.Fprintln(os.Stdout, "Not currently logged in.")
			return nil
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
		fmt.Fprintln(os.Stdout, "Logged out.")
		return nil
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
			fmt.Fprintln(os.Stdout, "Status:  not authenticated")
			fmt.Fprintln(os.Stdout, "Run `paladin auth login` to authenticate.")
			return nil
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
			fmt.Fprintln(os.Stdout, "Status:  token present (server unavailable)")
			return nil
		}

		var me struct {
			Email     string   `json:"email"`
			TenantID  string   `json:"tenant_id"`
			Roles     []string `json:"roles"`
			ExpiresAt string   `json:"expires_at"`
		}
		if err := json.Unmarshal(body, &me); err != nil {
			fmt.Fprintf(os.Stdout, "Status:  authenticated (could not parse details)\n")
			return nil
		}

		fmt.Fprintf(os.Stdout, "Status:    authenticated\n")
		if me.Email != "" {
			fmt.Fprintf(os.Stdout, "Email:     %s\n", me.Email)
		}
		if me.TenantID != "" {
			fmt.Fprintf(os.Stdout, "Tenant:    %s\n", me.TenantID)
		}
		if len(me.Roles) > 0 {
			fmt.Fprintf(os.Stdout, "Roles:     %v\n", me.Roles)
		}
		if me.ExpiresAt != "" {
			if t, err := time.Parse(time.RFC3339, me.ExpiresAt); err == nil {
				remaining := time.Until(t).Round(time.Minute)
				if remaining < 0 {
					fmt.Fprintf(os.Stdout, "Expires:   EXPIRED (%s ago)\n", (-remaining).String())
				} else {
					fmt.Fprintf(os.Stdout, "Expires:   %s (%s remaining)\n", me.ExpiresAt[:19], remaining.String())
				}
			} else {
				fmt.Fprintf(os.Stdout, "Expires:   %s\n", me.ExpiresAt)
			}
		}
		return nil
	},
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
