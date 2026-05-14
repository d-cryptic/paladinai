package main

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/paladinai/paladinai/internal/eval"
)

func TestPrintJSONSummary(t *testing.T) {
	result := &eval.RunResult{
		Total:     2,
		Passed:    1,
		Failed:    1,
		Skipped:   3,
		PassRate:  0.5,
		MeanScore: 0.75,
		Duration:  1500 * time.Millisecond,
		ByCategory: map[eval.Category]*eval.CategoryResult{
			eval.CategorySafety: {
				Total:     2,
				Passed:    1,
				PassRate:  0.5,
				MeanScore: 0.75,
			},
		},
	}

	var out bytes.Buffer
	if err := printJSONSummary(&out, result); err != nil {
		t.Fatalf("printJSONSummary: %v", err)
	}

	var summary evalJSONSummary
	if err := json.Unmarshal(out.Bytes(), &summary); err != nil {
		t.Fatalf("json summary is invalid: %v\n%s", err, out.String())
	}
	if summary.Total != 2 || summary.Passed != 1 || summary.Failed != 1 || summary.Skipped != 3 {
		t.Fatalf("unexpected totals: %+v", summary)
	}
	if summary.DurationMS != 1500 {
		t.Fatalf("duration_ms = %d, want 1500", summary.DurationMS)
	}
	safety, ok := summary.Categories["safety"]
	if !ok {
		t.Fatalf("missing safety category: %+v", summary.Categories)
	}
	if safety.Total != 2 || safety.Passed != 1 || safety.PassRate != 0.5 || safety.MeanScore != 0.75 {
		t.Fatalf("unexpected safety category: %+v", safety)
	}
}
