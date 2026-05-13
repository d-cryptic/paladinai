package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/paladinai/paladinai/cmd/paladin/client"
	"github.com/paladinai/paladinai/internal/projectconfig"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage paladin.yaml project configuration",
}

// validateConfig returns human-readable error strings; empty slice means valid.
func validateConfig(cfg projectconfig.Config) []string {
	var errs []string
	if cfg.APIVersion != "paladin.io/v2" {
		errs = append(errs, fmt.Sprintf("apiVersion: expected paladin.io/v2, got %q", cfg.APIVersion))
	}
	if cfg.Kind != "Config" {
		errs = append(errs, fmt.Sprintf("kind: expected Config, got %q", cfg.Kind))
	}
	if cfg.Metadata.Tenant == "" {
		errs = append(errs, "metadata.tenant: must not be empty")
	}
	if strings.ContainsAny(cfg.Metadata.Tenant, "\r\n\x00") {
		errs = append(errs, "metadata.tenant: contains invalid characters")
	}
	validTiers := map[string]bool{"pool": true, "bridge": true, "silo": true}
	if cfg.Metadata.Tier != "" && !validTiers[cfg.Metadata.Tier] {
		errs = append(errs, fmt.Sprintf("metadata.tier: must be pool, bridge, or silo; got %q", cfg.Metadata.Tier))
	}
	if cfg.Spec.Region == "" {
		errs = append(errs, "spec.region: must not be empty")
	}
	for i, intg := range cfg.Spec.Integrations {
		if intg.Name == "" {
			errs = append(errs, fmt.Sprintf("spec.integrations[%d]: name must not be empty", i))
		}
		if intg.Version == "" {
			errs = append(errs, fmt.Sprintf("spec.integrations[%d] (%s): version must not be empty", i, intg.Name))
		}
	}
	errs = append(errs, validateRouting(cfg.Spec.Routing)...)
	return errs
}

func validateRouting(r projectconfig.Routing) []string {
	var errs []string
	routes := []struct {
		name  string
		route projectconfig.SeverityRoute
	}{
		{name: "p1", route: r.P1},
		{name: "p2", route: r.P2},
		{name: "p3", route: r.P3},
		{name: "p4", route: r.P4},
	}
	validLLMTiers := map[string]bool{"A": true, "B": true, "C": true}
	validPolicies := map[string]bool{"auto": true, "manual": true, "skip": true}
	for _, item := range routes {
		if item.route.LLMTier == "" && item.route.ApprovalPolicy == "" {
			continue
		}
		prefix := "spec.routing." + item.name
		if !validLLMTiers[item.route.LLMTier] {
			errs = append(errs, fmt.Sprintf("%s.llm_tier: must be A, B, or C; got %q", prefix, item.route.LLMTier))
		}
		if !validPolicies[item.route.ApprovalPolicy] {
			errs = append(errs, fmt.Sprintf("%s.approval_policy: must be auto, manual, or skip; got %q", prefix, item.route.ApprovalPolicy))
		}
	}
	return errs
}

// paladin config validate
var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate paladin.yaml against the PaladinAI schema",
	Long: `Lints paladin.yaml in the current directory (or --file path).

Checks performed:
  - File exists and is valid YAML
  - apiVersion is paladin.io/v2
  - kind is Config
  - metadata.tenant is non-empty
  - metadata.tier is one of: pool, bridge, silo
  - spec.region is non-empty
  - Each integration has a non-empty name and version

Returns exit code 0 on success, 1 on validation errors.`,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := cmd.Flags().GetString("file")
		if path == "" {
			path = "paladin.yaml"
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		var cfg projectconfig.Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}

		errs := validateConfig(cfg)
		return writeConfigValidationResult(cmd, path, cfg, errs)
	},
}

// paladin config apply
var configApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply paladin.yaml changes to the control plane",
	Long: `Reads paladin.yaml (or --file path), validates it, and sends it to
the PaladinAI API. The control plane reconciles the desired state.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := cmd.Flags().GetString("file")
		if path == "" {
			path = "paladin.yaml"
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		const maxConfigBytes = 1 << 20 // 1 MiB
		if len(data) > maxConfigBytes {
			return fmt.Errorf("%s: file too large (%d bytes, max %d)", path, len(data), maxConfigBytes)
		}

		var cfg projectconfig.Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}

		// Full validation (same rules as `validate`) before sending.
		if errs := validateConfig(cfg); len(errs) > 0 {
			return writeConfigValidationResult(cmd, path, cfg, errs)
		}

		// Convert YAML → generic map to POST as JSON.
		var raw map[string]any
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		payload, err := json.Marshal(raw)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}

		u, err := url.Parse(apiURL(cmd))
		if err != nil {
			return fmt.Errorf("invalid api-url: %w", err)
		}
		u.Path = "/api/v1/config/apply"

		opts, err := commandOptions(cmd, cfg.Metadata.Tenant)
		if err != nil {
			return err
		}
		body, status, err := client.DoJSON(cmd.Context(), http.MethodPost, u.String(),
			opts, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		if status < 200 || status >= 300 {
			return fmt.Errorf("API error %d: %s", status, string(body))
		}

		return writeConfigApplyResult(cmd, path, cfg.Metadata.Tenant, body)
	},
}

type configValidationResult struct {
	Path         string   `json:"path"`
	Valid        bool     `json:"valid"`
	Integrations int      `json:"integrations"`
	Errors       []string `json:"errors"`
}

func writeConfigValidationResult(cmd *cobra.Command, path string, cfg projectconfig.Config, errs []string) error {
	if outputFormat(cmd) == "json" {
		result := configValidationResult{
			Path:         path,
			Valid:        len(errs) == 0,
			Integrations: len(cfg.Spec.Integrations),
			Errors:       errs,
		}
		if result.Errors == nil {
			result.Errors = []string{}
		}
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			return fmt.Errorf("write validation result: %w", err)
		}
		if len(errs) > 0 {
			return fmt.Errorf("found %d validation error(s)", len(errs))
		}
		return nil
	}

	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "%s: validation failed:\n", path)
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
		return fmt.Errorf("found %d validation error(s)", len(errs))
	}

	fmt.Fprintf(os.Stdout, "%s: OK (%d integration(s) configured)\n",
		path, len(cfg.Spec.Integrations))
	return nil
}

type configApplyResult struct {
	Applied  bool            `json:"applied"`
	Path     string          `json:"path"`
	Tenant   string          `json:"tenant"`
	Response json.RawMessage `json:"response,omitempty"`
}

func writeConfigApplyResult(cmd *cobra.Command, path, tenant string, body []byte) error {
	if outputFormat(cmd) == "json" {
		result := configApplyResult{Applied: true, Path: path, Tenant: tenant}
		if trimmed := bytes.TrimSpace(body); len(trimmed) > 0 && json.Valid(trimmed) {
			result.Response = append(json.RawMessage(nil), trimmed...)
		}
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			return fmt.Errorf("write apply result: %w", err)
		}
		return nil
	}

	fmt.Fprintf(os.Stdout, "Configuration applied (tenant: %s).\n", tenant)
	return nil
}

func init() {
	configValidateCmd.Flags().String("file", "", "Path to paladin.yaml (default: ./paladin.yaml)")
	configApplyCmd.Flags().String("file", "", "Path to paladin.yaml (default: ./paladin.yaml)")

	configCmd.AddCommand(configValidateCmd, configApplyCmd)
	rootCmd.AddCommand(configCmd)
}
