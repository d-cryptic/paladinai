package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
	"github.com/paladinai/paladinai/cmd/paladin/client"
	"github.com/paladinai/paladinai/cmd/paladin/tui"
	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/internal/projectconfig"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type deploymentMode string

const (
	deployModeAuto   deploymentMode = "auto"
	deployModeHosted deploymentMode = "hosted"
	deployModeK8s    deploymentMode = "k8s"
	deployModeDocker deploymentMode = "docker"
	deployModeBinary deploymentMode = "binary"
)

var (
	initLookPath       = exec.LookPath
	initKubectlContext = hasKubectlContext
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
// Token is retained only to read legacy plaintext configs; new tokens are stored
// in the OS keychain and are never written back to this file.
type PaladinConfig struct {
	APIEndpoint     string    `yaml:"api_endpoint"`
	AuthEndpoint    string    `yaml:"auth_endpoint"`
	DefaultTenant   string    `yaml:"default_tenant"`
	OutputFormat    string    `yaml:"output_format"`
	LastUpdateCheck time.Time `yaml:"last_update_check,omitempty"`
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
	toWrite := *c
	toWrite.Token = ""
	data, err := yaml.Marshal(&toWrite)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0o600)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Interactive setup wizard — get PaladinAI running in minutes",
	Long: `paladin init guides you through connecting PaladinAI to your infrastructure.

Steps (TUI mode, --tui):
  1. Environment detection — auto-detect Prometheus, Grafana, Datadog, PagerDuty, etc.
  2. Authentication       — provide or generate an API token
  3. Tier selection       — Pool / Bridge / Silo deployment tier
  4. Integrations         — enable detected and manual integrations
  5. Deploy               — apply configuration
  6. Verify               — inline health check

Steps (default / non-TTY mode):
  1. API endpoint    — point the CLI at your PaladinAI instance
  2. Authentication  — provide or generate an API token
  3. Tenant          — select or create your tenant
  4. Verify          — confirm connectivity and first triage is ready

After init, run 'paladin doctor' to verify all integrations are healthy.`,
	RunE: runInit,
}

