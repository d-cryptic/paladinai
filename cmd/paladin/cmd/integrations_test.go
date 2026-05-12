package cmd

import (
	"testing"

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
