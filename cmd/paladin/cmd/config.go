package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/paladinai/paladinai/cmd/paladin/client"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// paladinYAML represents the project config file (paladin.yaml).
// Only the fields needed for validation are parsed; unknown fields are preserved.
type paladinYAML struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Tenant string `yaml:"tenant"`
		Tier   string `yaml:"tier"`
	} `yaml:"metadata"`
	Spec struct {
		Region       string   `yaml:"region"`
		Integrations []struct {
			Name    string `yaml:"name"`
			Version string `yaml:"version"`
			Enabled bool   `yaml:"enabled"`
		} `yaml:"integrations"`
	} `yaml:"spec"`
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage paladin.yaml project configuration",
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
	RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := cmd.Flags().GetString("file")
		if path == "" {
			path = "paladin.yaml"
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		var cfg paladinYAML
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}

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

		// Validate before sending.
		var cfg paladinYAML
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		if cfg.Metadata.Tenant == "" {
			return fmt.Errorf("%s: metadata.tenant is required", path)
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

		body, status, err := client.DoJSON(cmd.Context(), http.MethodPost, u.String(),
			client.Options{TenantID: cfg.Metadata.Tenant, Token: optToken(cmd)},
			bytes.NewReader(payload))
		if err != nil {
			return err
		}
		if status < 200 || status >= 300 {
			return fmt.Errorf("API error %d: %s", status, string(body))
		}

		fmt.Fprintf(os.Stdout, "Configuration applied (tenant: %s).\n", cfg.Metadata.Tenant)
		return nil
	},
}

func init() {
	configValidateCmd.Flags().String("file", "", "Path to paladin.yaml (default: ./paladin.yaml)")
	configApplyCmd.Flags().String("file", "", "Path to paladin.yaml (default: ./paladin.yaml)")

	configCmd.AddCommand(configValidateCmd, configApplyCmd)
	rootCmd.AddCommand(configCmd)
}
