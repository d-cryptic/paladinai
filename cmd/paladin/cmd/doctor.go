package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/paladinai/paladinai/cmd/paladin/client"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor [integration]",
	Short: "Check PaladinAI connectivity and integration health",
	Long: `paladin doctor runs a series of health checks and reports status:

  ok  Reachable        -- API and Auth services respond
  ok  Authenticated    -- token is valid and non-expired
  ok  Tenant           -- tenant is configured
  ok  Agent            -- paladin-agent service is reachable
  ok  Memory           -- paladin-memory readiness probe responds
  ok  Integrations     -- registered MCP servers pass health checks

Flags:
  --json   Emit machine-readable JSON (for CI pipelines)
  --quiet  Print nothing; exit code only (0=all pass, 1=some fail)`,
	Args: cobra.MaximumNArgs(1),
	Example: `  # run full verification
  paladin doctor

  # validate only one integration (name or server ID)
  paladin doctor prometheus`,
	RunE: runDoctor,
}

type check struct {
	name string
	fn   func() error
}

// CheckResult holds the outcome of a single doctor check.
// Used for --json output.
type CheckResult struct {
	Name        string `json:"name"`
	Passed      bool   `json:"passed"`
	Status      string `json:"status"`
	DurationMS  int64  `json:"duration_ms"`
	Detail      string `json:"detail,omitempty"`
	Error       string `json:"error,omitempty"`
	Remediation string `json:"remediation,omitempty"`
}

// DoctorReport is the top-level JSON emitted by paladin doctor --json.
type DoctorReport struct {
	Passed    bool          `json:"passed"`
	Overall   string        `json:"overall"`
	Timestamp string        `json:"timestamp"`
	Checks    []CheckResult `json:"checks"`
}

const infraDialTimeout = 3 * time.Second

func runDoctor(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	quietMode, _ := cmd.Flags().GetBool("quiet")
	if isCIMode(cmd) && !jsonMode && !quietMode {
		quietMode = true
	}
	targetIntegration := ""
	if len(args) > 0 {
		targetIntegration = strings.TrimSpace(args[0])
	}

	cfg, cfgErr := loadConfig()
	if cfgErr != nil && !quietMode && !jsonMode {
		fmt.Fprintf(os.Stderr, "WARNING: could not read config: %v\n", cfgErr)
	}

	apiBase := apiURL(cmd)
	if !cmd.Flags().Changed("api-url") && cfg != nil && cfg.APIEndpoint != "" {
		apiBase = cfg.APIEndpoint
	}
	apiBase = strings.TrimRight(apiBase, "/")

	authBase := envStr("PALADIN_AUTH_URL", "http://localhost:9003")
	if cfg != nil && cfg.AuthEndpoint != "" {
		authBase = cfg.AuthEndpoint
	}
	authBase = strings.TrimRight(authBase, "/")

	tenant := doctorTenant(cmd, cfg)
	token, err := doctorToken(cmd, cfg, authBase)
	if err != nil {
		return err
	}
	opts := doctorClientOptions(tenant, token)

	checks := doctorChecks(cmd, doctorCheckOptions{
		apiBase:           apiBase,
		authBase:          authBase,
		tenant:            tenant,
		token:             token,
		opts:              opts,
		targetIntegration: targetIntegration,
		jsonMode:          jsonMode,
		quietMode:         quietMode,
	})
	results, allPassed := runDoctorChecks(checks)

	if err := writeDoctorReport(doctorReportOptions{
		apiBase:   apiBase,
		authBase:  authBase,
		jsonMode:  jsonMode,
		quietMode: quietMode,
		results:   results,
		allPassed: allPassed,
	}); err != nil {
		return err
	}

	if !allPassed {
		if quietMode || jsonMode {
			cmd.SilenceUsage = true
			cmd.Root().SilenceErrors = true
		}
		return fmt.Errorf("doctor: one or more checks failed")
	}
	return nil
}

type doctorCheckOptions struct {
	apiBase           string
	authBase          string
	tenant            string
	token             string
	opts              client.Options
	targetIntegration string
	jsonMode          bool
	quietMode         bool
}

