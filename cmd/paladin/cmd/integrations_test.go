package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/paladinai/paladinai/internal/integrationpkg"
	"github.com/paladinai/paladinai/internal/projectconfig"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintIntegrationsTable_ValidJSON(t *testing.T) {
	body := []byte(`{"data":[
		{"name":"datadog","category":"apm","status":"active","version":"1.0","healthy":true},
		{"name":"prometheus","category":"metrics","status":"active","version":"2.0","healthy":false}
	]}`)
	err := printIntegrationsTable(body)
	require.NoError(t, err)
}

func TestPrintIntegrationsTable_EmptyData(t *testing.T) {
	body := []byte(`{"data":[]}`)
	err := printIntegrationsTable(body)
	require.NoError(t, err)
}

func TestPrintIntegrationsTable_InvalidJSON_PrintsRaw(t *testing.T) {
	// Should not error — falls back to raw print.
	err := printIntegrationsTable([]byte("not json"))
	require.NoError(t, err)
}

func TestPrintIntegrationsTable_MissingFields_NoError(t *testing.T) {
	body := []byte(`{"data":[{"name":"partial"}]}`)
	err := printIntegrationsTable(body)
	require.NoError(t, err)
}

func TestPrintIntegrationsTable_NilData_NoError(t *testing.T) {
	body := []byte(`{"data":null}`)
	err := printIntegrationsTable(body)
	require.NoError(t, err)
}

// ─── envStr helper ────────────────────────────────────────────────────────────

func TestEnvStr_ReturnsEnvVar(t *testing.T) {
	t.Setenv("TEST_PALADIN_KEY", "hello")
	got := envStr("TEST_PALADIN_KEY", "fallback")
	assert.Equal(t, "hello", got)
}

func TestEnvStr_ReturnsFallback(t *testing.T) {
	got := envStr("_PALADIN_UNSET_KEY_12345", "fallback")
	assert.Equal(t, "fallback", got)
}

func TestUpdateProjectIntegration_CreatesProjectConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paladin.yaml")

	err := updateProjectIntegration(path, "tenant-a", "loki", "1.2.3", true, map[string]any{
		"url": "http://loki:3100",
	})
	require.NoError(t, err)

	cfg, err := projectconfig.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "tenant-a", cfg.Metadata.Tenant)

	var found *projectconfig.Integration
	for i := range cfg.Spec.Integrations {
		if cfg.Spec.Integrations[i].Name == "loki" {
			found = &cfg.Spec.Integrations[i]
			break
		}
	}
	require.NotNil(t, found)
	assert.True(t, found.Enabled)
	assert.Equal(t, "1.2.3", found.Version)
	assert.Equal(t, "http://loki:3100", found.Config["url"])
}

func TestUpdateProjectIntegration_DisablesExistingIntegration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paladin.yaml")
	cfg := projectconfig.Default("tenant-a", "bridge", "us-east-1")
	for i := range cfg.Spec.Integrations {
		if cfg.Spec.Integrations[i].Name == "prometheus" {
			cfg.Spec.Integrations[i].Enabled = true
			cfg.Spec.Integrations[i].Config = map[string]any{"url": "http://prometheus:9090"}
		}
	}
	require.NoError(t, projectconfig.WriteFile(path, cfg))

	err := updateProjectIntegration(path, "ignored-tenant", "prometheus", "9.9.9", false, nil)
	require.NoError(t, err)

	read, err := projectconfig.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "tenant-a", read.Metadata.Tenant)
	for _, integration := range read.Spec.Integrations {
		if integration.Name == "prometheus" {
			assert.False(t, integration.Enabled)
			assert.Equal(t, "1.4.0", integration.Version)
			assert.Equal(t, "http://prometheus:9090", integration.Config["url"])
			return
		}
	}
	t.Fatal("prometheus integration not found")
}

func TestNonSecretIntegrationConfig_RemovesSecretSchemaFields(t *testing.T) {
	def := &integrationpkg.Integration{
		ConfigSchema: map[string]integrationpkg.Field{
			"url":     {Type: "string"},
			"api_key": {Type: "string", Secret: true},
		},
	}

	filtered := nonSecretIntegrationConfig(map[string]any{
		"url":     "https://grafana.example.com",
		"api_key": "secret-token",
	}, def)

	require.NotNil(t, filtered)
	assert.Equal(t, "https://grafana.example.com", filtered["url"])
	assert.NotContains(t, filtered, "api_key")
}

func TestNonSecretIntegrationConfig_RemovesAuthSecrets(t *testing.T) {
	def := &integrationpkg.Integration{
		Auth: integrationpkg.AuthConfig{
			Fields: []integrationpkg.AuthField{
				{Name: "api_key", Secret: true},
				{Name: "app_key", Secret: true},
			},
		},
		ConfigSchema: map[string]integrationpkg.Field{
			"site": {Type: "string"},
		},
	}

	filtered := nonSecretIntegrationConfig(map[string]any{
		"api_key": "dd-secret",
		"app_key": "dd-app-secret",
		"site":    "datadoghq.eu",
	}, def)

	require.NotNil(t, filtered)
	assert.Equal(t, "datadoghq.eu", filtered["site"])
	assert.NotContains(t, filtered, "api_key")
	assert.NotContains(t, filtered, "app_key")
}

