package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func incidentTableBody(t *testing.T, incidents []map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{"data": incidents})
	require.NoError(t, err)
	return b
}

func TestPrintIncidentTable_PrintsHeaders(t *testing.T) {
	out := captureStdout(t, func() {
		err := printIncidentTable(incidentTableBody(t, nil))
		require.NoError(t, err)
	})
	assert.Contains(t, out, "ID")
	assert.Contains(t, out, "TITLE")
	assert.Contains(t, out, "SEVERITY")
	assert.Contains(t, out, "STATUS")
}

func TestPrintIncidentTable_PrintsRow(t *testing.T) {
	inc := map[string]any{
		"id":       "inc-001",
		"title":    "Database latency spike",
		"severity": "p2",
		"status":   "open",
	}
	out := captureStdout(t, func() {
		err := printIncidentTable(incidentTableBody(t, []map[string]any{inc}))
		require.NoError(t, err)
	})
	assert.Contains(t, out, "inc-001")
	assert.Contains(t, out, "Database latency spike")
	assert.Contains(t, out, "p2")
	assert.Contains(t, out, "open")
}

// TestPrintIncidentTable_IDNotTruncated ensures incident IDs are printed in full,
// unlike strField which truncates at 32 chars.
func TestPrintIncidentTable_IDNotTruncated(t *testing.T) {
	longID := strings.Repeat("x", 40) // 40 chars — beyond strField's 32-char limit
	inc := map[string]any{"id": longID, "title": "t", "severity": "p1", "status": "open"}
	out := captureStdout(t, func() {
		err := printIncidentTable(incidentTableBody(t, []map[string]any{inc}))
		require.NoError(t, err)
	})
	assert.Contains(t, out, longID, "incident IDs must not be truncated")
	assert.NotContains(t, out, "…", "no ellipsis should appear in the ID column")
}

func TestPrintIncidentTable_MalformedJSON_PrintsRawAndNoError(t *testing.T) {
	out := captureStdout(t, func() {
		err := printIncidentTable([]byte("not json {{"))
		require.NoError(t, err)
	})
	assert.Contains(t, out, "not json")
}

func TestPrintIncidentTable_EmptyData_PrintsHeaderOnly(t *testing.T) {
	out := captureStdout(t, func() {
		err := printIncidentTable(incidentTableBody(t, []map[string]any{}))
		require.NoError(t, err)
	})
	assert.Contains(t, out, "TITLE")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	assert.Len(t, lines, 1)
}

func TestWriteIncidentResolveResult_CIModeJSON(t *testing.T) {
	cmd := tenantDeleteTestCmd(t, false, true)

	stdout := captureStdout(t, func() {
		require.NoError(t, writeIncidentResolveResult(cmd, incidentResolveResult{
			IncidentID: "inc-001",
			Resolved:   true,
			Response:   json.RawMessage(`{"status":"resolved"}`),
		}))
	})

	var result incidentResolveResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, "inc-001", result.IncidentID)
	assert.True(t, result.Resolved)
	assert.JSONEq(t, `{"status":"resolved"}`, string(result.Response))
	assert.NotContains(t, stdout, "Incident")
}

func TestWriteIncidentResolveResult_HumanOutput(t *testing.T) {
	cmd := tenantDeleteTestCmd(t, false, false)

	stdout := captureStdout(t, func() {
		require.NoError(t, writeIncidentResolveResult(cmd, incidentResolveResult{
			IncidentID: "inc-001",
			Resolved:   true,
		}))
	})

	assert.Equal(t, "Incident \"inc-001\" resolved.\n", stdout)
}
