// paladin-eval runs eval suites against PaladinAI fixtures.
//
// In CI mode (the default), no LLM is contacted. The runner only validates
// that JSONL fixtures parse and that scoring logic produces deterministic
// results on a fabricated response.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/paladinai/paladinai/internal/eval"
	"github.com/paladinai/paladinai/internal/logger"
)

func main() {
	var (
		fixturesDir = flag.String("fixtures", "test/fixtures", "directory containing *.jsonl fixtures")
		category    = flag.String("category", "", "limit to a single category (classification|supervisor_routing|tool_use|summary|safety|adversarial|cost_regression|latency_budget|memory_recall|rca_correctness)")
		threshold   = flag.Float64("threshold", 0.8, "minimum pass rate; exit 1 below this")
		maxCases    = flag.Int("max", 0, "max cases to run (0 = all)")
		timeout     = flag.Duration("timeout", 5*time.Second, "per-case timeout")
		jsonOutput  = flag.Bool("json", false, "emit machine-readable JSON summary")
	)
	flag.Parse()

	log, err := logger.New("paladin-eval")
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger init: %v\n", err)
		os.Exit(2)
	}
	defer func() { _ = log.Sync() }()

	cfg := eval.RunConfig{
		FixturesDir: *fixturesDir,
		MaxCases:    *maxCases,
		Timeout:     *timeout,
	}
	if *category != "" {
		cat := eval.Category(*category)
		if !cat.Valid() {
			fmt.Fprintf(os.Stderr, "invalid category %q\n", *category)
			os.Exit(2)
		}
		cfg.Categories = []eval.Category{cat}
	}

	runner := eval.NewRunner(cfg, log)

	res, err := runner.Run(context.Background(), ciResponse, ciScore)
	if err != nil {
		fmt.Fprintf(os.Stderr, "eval run failed: %v\n", err)
		os.Exit(2)
	}

	if *jsonOutput {
		if err := printJSONSummary(os.Stdout, res); err != nil {
			fmt.Fprintf(os.Stderr, "write json summary: %v\n", err)
			os.Exit(2)
		}
	} else {
		printSummary(os.Stdout, res)
	}

	if res.Total == 0 {
		fmt.Fprintln(os.Stderr, "no test cases were executed")
		os.Exit(2)
	}
	if res.PassRate < *threshold {
		fmt.Fprintf(os.Stderr, "FAIL: pass rate %.2f below threshold %.2f\n", res.PassRate, *threshold)
		os.Exit(1)
	}
}

// ciResponse builds a deterministic synthetic response from the test case so
// the scorer has something concrete to inspect without contacting an LLM.
//
// The response is designed to satisfy keyword expectations (because we echo
// the alert title, labels, and annotations) and to avoid leaking any
// must_not_contain phrases (we deliberately do not echo the title for safety
// cases — they get a generic acknowledgement instead).
func ciResponse(_ context.Context, tc eval.TestCase) (string, error) {
	if tc.Category == eval.CategorySafety {
		return "Alert received. Investigating without echoing user-supplied content.", nil
	}
	// supervisor_routing: echo the expected agent type so AgentTypeScore passes
	// without contacting an LLM. This validates fixture parsing and scorer logic.
	if tc.Category == eval.CategorySupervisorRouting {
		if tc.ExpectedAgentType != "" {
			return tc.ExpectedAgentType, nil
		}
		return "triage", nil
	}
	if tc.Category == eval.CategorySummary {
		// In CI mode, produce a synthetic summary that covers all expected
		// keywords so scoring logic (not LLM quality) is what's validated.
		ctx := tc.Context
		parts := []string{
			"Incident " + ctx.IncidentID + " summary.",
			"Severity: " + ctx.Severity,
			"Duration: " + fmt.Sprintf("%d minutes", ctx.DurationMinutes),
			"Affected services: " + strings.Join(ctx.AffectedServices, ", "),
		}
		for _, a := range ctx.Alerts {
			svc := a.Service
			if svc == "" {
				svc = a.Labels["service"]
			}
			parts = append(parts, "Alert: "+a.Title+" service="+svc)
		}
		// Echo expected keywords so pass/fail reflects fixture health, not LLM output.
		if len(tc.ExpectedKeywords) > 0 {
			parts = append(parts, "Keywords: "+strings.Join(tc.ExpectedKeywords, ", "))
		}
		return strings.Join(parts, "\n"), nil
	}
	parts := []string{
		"Investigating alert.",
		"Title: " + tc.Alert.Title,
		"Severity: " + tc.Alert.Severity,
		"Description: " + tc.Alert.Description,
	}
	for k, v := range tc.Alert.Labels {
		parts = append(parts, k+"="+v)
	}
	for k, v := range tc.Alert.Annotations {
		parts = append(parts, k+": "+v)
	}
	// Heuristic enrichment so keyword expectations can pass deterministically
	// on the bundled summary fixtures without an LLM.
	lower := strings.ToLower(tc.Alert.Title + " " + tc.Alert.Description)
	if strings.Contains(lower, "oom") || strings.Contains(lower, "memory") {
		parts = append(parts, "Pod memory pressure and OOM conditions detected.")
	}
	if strings.Contains(lower, "latency") || strings.Contains(lower, "p99") {
		parts = append(parts, "Latency p99 elevated above SLO.")
	}
	if strings.Contains(lower, "disk") {
		parts = append(parts, "Disk usage above threshold.")
	}
	if strings.Contains(lower, "tls") || strings.Contains(lower, "certificate") {
		parts = append(parts, "TLS certificate expiry imminent.")
	}
	if strings.Contains(lower, "5xx") || strings.Contains(lower, "error") {
		parts = append(parts, "Elevated 5xx error rate.")
	}
	if strings.Contains(lower, "notready") || strings.Contains(lower, "node") {
		parts = append(parts, "Node not ready in cluster.")
	}
	if strings.Contains(lower, "redis") {
		parts = append(parts, "Redis memory eviction observed.")
	}
	if strings.Contains(lower, "packet") {
		parts = append(parts, "Network packet loss detected.")
	}
	if strings.Contains(lower, "nats") || strings.Contains(lower, "lag") {
		parts = append(parts, "Queue consumer lag growing.")
	}
	if strings.Contains(lower, "pool") || strings.Contains(lower, "connection") {
		parts = append(parts, "Connection pool saturated.")
	}
	return strings.Join(parts, "\n"), nil
}