func runInit(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	useTUI, _ := cmd.Flags().GetBool("tui")
	// Auto-enable TUI when stdout is an interactive terminal.
	if !useTUI && isatty.IsTerminal(os.Stdout.Fd()) {
		useTUI = true
	}

	defaultAPI := cfg.APIEndpoint
	if defaultAPI == "" {
		defaultAPI = defaultAPIURL
	}
	defaultAuth := cfg.AuthEndpoint
	if defaultAuth == "" {
		defaultAuth = "http://localhost:9003"
	}
	defaultTenant := cfg.DefaultTenant
	if t := os.Getenv("PALADIN_TENANT"); t != "" {
		defaultTenant = t
	}
	tokenFromEnv := os.Getenv("PALADIN_TOKEN") != ""
	existingToken := ""
	if !tokenFromEnv {
		existingToken, err = loadStoredToken(cfg, defaultAuth)
		if err != nil {
			return err
		}
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	tier, _ := cmd.Flags().GetString("tier")
	deployMode, _ := cmd.Flags().GetString("deploy-mode")
	if dryRun {
		return runInitDryRun(cmd, defaultAPI, defaultAuth, defaultTenant, existingToken, tier, deployMode)
	}

	if useTUI {
		return runInitTUI(cmd, cfg, defaultAPI, defaultAuth, defaultTenant, existingToken, tokenFromEnv)
	}
	return runInitPrompt(cmd, cfg, defaultAPI, defaultAuth, defaultTenant, existingToken, tokenFromEnv)
}

func runInitDryRun(cmd *cobra.Command, apiEndpoint, authEndpoint, tenant, token, tier, deployMode string) error {
	apiEndpoint = strings.TrimRight(apiEndpoint, "/")
	authEndpoint = strings.TrimRight(authEndpoint, "/")
	if _, err := url.ParseRequestURI(apiEndpoint); err != nil {
		return fmt.Errorf("invalid API URL %q: %w", apiEndpoint, err)
	}
	if _, err := url.ParseRequestURI(authEndpoint); err != nil {
		return fmt.Errorf("invalid auth URL %q: %w", authEndpoint, err)
	}
	if err := validateInitInputs(tenant, tier); err != nil {
		return err
	}

	activeToken := token
	if t := os.Getenv("PALADIN_TOKEN"); t != "" {
		activeToken = t
	}
	resolvedMode, err := resolveDeploymentMode(deployMode, tier)
	if err != nil {
		return err
	}

	fmt.Println("PaladinAI init dry-run")
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("  API:        %s\n", apiEndpoint)
	fmt.Printf("  Auth:       %s\n", authEndpoint)
	fmt.Printf("  Tenant:     %s\n", tenant)
	fmt.Printf("  Tier:       %s\n", normalizeTier(tier))
	fmt.Printf("  Deployment: %s\n", resolvedMode)
	fmt.Println("  Writes: disabled")

	if _, err := client.Get(cmd.Context(), apiEndpoint+"/readyz", client.Options{
		TenantID: tenant,
		Token:    activeToken,
	}); err != nil {
		fmt.Printf("  x API readiness check failed: %v\n", err)
		fmt.Println("Dry-run completed without writing config.")
		return nil
	}
	fmt.Println("  ok API readiness check passed")
	fmt.Println("Dry-run completed without writing config.")
	return nil
}

// runInitTUI launches the Bubble Tea 6-step wizard.
func runInitTUI(cmd *cobra.Command, cfg *PaladinConfig, defaultAPI, defaultAuth, defaultTenant, existingToken string, tokenFromEnv bool) error {
	wizard := tui.NewWizardModel(defaultAPI, defaultAuth, defaultTenant, existingToken)
	prog := tea.NewProgram(wizard, tea.WithAltScreen())
	finalModel, err := prog.Run()
	if err != nil {
		return fmt.Errorf("wizard: %w", err)
	}
	wm, ok := finalModel.(tui.WizardModel)
	if !ok {
		return fmt.Errorf("unexpected model type from wizard")
	}
	if wm.Aborted() {
		return fmt.Errorf("setup aborted")
	}
	res := wm.Result()
	return applyAndSaveProject(cmd, cfg, res.APIEndpoint, res.AuthEndpoint, res.Tenant, res.Token, tokenFromEnv, res.Tier, res.Integrations)
}

// runInitPrompt runs the classic line-prompt fallback (non-TTY / piped).
func runInitPrompt(cmd *cobra.Command, cfg *PaladinConfig, defaultAPI, defaultAuth, defaultTenant, existingToken string, tokenFromEnv bool) error {
	r := bufio.NewReader(os.Stdin)

	fmt.Println("Welcome to PaladinAI — interactive setup wizard")
	fmt.Println(strings.Repeat("-", 50))

	// ── Step 1: API endpoint ─────────────────────────────────────────────────
	fmt.Println("\n[1/4] API Endpoint")
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
	fmt.Println("  Tokens are stored in the OS keychain. In CI, set PALADIN_TOKEN.")

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
	fmt.Printf("  Default tenant ID [%s]: ", defaultTenant)
	tenant, err := readLine(r)
	if err != nil {
		return fmt.Errorf("read tenant: %w", err)
	}
	if tenant == "" {
		tenant = defaultTenant
	}

	return applyAndSave(cmd, cfg, apiEndpoint, authEndpoint, tenant, existingToken, tokenFromEnv)
}

// applyAndSave verifies connectivity, then writes config to disk.
func applyAndSave(cmd *cobra.Command, cfg *PaladinConfig, apiEndpoint, authEndpoint, tenant, token string, tokenFromEnv bool) error {
	return applyAndSaveProject(cmd, cfg, apiEndpoint, authEndpoint, tenant, token, tokenFromEnv, "pool", nil)
}

func applyAndSaveProject(cmd *cobra.Command, cfg *PaladinConfig, apiEndpoint, authEndpoint, tenant, token string, tokenFromEnv bool, tier string, integrations []string) error {
	if err := validateInitInputs(tenant, tier); err != nil {
		return err
	}
	activeToken := token
	if t := os.Getenv("PALADIN_TOKEN"); t != "" {
		activeToken = t
	}

	fmt.Println("\nVerifying connectivity...")
	if _, err := client.Get(cmd.Context(), apiEndpoint+"/readyz", client.Options{
		TenantID: tenant,
		Token:    activeToken,
	}); err != nil {
		fmt.Printf("  x Could not reach %s: %v\n", apiEndpoint, err)
		fmt.Println("    (continuing — retry with 'paladin doctor')")
	} else {
		fmt.Printf("  ok Connected to %s\n", apiEndpoint)
	}

	cfg.APIEndpoint = apiEndpoint
	cfg.AuthEndpoint = authEndpoint
	cfg.DefaultTenant = tenant
	cfg.Token = ""
	if !tokenFromEnv {
		if err := saveStoredToken(authEndpoint, token); err != nil {
			return err
		}
	}
	if cfg.OutputFormat == "" {
		cfg.OutputFormat = "table"
	}
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	if err := writeProjectConfig("paladin.yaml", tenant, tier, integrations); err != nil {
		return err
	}

	fmt.Printf("\nConfig saved to %s\n", configPath())
	fmt.Println("Project config saved to paladin.yaml")
	fmt.Println("\nNext steps:")
	fmt.Println("  paladin doctor             -- verify integrations")
	fmt.Println("  paladin alert list         -- view active alerts")
	fmt.Println("  paladin dashboard          -- open TUI dashboard")
	printCompletionHint()
	return nil
}

func writeProjectConfig(path, tenant, tier string, enabled []string) error {
	cfg := projectconfig.Default(tenant, tier, "")
	enabledSet := make(map[string]struct{}, len(enabled))
	for _, name := range enabled {
		name = strings.TrimSpace(name)
		if name != "" {
			enabledSet[name] = struct{}{}
		}
	}
	for i := range cfg.Spec.Integrations {
		_, cfg.Spec.Integrations[i].Enabled = enabledSet[cfg.Spec.Integrations[i].Name]
		delete(enabledSet, cfg.Spec.Integrations[i].Name)
	}
	customIntegrations := make([]string, 0, len(enabledSet))
	for name := range enabledSet {
		customIntegrations = append(customIntegrations, name)
	}
	sort.Strings(customIntegrations)
	for _, name := range customIntegrations {
		cfg.Spec.Integrations = append(cfg.Spec.Integrations, projectconfig.Integration{
			Name:    name,
			Version: "latest",
			Enabled: true,
		})
	}
	return projectconfig.WriteFile(path, cfg)
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

func resolveDeploymentMode(rawMode, tier string) (deploymentMode, error) {
	mode := deploymentMode(strings.ToLower(strings.TrimSpace(rawMode)))
	if mode == "" {
		mode = deployModeAuto
	}
	switch mode {
	case deployModeAuto:
		return detectDeploymentMode(tier), nil
	case deployModeHosted, deployModeK8s, deployModeDocker, deployModeBinary:
		return mode, nil
	default:
		return "", fmt.Errorf("unsupported deployment mode %q — use auto, hosted, k8s, docker, or binary", rawMode)
	}
}

func validateInitInputs(tenant, tier string) error {
	if err := alert.ValidateTenantID(tenant); err != nil {
		return fmt.Errorf("invalid tenant ID %q: %w", tenant, err)
	}
	if err := validateTier(tier); err != nil {
		return err
	}
	return nil
}

func validateTier(tier string) error {
	switch normalizeTier(tier) {
	case "pool", "bridge", "silo":
		return nil
	default:
		return fmt.Errorf("unsupported deployment tier %q — use pool, bridge, or silo", tier)
	}
}

func detectDeploymentMode(tier string) deploymentMode {
	tier = normalizeTier(tier)
	if tier == "pool" || tier == "bridge" {
		return deployModeHosted
	}
	if initKubectlContext() == nil && hasExecutable("helm") {
		return deployModeK8s
	}
	if hasExecutable("docker") && hasExecutable("docker-compose") {
		return deployModeDocker
	}
	return deployModeBinary
}

func normalizeTier(tier string) string {
	tier = strings.ToLower(strings.TrimSpace(tier))
	if tier == "" {
		return "pool"
	}
	return tier
}

func hasExecutable(name string) bool {
	_, err := initLookPath(name)
	return err == nil
}

func hasKubectlContext() error {
	out, err := exec.Command("kubectl", "config", "current-context").Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("kubectl not installed")
		}
		return fmt.Errorf("kubectl current-context: %w", err)
	}
	if strings.TrimSpace(string(out)) == "" {
		return fmt.Errorf("kubectl current-context is empty")
	}
	return nil
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(doctorCmd)
	initCmd.Flags().Bool("tui", false, "Force Bubble Tea interactive wizard (auto-detected when stdout is a TTY)")
	initCmd.Flags().Bool("dry-run", false, "Validate init defaults without writing config files")
	initCmd.Flags().String("tier", "pool", "Deployment tier: pool, bridge, or silo")
	initCmd.Flags().String("deploy-mode", "auto", "Deployment mode: auto, hosted, k8s, docker, or binary")
	doctorCmd.Flags().Bool("json", false, "Emit machine-readable JSON output (for CI)")
	doctorCmd.Flags().Bool("quiet", false, "Suppress output; communicate status via exit code only")
}