func TestIntegrationInlineConfig_SplitsSecretFlagsFromConfig(t *testing.T) {
	dir := writeIntegrationCatalog(t, "datadog", `
name: datadog
version: "1.0.0"
description: Datadog
receiver:
  type: webhook
auth:
  type: api_key
  fields:
    - name: api_key
      secret: true
    - name: app_key
      secret: true
config_schema:
  site:
    type: string
tools: []
`)
	cmd := newIntegrationEnableFlagTestCmd(t, dir)
	require.NoError(t, cmd.Flags().Set("api-key", "dd-api"))
	require.NoError(t, cmd.Flags().Set("app-key", "dd-app"))
	require.NoError(t, cmd.Flags().Set("site", "datadoghq.eu"))

	config, secrets, err := integrationInlineConfig(cmd, "datadog")
	require.NoError(t, err)

	assert.Equal(t, "dd-api", config["api_key"])
	assert.Equal(t, "dd-app", config["app_key"])
	assert.Equal(t, "datadoghq.eu", config["site"])
	assert.Equal(t, map[string]string{"api_key": "dd-api", "app_key": "dd-app"}, secrets)
}

func TestIntegrationInlineConfig_SetRequiresKeyValue(t *testing.T) {
	cmd := newIntegrationEnableFlagTestCmd(t, t.TempDir())
	require.NoError(t, cmd.Flags().Set("set", "missing-equals"))

	_, _, err := integrationInlineConfig(cmd, "unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "key=value")
}

func TestSaveIntegrationSecrets_StoresByTenantIntegrationAndKey(t *testing.T) {
	store := &fakeSecretStore{}
	withSecretStore(t, store)

	err := saveIntegrationSecrets("tenant-a", "datadog", map[string]string{
		"api_key": "dd-api",
		"app_key": "dd-app",
	})
	require.NoError(t, err)

	apiKey, err := paladinSecretStore.Get(tokenStoreService, integrationSecretAccount("tenant-a", "datadog", "api_key"))
	require.NoError(t, err)
	appKey, err := paladinSecretStore.Get(tokenStoreService, integrationSecretAccount("tenant-a", "datadog", "app_key"))
	require.NoError(t, err)
	assert.Equal(t, "dd-api", apiKey)
	assert.Equal(t, "dd-app", appKey)
}

func TestWriteIntegrationActionResult_CIModeJSON(t *testing.T) {
	cmd := tenantDeleteTestCmd(t, false, true)

	stdout := captureStdout(t, func() {
		require.NoError(t, writeIntegrationActionResult(cmd, integrationActionResult{
			Action:      "enable",
			Name:        "loki",
			Tenant:      "tenant-a",
			Enabled:     true,
			ProjectFile: "paladin.yaml",
			Response:    json.RawMessage(`{"status":"enabled"}`),
		}))
	})

	var result integrationActionResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, "enable", result.Action)
	assert.Equal(t, "loki", result.Name)
	assert.Equal(t, "tenant-a", result.Tenant)
	assert.True(t, result.Enabled)
	assert.Equal(t, "paladin.yaml", result.ProjectFile)
	assert.JSONEq(t, `{"status":"enabled"}`, string(result.Response))
	assert.NotContains(t, stdout, "Integration")
}

func TestWriteIntegrationActionResult_HumanOutput(t *testing.T) {
	cmd := tenantDeleteTestCmd(t, false, false)

	stdout := captureStdout(t, func() {
		require.NoError(t, writeIntegrationActionResult(cmd, integrationActionResult{
			Action:      "disable",
			Name:        "loki",
			Tenant:      "tenant-a",
			ProjectFile: "paladin.yaml",
		}))
	})

	assert.Equal(t, "Integration \"loki\" disabled.\n", stdout)
}

func TestIntegrationsLintCommand_JSONSuccess(t *testing.T) {
	dir := writeIntegrationCatalog(t, "valid", `
name: valid
version: "1.0.0"
description: Valid integration definition for testing
receiver:
  type: none
auth:
  type: none
tools:
  - query_metrics
docs_url: https://example.com/docs
`)

	var runErr error
	stdout := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"--ci", "integrations", "lint", "--integrations-dir", dir})
		t.Cleanup(func() { rootCmd.SetArgs(nil) })
		runErr = rootCmd.Execute()
	})

	require.NoError(t, runErr)
	var result integrationLintResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.True(t, result.Passed)
	assert.Equal(t, 1, result.Checked)
	assert.Empty(t, result.Issues)
}

func TestIntegrationsLintCommand_FailsOnInvalidCatalog(t *testing.T) {
	dir := writeIntegrationCatalog(t, "invalid", `
name: Invalid Name
version: "1.0.0"
description: anytime
receiver:
  type: webhook
  path: missing-slash
auth:
  type: made_up
`)

	var runErr error
	_ = captureStdout(t, func() {
		rootCmd.SetArgs([]string{"integrations", "lint", "--integrations-dir", dir})
		t.Cleanup(func() { rootCmd.SetArgs(nil) })
		runErr = rootCmd.Execute()
	})

	require.Error(t, runErr)
	assert.Contains(t, runErr.Error(), "integration lint failed")
}

func writeIntegrationCatalog(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	integrationDir := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(integrationDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(integrationDir, "integration.yaml"), []byte(content), 0o644))
	return dir
}

func newIntegrationEnableFlagTestCmd(t *testing.T, dir string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "enable"}
	cmd.Flags().String("integrations-dir", dir, "")
	cmd.Flags().String("api-key", "", "")
	cmd.Flags().String("app-key", "", "")
	cmd.Flags().String("bot-token", "", "")
	cmd.Flags().String("signing-secret", "", "")
	cmd.Flags().String("routing-key", "", "")
	cmd.Flags().String("url", "", "")
	cmd.Flags().String("site", "", "")
	cmd.Flags().String("default-channel", "", "")
	cmd.Flags().String("workspace-name", "", "")
	cmd.Flags().String("service-region", "", "")
	cmd.Flags().Bool("thread-on-update", true, "")
	cmd.Flags().StringArray("set", nil, "")
	return cmd
}