func doctorChecks(cmd *cobra.Command, opts doctorCheckOptions) []check {
	checks := []check{
		{
			name: "API service ready",
			fn: func() error {
				_, err := client.Get(cmd.Context(), opts.apiBase+"/readyz", opts.opts)
				return err
			},
		},
		{
			name: "Auth service ready",
			fn: func() error {
				_, err := client.Get(cmd.Context(), opts.authBase+"/readyz", client.Options{})
				return err
			},
		},
		{
			name: "Token configured",
			fn: func() error {
				if opts.token == "" {
					return fmt.Errorf("no token (set --token or PALADIN_TOKEN or run 'paladin init')")
				}
				return nil
			},
		},
		{
			name: "Tenant configured",
			fn: func() error {
				if opts.tenant == "" {
					return fmt.Errorf("no tenant (set --tenant or run 'paladin init')")
				}
				return nil
			},
		},
	}

	if opts.targetIntegration != "" {
		return append(checks, doctorIntegrationCheck(cmd, opts.apiBase, opts.opts, opts.targetIntegration))
	}
	return append(checks, doctorFullChecks(cmd, opts)...)
}

func doctorFullChecks(cmd *cobra.Command, opts doctorCheckOptions) []check {
	return []check{
		{
			name: "MCP servers registered",
			fn: func() error {
				body, err := getMCPServers(cmd, opts.apiBase, opts.opts)
				if err != nil {
					return err
				}
				_, err = parseMCPServers(body)
				return err
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
				if !opts.quietMode && !opts.jsonMode {
					fmt.Printf("      context: %s\n", strings.TrimSpace(string(out)))
				}
				return nil
			},
		},
		{name: "NATS reachable", fn: infraTCPCheck(envStr("NATS_URL", "nats://localhost:4222"), 4222)},
		{name: "Valkey reachable", fn: infraTCPCheck(envStr("VALKEY_URL", "redis://localhost:6379"), 6379)},
		{name: "Qdrant reachable", fn: infraTCPCheck(envStr("QDRANT_URL", "http://localhost:6333"), 6333)},
		serviceReadyCheck(cmd, "paladin-ingest ready", envStr("PALADIN_INGEST_PORT", "9001"), 9001),
		serviceReadyCheck(cmd, "paladin-hub ready", envStr("PALADIN_HUB_URL", envStr("PALADIN_HUB_PORT", "8082")), 8082),
		serviceReadyCheck(cmd, "paladin-memory ready", envStr("MEMORY_HTTP_ADDR", ":9011"), 9011),
		serviceReadyCheck(cmd, "paladin-agent ready", envStr("PALADIN_AGENT_PORT", "9006"), 9006),
		serviceReadyCheck(cmd, "paladin-ws ready", envStr("PALADIN_WS_PORT", "9007"), 9007),
		serviceReadyCheck(cmd, "paladin-comms ready", envStr("PALADIN_COMMS_PORT", "9009"), 9009),
		serviceReadyCheck(cmd, "paladin-orchestrator ready", envStr("PALADIN_ORCHESTRATOR_PORT", "9008"), 9008),
	}
}

func doctorIntegrationCheck(cmd *cobra.Command, apiBase string, opts client.Options, targetIntegration string) check {
	return check{
		name: fmt.Sprintf("Integration check: %s", targetIntegration),
		fn: func() error {
			body, err := getMCPServers(cmd, apiBase, opts)
			if err != nil {
				return err
			}
			servers, err := parseMCPServers(body)
			if err != nil {
				return err
			}
			return containsIntegration(servers, targetIntegration)
		},
	}
}

func serviceReadyCheck(cmd *cobra.Command, name, rawAddr string, defaultPort int) check {
	return check{
		name: name,
		fn: func() error {
			base := localHTTPBase(rawAddr, defaultPort)
			_, err := client.Get(cmd.Context(), base+"/readyz", client.Options{})
			return err
		},
	}
}

func getMCPServers(cmd *cobra.Command, apiBase string, opts client.Options) ([]byte, error) {
	endpoint, err := url.JoinPath(apiBase, "/api/v1/mcp/servers")
	if err != nil {
		return nil, fmt.Errorf("build url: %w", err)
	}
	return client.Get(cmd.Context(), endpoint, opts)
}

