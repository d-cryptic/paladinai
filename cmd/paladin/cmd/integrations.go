package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/paladinai/paladinai/cmd/paladin/client"
	"github.com/paladinai/paladinai/internal/integrationpkg"
	"github.com/paladinai/paladinai/internal/projectconfig"
	"github.com/spf13/cobra"
)

var integrationsCmd = &cobra.Command{
	Use:   "integrations",
	Short: "Manage PaladinAI integrations (MCP server marketplace)",
}

// integrationsDir resolves the directory holding integration.yaml definitions.
// Order: --integrations-dir flag, ./integrations, then <binary>/../integrations.
func integrationsDir(cmd *cobra.Command) string {
	if dir, _ := cmd.Flags().GetString("integrations-dir"); dir != "" {
		return dir
	}
	if _, err := os.Stat("integrations"); err == nil {
		return "integrations"
	}
	if exe, err := exec.LookPath(os.Args[0]); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "..", "integrations")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "integrations"
}

var integrationsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available integrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		all, err := integrationpkg.LoadAll(integrationsDir(cmd))
		if err != nil {
			return fmt.Errorf("load integrations: %w", err)
		}

		if outputFormat(cmd) == "json" {
			data, err := json.MarshalIndent(all, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal json: %w", err)
			}
			fmt.Fprintln(os.Stdout, string(data))
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tVERSION\tDESCRIPTION\tTOOLS")
		for _, i := range all {
			toolsStr := strings.Join(i.Tools, ",")
			if toolsStr == "" {
				toolsStr = "-"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", i.Name, i.Version, i.Description, toolsStr)
		}
		return w.Flush()
	},
}

var integrationsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show enabled integrations and their health",
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant, err := requireTenant(cmd)
		if err != nil {
			return err
		}
		u, err := url.Parse(apiURL(cmd))
		if err != nil {
			return fmt.Errorf("invalid api-url: %w", err)
		}
		u.Path = "/api/v1/integrations/status"

		opts, err := commandOptions(cmd, tenant)
		if err != nil {
			return err
		}
		body, err := client.Get(cmd.Context(), u.String(), opts)
		if err != nil {
			return err
		}

		if outputFormat(cmd) == "json" {
			fmt.Fprintln(os.Stdout, string(body))
			return nil
		}
		return printIntegrationsTable(body)
	},
}

var integrationsEnableCmd = &cobra.Command{
	Use:   "enable <name>",
	Short: "Enable an integration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant, err := requireTenant(cmd)
		if err != nil {
			return err
		}

		configFile, _ := cmd.Flags().GetString("config")
		projectFile, _ := cmd.Flags().GetString("file")
		if projectFile == "" {
			projectFile = "paladin.yaml"
		}

		var configData []byte
		configMap := map[string]any(nil)
		if configFile != "" {
			var err error
			configData, err = os.ReadFile(configFile)
			if err != nil {
				return fmt.Errorf("read config file: %w", err)
			}
			if err := json.Unmarshal(configData, &configMap); err != nil {
				return fmt.Errorf("parse config JSON: %w", err)
			}
			if configMap == nil {
				return fmt.Errorf("config JSON must be an object")
			}
		}

		payload := map[string]any{"name": args[0]}
		if configMap != nil {
			payload["config"] = configMap
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}

		u, err := url.Parse(apiURL(cmd))
		if err != nil {
			return fmt.Errorf("invalid api-url: %w", err)
		}
		u.Path = fmt.Sprintf("/api/v1/integrations/%s/enable", url.PathEscape(args[0]))

		opts, err := commandOptions(cmd, tenant)
		if err != nil {
			return err
		}
		body, status, err := client.DoJSON(cmd.Context(), http.MethodPost, u.String(),
			opts, bytes.NewReader(data))
		if err != nil {
			return err
		}
		if status < 200 || status >= 300 {
			return fmt.Errorf("API error %d: %s", status, string(body))
		}
		if err := updateProjectIntegrationFromDefinition(cmd, projectFile, tenant, args[0], true, configMap); err != nil {
			return err
		}
		return writeIntegrationActionResult(cmd, integrationActionResult{
			Action:      "enable",
			Name:        args[0],
			Tenant:      tenant,
			Enabled:     true,
			ProjectFile: projectFile,
			Response:    jsonResponseBody(body),
			Message:     nonJSONResponseMessage(body),
		})
	},
}

