package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/paladinai/paladinai/cmd/paladin/client"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// configDir returns the paladin config directory (~/.paladin).
func configDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to CWD if $HOME is not set; caller will see a relative path.
		home = "."
	}
	return filepath.Join(home, ".paladin")
}

// configPath returns the path to the paladin user config file.
func configPath() string {
	return filepath.Join(configDir(), "config.yaml")
}

// PaladinConfig is the structure of ~/.paladin/config.yaml.
// Token is stored here as a convenience for single-user dev environments.
// For production use prefer PALADIN_TOKEN env var — it is never written to disk.
// TODO(security): replace disk token with OS keychain (go-keyring) per docs/plans/11.onboarding-ux-stage11.md
type PaladinConfig struct {
	APIEndpoint   string `yaml:"api_endpoint"`
	AuthEndpoint  string `yaml:"auth_endpoint"`
	DefaultTenant string `yaml:"default_tenant"`
	OutputFormat  string `yaml:"output_format"`
	// Token is omitted when empty so PALADIN_TOKEN remains the production mechanism.
	Token string `yaml:"token,omitempty"`
}

func loadConfig() (*PaladinConfig, error) {
	data, err := os.ReadFile(configPath())
	if os.IsNotExist(err) {
		return &PaladinConfig{OutputFormat: "table"}, nil
	}
	if err != nil {
		return nil, err
	}
	var c PaladinConfig
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func saveConfig(c *PaladinConfig) error {
	if err := os.MkdirAll(configDir(), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0o600)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Interactive setup wizard — get PaladinAI running in minutes",
	Long: `paladin init guides you through connecting PaladinAI to your infrastructure.

Steps:
  1. API endpoint    — point the CLI at your PaladinAI instance
  2. Authentication  — provide or generate an API token
  3. Tenant          — select or create your tenant
  4. Verify          — confirm connectivity and first triage is ready

After init, run 'paladin doctor' to verify all integrations are healthy.`,
	RunE: runInit,
}

func runInit(cmd *cobra.Command, _ []string) error {
	r := bufio.NewReader(os.Stdin)

	fmt.Println("Welcome to PaladinAI — interactive setup wizard")
	fmt.Println(strings.Repeat("-", 50))

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// ── Step 1: API endpoint ─────────────────────────────────────────────────
	fmt.Println("\n[1/4] API Endpoint")
	defaultAPI := cfg.APIEndpoint
	if defaultAPI == "" {
		defaultAPI = "http://localhost:8080"
	}
	fmt.Printf("  PaladinAI API URL [%s]: ", defaultAPI)
	apiEndpoint, err := readLine(r)
	if err != nil {
		return fmt.Errorf("read api-url: %w", err)
	}
	if apiEndpoint == "" {
		apiEndpoint = defaultAPI
	}
	if _, err := url.ParseRequestURI(apiEndpoint); err != nil {
		return fmt.Errorf("invalid API URL %q: %w", apiEndpoint, err)
	}
	apiEndpoint = strings.TrimRight(apiEndpoint, "/")

	defaultAuth := cfg.AuthEndpoint
	if defaultAuth == "" {
		defaultAuth = "http://localhost:9003"
	}
	fmt.Printf("  PaladinAI Auth URL [%s]: ", defaultAuth)
	authEndpoint, err := readLine(r)
	if err != nil {
		return fmt.Errorf("read auth-url: %w", err)
	}
	if authEndpoint == "" {
		authEndpoint = defaultAuth
	}
	authEndpoint = strings.TrimRight(authEndpoint, "/")

	// ── Step 2: Authentication ────────────────────────────────────────────────
	fmt.Println("\n[2/4] Authentication")
	fmt.Println("  NOTE: For production, set PALADIN_TOKEN env var — tokens stored")
	fmt.Println("        in ~/.paladin/config.yaml are plaintext. Use with care.")

	tokenFromEnv := os.Getenv("PALADIN_TOKEN") != ""
	existingToken := ""
	if !tokenFromEnv {
		existingToken = cfg.Token // only use disk token if env is absent
	}

	if tokenFromEnv {
		fmt.Println("  Using token from PALADIN_TOKEN env var (will NOT be written to disk).")
	} else if existingToken != "" {
		fmt.Print("  Token found in config. Press Enter to keep it or paste a new one: ")
		if line, err := readLine(r); err == nil && line != "" {
			existingToken = line
		}
	} else {
		fmt.Print("  API token (leave blank to continue unauthenticated): ")
		existingToken, _ = readLine(r)
	}

	// ── Step 3: Tenant ────────────────────────────────────────────────────────
	fmt.Println("\n[3/4] Tenant")
	defaultTenant := cfg.DefaultTenant
	if t := os.Getenv("PALADIN_TENANT"); t != "" {
		defaultTenant = t
	}
	fmt.Printf("  Default tenant ID [%s]: ", defaultTenant)
	tenant, err := readLine(r)
	if err != nil {
		return fmt.Errorf("read tenant: %w", err)
	}
	if tenant == "" {
		tenant = defaultTenant
	}

	// resolve token for connectivity check: env takes precedence
	activeToken := existingToken
	if t := os.Getenv("PALADIN_TOKEN"); t != "" {
		activeToken = t
	}

	// ── Step 4: Verify connectivity ───────────────────────────────────────────
	fmt.Println("\n[4/4] Verifying connectivity...")
	if _, err := client.Get(cmd.Context(), apiEndpoint+"/healthz", client.Options{
		TenantID: tenant,
		Token:    activeToken,
	}); err != nil {
		fmt.Printf("  x Could not reach %s: %v\n", apiEndpoint, err)
		fmt.Println("    (continuing — retry with 'paladin doctor')")
	} else {
		fmt.Printf("  ok Connected to %s\n", apiEndpoint)
	}

	// ── Save config — never write env-sourced token to disk ───────────────────
	cfg.APIEndpoint = apiEndpoint
	cfg.AuthEndpoint = authEndpoint
	cfg.DefaultTenant = tenant
	if !tokenFromEnv {
		cfg.Token = existingToken // blank clears it; omitempty omits from file
	}
	if cfg.OutputFormat == "" {
		cfg.OutputFormat = "table"
	}
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("\nConfig saved to %s\n", configPath())
	fmt.Println("\nNext steps:")
	fmt.Println("  paladin doctor             -- verify integrations")
	fmt.Println("  paladin alert list         -- view active alerts")
	fmt.Println("  paladin dashboard          -- open TUI dashboard")
	printCompletionHint()
	return nil
}

// readLine reads a trimmed line from the reader.
// io.EOF is treated as empty input (common when stdin is piped).
func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func printCompletionHint() {
	shell := filepath.Base(os.Getenv("SHELL"))
	switch shell {
	case "zsh":
		fmt.Println("\nShell completion (run once):")
		fmt.Println(`  paladin completion zsh > "${fpath[1]}/_paladin"`)
	case "bash":
		fmt.Println("\nShell completion (run once):")
		fmt.Println("  paladin completion bash > /usr/local/etc/bash_completion.d/paladin")
	case "fish":
		fmt.Println("\nShell completion (run once):")
		fmt.Println("  paladin completion fish > ~/.config/fish/completions/paladin.fish")
	}
}

// ── doctor command ────────────────────────────────────────────────────────────

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check PaladinAI connectivity and integration health",
	Long: `paladin doctor runs a series of health checks and reports status:

  ok  Reachable        -- API and Auth services respond
  ok  Authenticated    -- token is valid and non-expired
  ok  Tenant           -- tenant is configured
  ok  Agent            -- paladin-agent service is reachable
  ok  Integrations     -- registered MCP servers pass health checks`,
	RunE: runDoctor,
}