func runDoctorChecks(checks []check) ([]CheckResult, bool) {
	results := make([]CheckResult, 0, len(checks))
	allPassed := true
	for _, c := range checks {
		start := time.Now()
		err := c.fn()
		result := CheckResult{
			Name:       c.name,
			Passed:     err == nil,
			Status:     "pass",
			DurationMS: time.Since(start).Milliseconds(),
		}
		if err != nil {
			result.Status = "fail"
			result.Detail = err.Error()
			result.Error = err.Error()
			result.Remediation = doctorRemediation(c.name)
			allPassed = false
		}
		results = append(results, result)
	}
	return results, allPassed
}

type doctorReportOptions struct {
	apiBase   string
	authBase  string
	jsonMode  bool
	quietMode bool
	results   []CheckResult
	allPassed bool
}

func writeDoctorReport(opts doctorReportOptions) error {
	switch {
	case opts.jsonMode:
		overall := "pass"
		if !opts.allPassed {
			overall = "fail"
		}
		report := DoctorReport{
			Passed:    opts.allPassed,
			Overall:   overall,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Checks:    opts.results,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return fmt.Errorf("encode json: %w", err)
		}
	case opts.quietMode:
	default:
		fmt.Println("paladin doctor")
		fmt.Println(strings.Repeat("-", 50))
		fmt.Printf("  API:   %s\n", opts.apiBase)
		fmt.Printf("  Auth:  %s\n", opts.authBase)
		fmt.Printf("  OS:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Println()
		for _, result := range opts.results {
			if result.Passed {
				fmt.Printf("  ok %-35s\n", result.Name)
			} else {
				fmt.Printf("  x %-35s %s\n", result.Name, result.Error)
			}
		}
		fmt.Println()
		if opts.allPassed {
			fmt.Println("All checks passed. PaladinAI is ready.")
		} else {
			fmt.Println("Some checks failed. Run 'paladin init' to fix configuration.")
		}
	}
	return nil
}

func infraDialAddr(raw string, defaultPort int) string {
	if raw == "" {
		return fmt.Sprintf("localhost:%d", defaultPort)
	}
	u, err := url.Parse(raw)
	if err == nil && u.Host != "" {
		if _, _, serr := net.SplitHostPort(u.Host); serr == nil {
			return u.Host
		}
		return fmt.Sprintf("%s:%d", u.Host, defaultPort)
	}
	if _, _, serr := net.SplitHostPort(raw); serr == nil {
		return raw
	}
	if !strings.Contains(raw, ":") {
		return fmt.Sprintf("%s:%d", raw, defaultPort)
	}
	return raw
}

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

type mcpServerRegistration struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Healthy  bool   `json:"healthy"`
	Endpoint string `json:"endpoint"`
}

type mcpServerList struct {
	Data []mcpServerRegistration `json:"data"`
}

func parseMCPServers(body []byte) (mcpServerList, error) {
	var response mcpServerList
	if err := json.Unmarshal(body, &response); err != nil {
		return mcpServerList{}, fmt.Errorf("invalid MCP response: %w", err)
	}
	return response, nil
}

func containsIntegration(servers mcpServerList, integration string) error {
	target := strings.ToLower(strings.TrimSpace(integration))
	if target == "" {
		return fmt.Errorf("integration argument is required")
	}
	for _, srv := range servers.Data {
		if strings.EqualFold(strings.TrimSpace(srv.ID), target) || strings.EqualFold(strings.TrimSpace(srv.Name), target) {
			if srv.Healthy {
				return nil
			}
			return fmt.Errorf("integration %q is not healthy", integration)
		}
	}
	return fmt.Errorf("integration %q is not registered", integration)
}

func localHTTPBase(raw string, defaultPort int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = fmt.Sprintf(":%d", defaultPort)
	}
	if isPort(raw) {
		return "http://localhost:" + raw
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return strings.TrimRight(raw, "/")
	}
	if strings.HasPrefix(raw, ":") {
		return "http://localhost" + raw
	}
	if _, _, err := net.SplitHostPort(raw); err == nil {
		return "http://" + raw
	}
	if !strings.Contains(raw, ":") {
		return fmt.Sprintf("http://%s:%d", raw, defaultPort)
	}
	return "http://" + raw
}