var integrationsDisableCmd = &cobra.Command{
	Use:   "disable <name>",
	Short: "Disable an integration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant, err := requireTenant(cmd)
		if err != nil {
			return err
		}
		projectFile, _ := cmd.Flags().GetString("file")
		if projectFile == "" {
			projectFile = "paladin.yaml"
		}
		u, err := url.Parse(apiURL(cmd))
		if err != nil {
			return fmt.Errorf("invalid api-url: %w", err)
		}
		u.Path = fmt.Sprintf("/api/v1/integrations/%s/disable", url.PathEscape(args[0]))

		opts, err := commandOptions(cmd, tenant)
		if err != nil {
			return err
		}
		body, status, err := client.DoJSON(cmd.Context(), http.MethodPost, u.String(),
			opts, nil)
		if err != nil {
			return err
		}
		if status < 200 || status >= 300 {
			return fmt.Errorf("API error %d: %s", status, string(body))
		}
		if err := updateProjectIntegrationFromDefinition(cmd, projectFile, tenant, args[0], false, nil); err != nil {
			return err
		}
		return writeIntegrationActionResult(cmd, integrationActionResult{
			Action:      "disable",
			Name:        args[0],
			Tenant:      tenant,
			Enabled:     false,
			ProjectFile: projectFile,
			Response:    jsonResponseBody(body),
			Message:     nonJSONResponseMessage(body),
		})
	},
}

var integrationsShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show details of a specific integration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		integ, err := integrationpkg.LoadByName(integrationsDir(cmd), args[0])
		if err != nil {
			return fmt.Errorf("load integration: %w", err)
		}

		if outputFormat(cmd) == "json" {
			data, err := json.MarshalIndent(integ, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal json: %w", err)
			}
			fmt.Fprintln(os.Stdout, string(data))
			return nil
		}

		printIntegrationDefinition(integ)
		return nil
	},
}

var integrationsLintCmd = &cobra.Command{
	Use:   "lint [name]",
	Short: "Validate integration package definitions",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var integrations []integrationpkg.Integration
		if len(args) == 1 {
			integ, err := integrationpkg.LoadByName(integrationsDir(cmd), args[0])
			if err != nil {
				return fmt.Errorf("load integration: %w", err)
			}
			integrations = []integrationpkg.Integration{*integ}
		} else {
			all, err := integrationpkg.LoadAll(integrationsDir(cmd))
			if err != nil {
				return fmt.Errorf("load integrations: %w", err)
			}
			integrations = all
		}

		issues := make([]integrationpkg.LintIssue, 0)
		for _, integ := range integrations {
			issues = append(issues, integrationpkg.Lint(integ)...)
		}

		if outputFormat(cmd) == "json" {
			result := integrationLintResult{
				Checked: len(integrations),
				Passed:  !integrationpkg.HasLintErrors(issues),
				Issues:  issues,
			}
			enc := json.NewEncoder(os.Stdout)
			if err := enc.Encode(result); err != nil {
				return fmt.Errorf("write json: %w", err)
			}
		} else {
			printIntegrationLintResult(len(integrations), issues)
		}

		if integrationpkg.HasLintErrors(issues) {
			return fmt.Errorf("integration lint failed")
		}
		return nil
	},
}

type integrationLintResult struct {
	Checked int                        `json:"checked"`
	Passed  bool                       `json:"passed"`
	Issues  []integrationpkg.LintIssue `json:"issues"`
}

func printIntegrationLintResult(checked int, issues []integrationpkg.LintIssue) {
	if len(issues) == 0 {
		fmt.Fprintf(os.Stdout, "Checked %d integration(s): ok\n", checked)
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SEVERITY\tINTEGRATION\tFIELD\tMESSAGE")
	for _, issue := range issues {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", issue.Severity, issue.Integration, issue.Field, issue.Message)
	}
	_ = w.Flush()
}

func printIntegrationsTable(body []byte) error {
	var response struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Fprintln(os.Stdout, string(body))
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tCATEGORY\tSTATUS\tVERSION\tHEALTHY")
	for _, i := range response.Data {
		name := strField(i, "name")
		cat := strField(i, "category")
		status := strField(i, "status")
		version := strField(i, "version")
		healthy := fmt.Sprintf("%v", i["healthy"])
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", name, cat, status, version, healthy)
	}
	return w.Flush()
}

func printIntegrationDefinition(i *integrationpkg.Integration) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Name:\t%s\n", i.Name)
	fmt.Fprintf(w, "Version:\t%s\n", i.Version)
	fmt.Fprintf(w, "Description:\t%s\n", i.Description)
	fmt.Fprintf(w, "Docs:\t%s\n", i.DocsURL)
	fmt.Fprintf(w, "Receiver:\t%s\n", i.Receiver.Type)
	if i.Receiver.Path != "" {
		fmt.Fprintf(w, "  Path:\t%s\n", i.Receiver.Path)
	}
	if i.Receiver.HMACHeader != "" {
		fmt.Fprintf(w, "  HMAC Header:\t%s\n", i.Receiver.HMACHeader)
	}
	fmt.Fprintf(w, "Auth:\t%s\n", i.Auth.Type)
	for _, f := range i.Auth.Fields {
		secret := ""
		if f.Secret {
			secret = " (secret)"
		}
		fmt.Fprintf(w, "  - %s%s\t%s\n", f.Name, secret, f.Description)
	}
	if len(i.Tools) > 0 {
		fmt.Fprintf(w, "Tools:\t%s\n", strings.Join(i.Tools, ", "))
	}
	if len(i.ConfigSchema) > 0 {
		fmt.Fprintln(w, "Config schema:")
		for key, field := range i.ConfigSchema {
			req := ""
			if field.Required {
				req = " (required)"
			}
			fmt.Fprintf(w, "  - %s [%s]%s\t%s\n", key, field.Type, req, field.Description)
		}
	}
	w.Flush()
}

