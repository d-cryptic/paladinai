package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── strField ──────────────────────────────────────────────────────────────────

func TestStrField_ReturnsStringValue(t *testing.T) {
	m := map[string]any{"k": "hello"}
	assert.Equal(t, "hello", strField(m, "k"))
}

func TestStrField_MissingKeyReturnsEmpty(t *testing.T) {
	assert.Empty(t, strField(map[string]any{}, "missing"))
}

func TestStrField_NonStringValueReturnsEmpty(t *testing.T) {
	m := map[string]any{"k": 42}
	assert.Empty(t, strField(m, "k"))
}

func TestStrField_Exactly32CharsNotTruncated(t *testing.T) {
	s := strings.Repeat("a", 32)
	m := map[string]any{"k": s}
	assert.Equal(t, s, strField(m, "k"))
}

func TestStrField_Over32CharsTruncated(t *testing.T) {
	s := strings.Repeat("b", 33)
	m := map[string]any{"k": s}
	got := strField(m, "k")
	want := strings.Repeat("b", 32) + "…"
	assert.Equal(t, want, got)
	assert.Equal(t, len(strings.Repeat("b", 32))+len("…"), len(got),
		"should be 32 ASCII bytes + 3-byte UTF-8 ellipsis")
}

// ── printAlertTable ───────────────────────────────────────────────────────────

func alertTableBody(t *testing.T, alerts []map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{"data": alerts})
	require.NoError(t, err)
	return b
}

func TestPrintAlertTable_PrintsHeaders(t *testing.T) {
	out := captureStdout(t, func() {
		err := printAlertTable(alertTableBody(t, nil))
		require.NoError(t, err)
	})
	assert.Contains(t, out, "FINGERPRINT")
	assert.Contains(t, out, "SEVERITY")
	assert.Contains(t, out, "STATUS")
}

func TestPrintAlertTable_PrintsAlertRow(t *testing.T) {
	alert := map[string]any{
		"fingerprint":    "fp-abc123",
		"severity":       "p1",
		"status":         "firing",
		"correlation_id": "corr-xyz",
	}
	out := captureStdout(t, func() {
		err := printAlertTable(alertTableBody(t, []map[string]any{alert}))
		require.NoError(t, err)
	})
	assert.Contains(t, out, "fp-abc123")
	assert.Contains(t, out, "p1")
	assert.Contains(t, out, "firing")
	assert.Contains(t, out, "corr-xyz")
}

func TestPrintAlertTable_MalformedJSON_PrintsRawAndNoError(t *testing.T) {
	out := captureStdout(t, func() {
		err := printAlertTable([]byte("not json"))
		// Malformed JSON falls back to printing raw body and returning nil.
		require.NoError(t, err)
	})
	assert.Contains(t, out, "not json")
}

func TestPrintAlertTable_EmptyBody_PrintsRawAndNoError(t *testing.T) {
	out := captureStdout(t, func() {
		err := printAlertTable([]byte{})
		require.NoError(t, err)
	})
	// Empty body is not valid JSON; raw bytes (empty) are printed.
	_ = out
}

func TestPrintAlertTable_EmptyData_PrintsHeaderOnly(t *testing.T) {
	out := captureStdout(t, func() {
		err := printAlertTable(alertTableBody(t, []map[string]any{}))
		require.NoError(t, err)
	})
	assert.Contains(t, out, "FINGERPRINT")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	assert.Len(t, lines, 1)
}
