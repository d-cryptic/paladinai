package cmd

import (
	"path/filepath"
	"testing"

	"github.com/paladinai/paladinai/internal/integrationpkg"
	"github.com/paladinai/paladinai/internal/projectconfig"
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