type integrationActionResult struct {
	Action      string          `json:"action"`
	Name        string          `json:"name"`
	Tenant      string          `json:"tenant"`
	Enabled     bool            `json:"enabled"`
	ProjectFile string          `json:"project_file"`
	Response    json.RawMessage `json:"response,omitempty"`
	Message     string          `json:"message,omitempty"`
}

func writeIntegrationActionResult(cmd *cobra.Command, result integrationActionResult) error {
	if outputFormat(cmd) == "json" {
		enc := json.NewEncoder(os.Stdout)
		if err := enc.Encode(result); err != nil {
			return fmt.Errorf("write json: %w", err)
		}
		return nil
	}
	state := "enabled"
	if !result.Enabled {
		state = "disabled"
	}
	fmt.Printf("Integration %q %s.\n", result.Name, state)
	return nil
}

func updateProjectIntegrationFromDefinition(cmd *cobra.Command, path, tenant, name string, enabled bool, config map[string]any) error {
	version := "latest"
	projectConfig := config
	if def, err := integrationpkg.LoadByName(integrationsDir(cmd), name); err == nil {
		version = def.Version
		projectConfig = nonSecretIntegrationConfig(config, def)
	}
	if err := updateProjectIntegration(path, tenant, name, version, enabled, projectConfig); err != nil {
		return fmt.Errorf("update %s: %w", path, err)
	}
	return nil
}

func nonSecretIntegrationConfig(config map[string]any, def *integrationpkg.Integration) map[string]any {
	if len(config) == 0 {
		return nil
	}
	filtered := make(map[string]any, len(config))
	for key, value := range config {
		if field, ok := def.ConfigSchema[key]; ok && field.Secret {
			continue
		}
		filtered[key] = value
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func updateProjectIntegration(path, tenant, name, version string, enabled bool, config map[string]any) error {
	if name == "" {
		return fmt.Errorf("integration name is required")
	}
	cfg, err := projectconfig.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		cfg = projectconfig.Default(tenant, "", "")
	}
	if cfg.Metadata.Tenant == "" {
		cfg.Metadata.Tenant = tenant
	}
	if version == "" {
		version = "latest"
	}

	for i := range cfg.Spec.Integrations {
		if cfg.Spec.Integrations[i].Name != name {
			continue
		}
		cfg.Spec.Integrations[i].Enabled = enabled
		if cfg.Spec.Integrations[i].Version == "" {
			cfg.Spec.Integrations[i].Version = version
		}
		if config != nil {
			cfg.Spec.Integrations[i].Config = config
		}
		return projectconfig.WriteFile(path, cfg)
	}

	cfg.Spec.Integrations = append(cfg.Spec.Integrations, projectconfig.Integration{
		Name:    name,
		Version: version,
		Enabled: enabled,
		Config:  config,
	})
	return projectconfig.WriteFile(path, cfg)
}

func init() {
	integrationsCmd.PersistentFlags().String("integrations-dir", "", "Path to integrations/ directory with integration.yaml files")
	integrationsEnableCmd.Flags().String("config", "", "Path to JSON config file for the integration")
	integrationsEnableCmd.Flags().String("file", "", "Path to paladin.yaml (default: ./paladin.yaml)")
	integrationsDisableCmd.Flags().String("file", "", "Path to paladin.yaml (default: ./paladin.yaml)")

	integrationsCmd.AddCommand(
		integrationsListCmd,
		integrationsStatusCmd,
		integrationsEnableCmd,
		integrationsDisableCmd,
		integrationsShowCmd,
		integrationsLintCmd,
	)
	rootCmd.AddCommand(integrationsCmd)
}