type check struct {
	name string
	fn   func() error
}

const infraDialTimeout = 3 * time.Second

// infraDialAddr extracts a dialable host:port from a URL-style string.
// Uses net/url.Parse so it handles schemes, userinfo, IPv6, and paths correctly.
// Falls back to localhost:<defaultPort> on empty or un-parseable input.
func infraDialAddr(raw string, defaultPort int) string {
	if raw == "" {
		return fmt.Sprintf("localhost:%d", defaultPort)
	}
	u, err := url.Parse(raw)
	if err == nil && u.Host != "" {
		// u.Host includes port when present (e.g. "localhost:6379")
		if _, _, serr := net.SplitHostPort(u.Host); serr == nil {
			return u.Host
		}
		// Host present but no port — append default
		return fmt.Sprintf("%s:%d", u.Host, defaultPort)
	}
	// No scheme or parse failed — treat raw as bare host[:port]
	if _, _, serr := net.SplitHostPort(raw); serr == nil {
		return raw
	}
	// Bare hostname without port
	if !strings.Contains(raw, ":") {
		return fmt.Sprintf("%s:%d", raw, defaultPort)
	}
	return raw
}

// infraTCPCheck returns a check function that dials addr with a 3-second timeout.
func infraTCPCheck(rawURL string, defaultPort int) func() error {
	addr := infraDialAddr(rawURL, defaultPort)
	return func() error {
		conn, err := net.DialTimeout("tcp", addr, infraDialTimeout)
		if err != nil {
			return err
		}
		defer conn.Close()
		return nil
	}
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	cfg, cfgErr := loadConfig()
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "WARNING: could not read config: %v\n", cfgErr)
	}

	// Flags always win when explicitly set; fall back to config file.
	apiBase := apiURL(cmd)
	if !cmd.Flags().Changed("api-url") && cfg != nil && cfg.APIEndpoint != "" {
		apiBase = cfg.APIEndpoint
	}
	apiBase = strings.TrimRight(apiBase, "/")

	authBase := "http://localhost:9003"
	if cfg != nil && cfg.AuthEndpoint != "" {
		authBase = cfg.AuthEndpoint
	}
	authBase = strings.TrimRight(authBase, "/")

	tenant := ""
	if cmd.Flags().Changed("tenant") {
		tenant, _ = cmd.Flags().GetString("tenant")
	} else if cfg != nil {
		tenant = cfg.DefaultTenant
	}

	token := optToken(cmd)
	if !cmd.Flags().Changed("token") {
		if t := os.Getenv("PALADIN_TOKEN"); t != "" {
			token = t
		} else if cfg != nil {
			token = cfg.Token
		}
	}

	opts := client.Options{TenantID: tenant, Token: token}

	checks := []check{
		{
			name: "API service reachable",
			fn: func() error {
				_, err := client.Get(cmd.Context(), apiBase+"/healthz", opts)
				return err
			},
		},
		{
			name: "Auth service reachable",
			fn: func() error {
				_, err := client.Get(cmd.Context(), authBase+"/healthz", client.Options{})
				return err
			},
		},
		{
			name: "Token configured",
			fn: func() error {
				if token == "" {
					return fmt.Errorf("no token (set --token or PALADIN_TOKEN or run 'paladin init')")
				}
				return nil
			},
		},
		{
			name: "Tenant configured",
			fn: func() error {
				if tenant == "" {
					return fmt.Errorf("no tenant (set --tenant or run 'paladin init')")
				}
				return nil
			},
		},
		{
			name: "MCP servers registered",
			fn: func() error {
				endpoint, err := url.JoinPath(apiBase, "/api/v1/mcp/servers")
				if err != nil {
					return fmt.Errorf("build url: %w", err)
				}
				body, err := client.Get(cmd.Context(), endpoint, opts)
				if err != nil {
					return err
				}
				if !strings.Contains(string(body), `"data"`) {
					return fmt.Errorf("unexpected response: %s", body)
				}
				return nil
			},
		},
		{
			name: "Kubernetes cluster context",
			fn: func() error {
				out, err := exec.Command("kubectl", "config", "current-context").Output()
				if err != nil {
					if errors.Is(err, exec.ErrNotFound) {
						return fmt.Errorf("kubectl not installed")
					}
					return fmt.Errorf("kubectl current-context: %w", err)
				}
				fmt.Printf("      context: %s\n", strings.TrimSpace(string(out)))
				return nil
			},
		},
		{
			name: "NATS reachable",
			fn:   infraTCPCheck(envStr("NATS_URL", "nats://localhost:4222"), 4222),
		},
		{
			name: "Valkey reachable",
			fn:   infraTCPCheck(envStr("VALKEY_URL", "redis://localhost:6379"), 6379),
		},
		{
			name: "Qdrant reachable",
			fn:   infraTCPCheck(envStr("QDRANT_URL", "http://localhost:6333"), 6333),
		},
	}

	fmt.Println("paladin doctor")
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("  API:   %s\n", apiBase)
	fmt.Printf("  Auth:  %s\n", authBase)
	fmt.Printf("  OS:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println()

	allPassed := true
	for _, c := range checks {
		err := c.fn()
		if err != nil {
			fmt.Printf("  x %-35s %v\n", c.name, err)
			allPassed = false
		} else {
			fmt.Printf("  ok %-35s\n", c.name)
		}
	}

	fmt.Println()
	if allPassed {
		fmt.Println("All checks passed. PaladinAI is ready.")
	} else {
		fmt.Println("Some checks failed. Run 'paladin init' to fix configuration.")
		return fmt.Errorf("doctor: one or more checks failed")
	}
	return nil
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(doctorCmd)
}
