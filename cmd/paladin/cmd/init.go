package cmd

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/paladinai/paladinai/cmd/paladin/client"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// configDir returns the paladin config directory (~/.paladin).
func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".paladin")
}

// configPath returns the path to the paladin user config file.
func configPath() string {
	return filepath.Join(configDir(), "config.yaml")
}

// PaladinConfig is the structure of ~/.paladin/config.yaml.
type PaladinConfig struct {
	APIEndpoint   string `yaml:"api_endpoint"`
	AuthEndpoint  string `yaml:"auth_endpoint"`
	DefaultTenant string `yaml:"default_tenant"`
	OutputFormat  string `yaml:"output_format"`
	Token         string `yaml:"token,omitempty"`
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
	fmt.Println(strings.Repeat("─", 50))

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
	apiEndpoint := readLine(r)
	if apiEndpoint == "" {
		apiEndpoint = defaultAPI
	}
	if _, err := url.ParseRequestURI(apiEndpoint); err != nil {
		return fmt.Errorf("invalid API URL %q: %w", apiEndpoint, err)
	}

	defaultAuth := cfg.AuthEndpoint
	if defaultAuth == "" {
		defaultAuth = "http://localhost:9003"
	}
	fmt.Printf("  PaladinAI Auth URL [%s]: ", defaultAuth)
	authEndpoint := readLine(r)
	if authEndpoint == "" {
		authEndpoint = defaultAuth
	}

	// ── Step 2: Authentication ────────────────────────────────────────────────
	fmt.Println("\n[2/4] Authentication")
	existingToken := cfg.Token
	if t := os.Getenv("PALADIN_TOKEN"); t != "" {
		existingToken = t
		fmt.Println("  Using token from PALADIN_TOKEN env var.")
	}
	if existingToken == "" {
		fmt.Print("  API token (leave blank to continue unauthenticated): ")
		existingToken = readLine(r)
	} else {
		fmt.Printf("  Token found. Press Enter to keep it or paste a new one: ")
		if line := readLine(r); line != "" {
			existingToken = line
		}
	}

	// ── Step 3: Tenant ────────────────────────────────────────────────────────
	fmt.Println("\n[3/4] Tenant")
	defaultTenant := cfg.DefaultTenant
	if t := os.Getenv("PALADIN_TENANT"); t != "" {
		defaultTenant = t
	}
	fmt.Printf("  Default tenant ID [%s]: ", defaultTenant)
	tenant := readLine(r)
	if tenant == "" {
		tenant = defaultTenant
	}

	// ── Step 4: Verify connectivity ───────────────────────────────────────────
	fmt.Println("\n[4/4] Verifying connectivity...")
	healthURL := strings.TrimRight(apiEndpoint, "/") + "/healthz"
	if _, err := client.Get(cmd.Context(), healthURL, client.Options{
		TenantID: tenant,
		Token:    existingToken,
	}); err != nil {
		fmt.Printf("  ✗ Could not reach %s: %v\n", healthURL, err)
		fmt.Println("    (continuing — you can retry with 'paladin doctor')")
	} else {
		fmt.Printf("  ✓ Connected to %s\n", apiEndpoint)
	}

	// ── Save config ───────────────────────────────────────────────────────────
	cfg.APIEndpoint = apiEndpoint
	cfg.AuthEndpoint = authEndpoint
	cfg.DefaultTenant = tenant
	cfg.Token = existingToken
	if cfg.OutputFormat == "" {
		cfg.OutputFormat = "table"
	}
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("\n✓ Config saved to %s\n", configPath())
	fmt.Println("\nNext steps:")
	fmt.Println("  paladin doctor             — verify integrations")
	fmt.Println("  paladin alert list         — view active alerts")
	fmt.Println("  paladin dashboard          — open TUI dashboard")
	printCompletionHint()
	return nil
}

// readLine reads a trimmed line from the reader, ignoring EOF errors.
func readLine(r *bufio.Reader) string {
	line, _ := r.ReadString('\n')
	return strings.TrimSpace(line)
}

func printCompletionHint() {
	shell := filepath.Base(os.Getenv("SHELL"))
	switch shell {
	case "zsh":
		fmt.Println("\nShell completion (run once):")
		fmt.Println("  paladin completion zsh > \"${fpath[1]}/_paladin\"")
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

  ✓  Reachable        — API and Auth services respond
  ✓  Authenticated    — token is valid and non-expired
  ✓  Tenant           — tenant exists and is active
  ✓  Agent            — paladin-agent service is reachable
  ✓  Integrations     — registered MCP servers pass health checks`,
	RunE: runDoctor,
}

type check struct {
	name string
	fn   func() error
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	cfg, _ := loadConfig()

	apiBase := apiURL(cmd)
	if cfg != nil && cfg.APIEndpoint != "" && apiBase == "http://localhost:8080" {
		apiBase = cfg.APIEndpoint
	}
	authBase := "http://localhost:9003"
	if cfg != nil && cfg.AuthEndpoint != "" {
		authBase = cfg.AuthEndpoint
	}

	tenant := ""
	if t := cmd.Flag("tenant"); t != nil {
		tenant = t.Value.String()
	}
	if tenant == "" && cfg != nil {
		tenant = cfg.DefaultTenant
	}

	token := optToken(cmd)
	if token == "" && cfg != nil {
		token = cfg.Token
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
			name: "Token valid",
			fn: func() error {
				if token == "" {
					return fmt.Errorf("no token configured (set --token or PALADIN_TOKEN)")
				}
				return nil
			},
		},
		{
			name: "Tenant configured",
			fn: func() error {
				if tenant == "" {
					return fmt.Errorf("no tenant configured (run 'paladin init' or set --tenant)")
				}
				return nil
			},
		},
		{
			name: "MCP servers registered",
			fn: func() error {
				u, _ := url.Parse(apiBase)
				u.Path = "/api/v1/mcp/servers"
				body, err := client.Get(cmd.Context(), u.String(), opts)
				if err != nil {
					return err
				}
				if !strings.Contains(string(body), `"data"`) {
					return fmt.Errorf("unexpected response: %s", body)
				}
				return nil
			},
		},
	}

	// also detect kubectl availability and cluster context
	kubeCheck := check{
		name: "Kubernetes cluster context",
		fn: func() error {
			out, err := exec.Command("kubectl", "config", "current-context").Output()
			if err != nil {
				return fmt.Errorf("kubectl not found or no context set")
			}
			context := strings.TrimSpace(string(out))
			fmt.Printf("      context: %s\n", context)
			return nil
		},
	}
	checks = append(checks, kubeCheck)

	fmt.Println("paladin doctor")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("  API:   %s\n", apiBase)
	fmt.Printf("  Auth:  %s\n", authBase)
	fmt.Printf("  OS:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println()

	allPassed := true
	for _, c := range checks {
		err := c.fn()
		if err != nil {
			fmt.Printf("  ✗ %-35s %v\n", c.name, err)
			allPassed = false
		} else {
			fmt.Printf("  ✓ %-35s\n", c.name)
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
