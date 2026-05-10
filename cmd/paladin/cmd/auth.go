package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
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

The token is written to ~/.paladin/config.yaml. In CI/CD environments,
set PALADIN_TOKEN instead to avoid filesystem writes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		email, _ := cmd.Flags().GetString("email")
		password, _ := cmd.Flags().GetString("password")

		r := bufio.NewReader(os.Stdin)
		if email == "" {
			fmt.Print("Email: ")
			e, err := readLine(r)
			if err != nil {
				return fmt.Errorf("read email: %w", err)
			}
			email = e
		}
		if password == "" {
			fmt.Print("Password: ")
			p, err := readLine(r)
			if err != nil {
				return fmt.Errorf("read password: %w", err)
			}
			password = p
		}

		authBase := authURL(cmd)
		u, err := url.Parse(authBase)
		if err != nil {
			return fmt.Errorf("invalid auth-url: %w", err)
		}
		u.Path = "/api/v1/auth/login"

		payload := map[string]string{"email": email, "password": password}
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}

		body, status, err := client.DoJSON(cmd.Context(), http.MethodPost, u.String(),
			client.Options{}, bytes.NewReader(data))
		if err != nil {
			return err
		}
		if status < 200 || status >= 300 {
			return fmt.Errorf("login failed (%d): %s", status, string(body))
		}

		var resp struct {
			Token    string `json:"token"`
			TenantID string `json:"tenant_id"`
			ExpiresAt string `json:"expires_at"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
		if resp.Token == "" {
			return fmt.Errorf("server returned no token")
		}

		// Load existing config and update token.
		cfg, _ := loadConfig()
		if cfg == nil {
			cfg = &PaladinConfig{}
		}
		cfg.Token = resp.Token
		if resp.TenantID != "" && cfg.DefaultTenant == "" {
			cfg.DefaultTenant = resp.TenantID
		}

		if err := saveConfig(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Fprintln(os.Stdout, "Logged in successfully.")
		if resp.ExpiresAt != "" {
			fmt.Fprintf(os.Stdout, "Token expires: %s\n", resp.ExpiresAt)
		}
		if resp.TenantID != "" {
			fmt.Fprintf(os.Stdout, "Tenant: %s\n", resp.TenantID)
		}
		return nil
	},
}

// paladin auth logout
var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Revoke the stored token",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil || cfg == nil || cfg.Token == "" {
			fmt.Fprintln(os.Stdout, "Not currently logged in.")
			return nil
		}

		// Attempt server-side revocation (best-effort; don't fail if unavailable).
		authBase := authURL(cmd)
		u, err := url.Parse(authBase)
		if err == nil {
			u.Path = "/api/v1/auth/logout"
			body, status, rErr := client.DoJSON(cmd.Context(), http.MethodPost, u.String(),
				client.Options{Token: cfg.Token}, nil)
			if rErr == nil && (status < 200 || status >= 300) {
				fmt.Fprintf(os.Stderr, "server revocation failed (%d): %s\n", status, string(body))
			}
		}

		// Clear local token.
		cfg.Token = ""
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
				token = cfg.Token
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
			Email     string `json:"email"`
			TenantID  string `json:"tenant_id"`
			Roles     []string `json:"roles"`
			ExpiresAt string `json:"expires_at"`
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
	authLoginCmd.Flags().String("email", "", "Email address")
	authLoginCmd.Flags().String("password", "", "Password (prefer interactive prompt)")
	authLoginCmd.Flags().String("auth-url", envStr("PALADIN_AUTH_URL", "http://localhost:9003"), "PaladinAI auth service URL")

	authLogoutCmd.Flags().String("auth-url", envStr("PALADIN_AUTH_URL", "http://localhost:9003"), "PaladinAI auth service URL")
	authStatusCmd.Flags().String("auth-url", envStr("PALADIN_AUTH_URL", "http://localhost:9003"), "PaladinAI auth service URL")

	authCmd.AddCommand(authLoginCmd, authLogoutCmd, authStatusCmd)
	rootCmd.AddCommand(authCmd)
}
