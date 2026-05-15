package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

		configMap, err := readIntegrationConfigFile(configFile)
		if err != nil {
			return err
		}

		inlineConfig, secrets, err := integrationInlineConfig(cmd, args[0])
		if err != nil {
			return err
		}
		if len(inlineConfig) > 0 {
			if configMap == nil {
				configMap = make(map[string]any, len(inlineConfig))
			}
			for key, value := range inlineConfig {
				configMap[key] = value
			}
		}

		body, err := postIntegrationAction(cmd, tenant, args[0], "enable", configMap)
		if err != nil {
			return err
		}
		if err := saveIntegrationSecrets(tenant, args[0], secrets); err != nil {
			return err
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
		body, err := postIntegrationAction(cmd, tenant, args[0], "disable", nil)
		if err != nil {
			return err
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

func readIntegrationConfigFile(path string) (map[string]any, error) {
	if path == "" {
		return nil, nil
	}
	configData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	var configMap map[string]any
	if err := json.Unmarshal(configData, &configMap); err != nil {
		return nil, fmt.Errorf("parse config JSON: %w", err)
	}
	if configMap == nil {
		return nil, fmt.Errorf("config JSON must be an object")
	}
	return configMap, nil
}

func postIntegrationAction(cmd *cobra.Command, tenant, name, action string, config map[string]any) ([]byte, error) {
	var bodyReader io.Reader
	if action == "enable" || config != nil {
		payload := map[string]any{"name": name}
		if config != nil {
			payload["config"] = config
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	u, err := url.Parse(apiURL(cmd))
	if err != nil {
		return nil, fmt.Errorf("invalid api-url: %w", err)
	}
	u.Path = fmt.Sprintf("/api/v1/integrations/%s/%s", url.PathEscape(name), action)

	opts, err := commandOptions(cmd, tenant)
	if err != nil {
		return nil, err
	}
	body, status, err := client.DoJSON(cmd.Context(), http.MethodPost, u.String(), opts, bodyReader)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("API error %d: %s", status, string(body))
	}
	return body, nil
}

func integrationInlineConfig(cmd *cobra.Command, name string) (map[string]any, map[string]string, error) {
	def, err := integrationpkg.LoadByName(integrationsDir(cmd), name)
	if err != nil && !errors.Is(err, integrationpkg.ErrNotFound) {
		return nil, nil, fmt.Errorf("load integration: %w", err)
	}

	raw := make(map[string]any)
	for _, spec := range []struct {
		flag string
		key  string
	}{
		{flag: "api-key", key: "api_key"},
		{flag: "app-key", key: "app_key"},
		{flag: "bot-token", key: "bot_token"},
		{flag: "signing-secret", key: "signing_secret"},
		{flag: "routing-key", key: "routing_key"},
		{flag: "url", key: "url"},
		{flag: "site", key: "site"},
		{flag: "default-channel", key: "default_channel"},
		{flag: "workspace-name", key: "workspace_name"},
		{flag: "service-region", key: "service_region"},
	} {
		if !cmd.Flags().Changed(spec.flag) {
			continue
		}
		value, _ := cmd.Flags().GetString(spec.flag)
		if strings.TrimSpace(value) != "" {
			raw[spec.key] = value
		}
	}
	if cmd.Flags().Changed("thread-on-update") {
		value, _ := cmd.Flags().GetBool("thread-on-update")
		raw["thread_on_update"] = value
	}
	sets, _ := cmd.Flags().GetStringArray("set")
	for _, item := range sets {
		key, value, ok := strings.Cut(item, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, nil, fmt.Errorf("--set must be key=value")
		}
		raw[key] = value
	}
	if len(raw) == 0 {
		return nil, nil, nil
	}

	config := make(map[string]any, len(raw))
	secrets := make(map[string]string)
	for key, value := range raw {
		if isIntegrationSecretField(def, key) {
			text, ok := value.(string)
			if !ok || strings.TrimSpace(text) == "" {
				continue
			}
			secrets[key] = text
			config[key] = text
			continue
		}
		config[key] = value
	}
	return config, secrets, nil
}

func isIntegrationSecretField(def *integrationpkg.Integration, key string) bool {
	if def == nil {
		return false
	}
	for _, field := range def.Auth.Fields {
		if field.Name == key && field.Secret {
			return true
		}
	}
	if field, ok := def.ConfigSchema[key]; ok && field.Secret {
		return true
	}
	return false
}

func integrationSecretAccount(tenant, integration, key string) string {
	return "integrations/" + tenant + "/" + integration + "/" + key
}

func saveIntegrationSecrets(tenant, name string, secrets map[string]string) error {
	for key, value := range secrets {
		if err := paladinSecretStore.Set(tokenStoreService, integrationSecretAccount(tenant, name, key), strings.TrimSpace(value)); err != nil {
			return fmt.Errorf("save integration secret %s/%s to keychain: %w", name, key, err)
		}
	}
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
		if isIntegrationSecretField(def, key) {
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
	integrationsEnableCmd.Flags().String("api-key", "", "Integration API key; stored in the OS keychain when marked secret")
	integrationsEnableCmd.Flags().String("app-key", "", "Integration application key; stored in the OS keychain when marked secret")
	integrationsEnableCmd.Flags().String("bot-token", "", "Slack bot token; stored in the OS keychain")
	integrationsEnableCmd.Flags().String("signing-secret", "", "Webhook signing secret; stored in the OS keychain")
	integrationsEnableCmd.Flags().String("routing-key", "", "Incident routing key; stored in the OS keychain when marked secret")
	integrationsEnableCmd.Flags().String("url", "", "Integration base URL")
	integrationsEnableCmd.Flags().String("site", "", "Integration site or region hostname")
	integrationsEnableCmd.Flags().String("default-channel", "", "Default notification channel")
	integrationsEnableCmd.Flags().String("workspace-name", "", "Workspace display name")
	integrationsEnableCmd.Flags().String("service-region", "", "Service region")
	integrationsEnableCmd.Flags().Bool("thread-on-update", true, "Reply in thread for incident updates")
	integrationsEnableCmd.Flags().StringArray("set", nil, "Additional integration config as key=value; may be repeated")
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
