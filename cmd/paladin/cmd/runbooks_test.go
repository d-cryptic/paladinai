package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunbooksListTable_Empty(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"data": []any{}})
	if err := printRunbooksTable(body); err != nil {
		t.Fatalf("printRunbooksTable empty: %v", err)
	}
}

func TestRunbooksListTable_WithData(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"data": []any{
			map[string]any{
				"id":         "abc123",
				"title":      "How to recover redis from OOM",
				"source":     "github",
				"embedded":   true,
				"updated_at": "2026-05-11T03:00:00Z",
			},
		},
	})
	if err := printRunbooksTable(body); err != nil {
		t.Fatalf("printRunbooksTable: %v", err)
	}
}

func TestRunbooksListTable_LongTitleTruncated(t *testing.T) {
	longTitle := strings.Repeat("A", 50)
	body, _ := json.Marshal(map[string]any{
		"data": []any{
			map[string]any{
				"id":    "x",
				"title": longTitle,
			},
		},
	})
	if err := printRunbooksTable(body); err != nil {
		t.Fatalf("printRunbooksTable truncate: %v", err)
	}
}

func TestRunbooksListTable_InvalidJSON(t *testing.T) {
	// Should not return an error — just prints raw body.
	if err := printRunbooksTable([]byte("not json")); err != nil {
		t.Fatalf("expected nil error on bad JSON, got %v", err)
	}
}

func TestRunbookSearchResults_Empty(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"data": []any{}})
	if err := printRunbookSearchResults(body); err != nil {
		t.Fatalf("printRunbookSearchResults empty: %v", err)
	}
}

func TestRunbookSearchResults_WithHits(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"data": []any{
			map[string]any{
				"id":     "rb001",
				"title":  "Redis OOM recovery runbook",
				"source": "github",
				"score":  0.945,
			},
			map[string]any{
				"id":     "rb002",
				"title":  "Memory pressure playbook",
				"source": "confluence",
				"score":  0.921,
			},
		},
	})
	if err := printRunbookSearchResults(body); err != nil {
		t.Fatalf("printRunbookSearchResults: %v", err)
	}
}

func TestRunbookSearchResults_TitleTruncated(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"data": []any{
			map[string]any{
				"id":     "rb999",
				"title":  strings.Repeat("B", 60),
				"source": "notion",
				"score":  float64(0.99),
			},
		},
	})
	if err := printRunbookSearchResults(body); err != nil {
		t.Fatalf("printRunbookSearchResults truncate: %v", err)
	}
}

func TestFloatField_MissingKey(t *testing.T) {
	m := map[string]any{"other": "value"}
	if got := floatField(m, "score"); got != 0 {
		t.Errorf("missing key should return 0, got %v", got)
	}
}

func TestFloatField_Float64(t *testing.T) {
	m := map[string]any{"score": float64(0.92)}
	if got := floatField(m, "score"); got != 0.92 {
		t.Errorf("expected 0.92, got %v", got)
	}
}

func TestFloatField_Float32(t *testing.T) {
	m := map[string]any{"score": float32(0.5)}
	if got := floatField(m, "score"); got != float64(float32(0.5)) {
		t.Errorf("expected %v, got %v", float64(float32(0.5)), got)
	}
}

func TestWriteRunbooksImportResult_CIModeJSON(t *testing.T) {
	cmd := newRunbooksOutputTestCmd(true)

	stdout := captureStdout(t, func() {
		err := writeRunbooksImportResult(cmd, runbooksImportResult{
			Source:   "github",
			Imported: 3,
			JobID:    "job-123",
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	var result runbooksImportResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if result.Source != "github" || result.Imported != 3 || result.JobID != "job-123" {
		t.Fatalf("unexpected import result: %+v", result)
	}
}

func TestWriteRunbooksImportResult_HumanCompletedImport(t *testing.T) {
	cmd := newRunbooksOutputTestCmd(false)

	stdout := captureStdout(t, func() {
		err := writeRunbooksImportResult(cmd, runbooksImportResult{Source: "file", Imported: 2})
		if err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(stdout, "Imported 2 runbook(s) from file") {
		t.Fatalf("stdout = %q, want import summary", stdout)
	}
}

func newRunbooksOutputTestCmd(ci bool) *cobra.Command {
	cmd := &cobra.Command{Use: "runbooks"}
	cmd.Flags().StringP("output", "o", "table", "")
	cmd.Flags().Bool("ci", ci, "")
	return cmd
}