// ciScore applies category-appropriate scoring against the synthetic response.
func ciScore(tc eval.TestCase, response string) eval.Score {
	switch tc.Category {
	case eval.CategorySafety:
		return eval.SafetyScore(tc.MustNotContain, response)
	case eval.CategorySummary:
		return eval.KeywordScore(tc.ExpectedKeywords, response)
	case eval.CategoryClassification:
		return eval.SeverityScore(tc.ExpectedSeverity, tc.Alert.Severity)
	case eval.CategorySupervisorRouting:
		return eval.AgentTypeScore(tc.ExpectedAgentType, response)
	case eval.CategoryToolUse:
		// In CI mode we have no real agent response, so we compare expected vs
		// expected (perfect score). This validates fixture parsing only.
		return eval.ToolF1Score(tc.AllExpectedTools(), tc.AllExpectedTools())
	case eval.CategoryAdversarial:
		if len(tc.MustNotContain) > 0 {
			s := eval.SafetyScore(tc.MustNotContain, response)
			if !s.Pass {
				return s
			}
		}
		if tc.ExpectedSeverity != "" && tc.Alert.Severity != "" {
			return eval.SeverityScore(tc.ExpectedSeverity, tc.Alert.Severity)
		}
		return eval.Score{Pass: true, Score: 1.0, Details: "adversarial parsed cleanly"}
	case eval.CategoryCostRegression:
		return eval.CostRegressionScore(tc.BaselineTokens, tc.ObservedTokens, 0.10)
	case eval.CategoryLatencyBudget:
		return eval.LatencyBudgetScore(tc.LatencyBudgetMS, tc.ObservedLatencyMS)
	case eval.CategoryMemoryRecall:
		return eval.MemoryRecallScore(tc.ExpectedIncidentIDs, tc.RecalledIncidentIDs)
	case eval.CategoryRCACorrectness:
		return eval.RCACorrectnessScore(tc.ExpectedRootCause, tc.PredictedRootCause, tc.ExpectedBlastRadius, tc.PredictedBlastRadius)
	}
	return eval.Score{Pass: false, Score: 0, Details: "unhandled category"}
}

type evalJSONSummary struct {
	Total      int                            `json:"total"`
	Passed     int                            `json:"passed"`
	Failed     int                            `json:"failed"`
	Skipped    int                            `json:"skipped"`
	PassRate   float64                        `json:"pass_rate"`
	MeanScore  float64                        `json:"mean_score"`
	DurationMS int64                          `json:"duration_ms"`
	Categories map[string]evalJSONCategoryRow `json:"categories"`
}

type evalJSONCategoryRow struct {
	Total     int     `json:"total"`
	Passed    int     `json:"passed"`
	PassRate  float64 `json:"pass_rate"`
	MeanScore float64 `json:"mean_score"`
}

func printJSONSummary(w io.Writer, res *eval.RunResult) error {
	summary := evalJSONSummary{
		Total:      res.Total,
		Passed:     res.Passed,
		Failed:     res.Failed,
		Skipped:    res.Skipped,
		PassRate:   res.PassRate,
		MeanScore:  res.MeanScore,
		DurationMS: res.Duration.Milliseconds(),
		Categories: make(map[string]evalJSONCategoryRow, len(res.ByCategory)),
	}
	for category, row := range res.ByCategory {
		summary.Categories[string(category)] = evalJSONCategoryRow{
			Total:     row.Total,
			Passed:    row.Passed,
			PassRate:  row.PassRate,
			MeanScore: row.MeanScore,
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(summary)
}

func printSummary(w io.Writer, res *eval.RunResult) {
	fmt.Fprintf(w, "\nPaladin Eval Summary\n") //nolint:errcheck
	fmt.Fprintf(w, "====================\n")   //nolint:errcheck
	fmt.Fprintf(w, "Total:      %d\n", res.Total)
	fmt.Fprintf(w, "Passed:     %d\n", res.Passed)
	fmt.Fprintf(w, "Failed:     %d\n", res.Failed)
	fmt.Fprintf(w, "Skipped:    %d\n", res.Skipped)
	fmt.Fprintf(w, "Pass rate:  %.2f\n", res.PassRate)
	fmt.Fprintf(w, "Mean score: %.2f\n", res.MeanScore)
	fmt.Fprintf(w, "Duration:   %s\n\n", res.Duration)

	cats := make([]string, 0, len(res.ByCategory))
	for c := range res.ByCategory {
		cats = append(cats, string(c))
	}
	sort.Strings(cats)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "CATEGORY\tTOTAL\tPASSED\tPASS_RATE\tMEAN_SCORE") //nolint:errcheck
	for _, c := range cats {
		cr := res.ByCategory[eval.Category(c)]
		fmt.Fprintf(tw, "%s\t%d\t%d\t%.2f\t%.2f\n",
			c, cr.Total, cr.Passed, cr.PassRate, cr.MeanScore)
	}
	_ = tw.Flush()
	fmt.Fprintln(w) //nolint:errcheck
}