func isPort(raw string) bool {
	if raw == "" {
		return false
	}
	for _, r := range raw {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func doctorRemediation(name string) string {
	switch {
	case name == "API service ready":
		return "Check PALADIN_API_URL or run `paladin init --dry-run` with the correct --api-url."
	case name == "Auth service ready":
		return "Check PALADIN_AUTH_URL or run `paladin auth login` after the auth service is reachable."
	case name == "Token configured":
		return "Run `paladin auth login` or set PALADIN_TOKEN."
	case name == "Tenant configured":
		return "Set --tenant, PALADIN_TENANT, or rerun `paladin init`."
	case name == "MCP servers registered":
		return "Run `paladin integrations status` and confirm paladin-hub can list registered MCP servers."
	case name == "Kubernetes cluster context":
		return "Run `kubectl config current-context` and set kubeconfig before rerunning `paladin doctor`."
	case name == "NATS reachable":
		return "Start NATS with `make up` or set NATS_URL to the reachable JetStream endpoint."
	case name == "Valkey reachable":
		return "Start Valkey with `make up` or set VALKEY_URL to the reachable Redis-compatible endpoint."
	case name == "Qdrant reachable":
		return "Start Qdrant with `make up` or set QDRANT_URL to the reachable vector database endpoint."
	case name == "paladin-ingest ready":
		return "Start paladin-ingest or set PALADIN_INGEST_PORT to its HTTP readiness endpoint."
	case name == "paladin-hub ready":
		return "Start paladin-hub or set PALADIN_HUB_URL to its HTTP readiness endpoint."
	case name == "paladin-memory ready":
		return "Start paladin-memory or set MEMORY_HTTP_ADDR to its HTTP readiness endpoint."
	case name == "paladin-agent ready":
		return "Start paladin-agent or set PALADIN_AGENT_PORT to its HTTP readiness endpoint."
	case name == "paladin-ws ready":
		return "Start paladin-ws or set PALADIN_WS_PORT to its HTTP readiness endpoint."
	case name == "paladin-comms ready":
		return "Start paladin-comms or set PALADIN_COMMS_PORT to its HTTP readiness endpoint."
	case name == "paladin-orchestrator ready":
		return "Start paladin-orchestrator or set PALADIN_ORCHESTRATOR_PORT to its HTTP readiness endpoint."
	case strings.HasPrefix(name, "Integration check: "):
		integration := strings.TrimSpace(strings.TrimPrefix(name, "Integration check: "))
		return fmt.Sprintf("Run `paladin integrations status` or `paladin integrations enable %s` before rerunning `paladin doctor %s`.", integration, integration)
	default:
		return "Rerun `paladin doctor --json` after checking the failed dependency."
	}
}

func doctorClientOptions(tenant, token string) client.Options {
	if token != "" {
		return client.Options{Token: token}
	}
	return client.Options{TenantID: tenant}
}

func doctorTenant(cmd *cobra.Command, cfg *PaladinConfig) string {
	if cmd.Flags().Changed("tenant") {
		tenant, _ := cmd.Flags().GetString("tenant")
		return strings.TrimSpace(tenant)
	}
	if f := cmd.Flag("tenant"); f != nil && strings.TrimSpace(f.Value.String()) != "" {
		return strings.TrimSpace(f.Value.String())
	}
	if cfg != nil {
		return strings.TrimSpace(cfg.DefaultTenant)
	}
	return ""
}

func doctorToken(cmd *cobra.Command, cfg *PaladinConfig, authBase string) (string, error) {
	token := optToken(cmd)
	if cmd.Flags().Changed("token") {
		return token, nil
	}
	if envToken := os.Getenv("PALADIN_TOKEN"); envToken != "" {
		return envToken, nil
	}
	if cfg == nil {
		return token, nil
	}
	stored, err := loadStoredToken(cfg, authBase)
	if err != nil {
		return "", err
	}
	return stored, nil
}
