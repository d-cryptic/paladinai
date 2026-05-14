package eval

import (
	"fmt"
	"strings"
)

// Score is the result of evaluating a single test case.
type Score struct {
	TestID         string
	Category       Category
	Pass           bool
	Score          float64 // 0.0 - 1.0
	Details        string
	TruePositives  int
	FalsePositives int
	FalseNegatives int
}

// SeverityScore checks if result severity matches expected (exact match,
// case-insensitive trim).
func SeverityScore(expected, got string) Score {
	exp := strings.ToUpper(strings.TrimSpace(expected))
	gt := strings.ToUpper(strings.TrimSpace(got))
	pass := exp != "" && exp == gt
	s := 0.0
	if pass {
		s = 1.0
	}
	return Score{
		Pass:           pass,
		Score:          s,
		Details:        fmt.Sprintf("expected=%q got=%q", exp, gt),
		TruePositives:  boolInt(pass),
		FalsePositives: boolInt(!pass && gt != ""),
		FalseNegatives: boolInt(!pass && exp != ""),
	}
}

// KeywordScore returns the fraction of expected keywords found in response
// (case-insensitive substring match). Pass if all are present.
func KeywordScore(expected []string, response string) Score {
	if len(expected) == 0 {
		return Score{Pass: true, Score: 1.0, Details: "no keywords required"}
	}
	resp := strings.ToLower(response)
	hits := 0
	var missing []string
	for _, kw := range expected {
		needle := strings.ToLower(strings.TrimSpace(kw))
		if needle == "" {
			continue
		}
		if strings.Contains(resp, needle) {
			hits++
		} else {
			missing = append(missing, kw)
		}
	}
	score := float64(hits) / float64(len(expected))
	return Score{
		Pass:           hits == len(expected),
		Score:          score,
		Details:        fmt.Sprintf("hits=%d/%d missing=%v", hits, len(expected), missing),
		TruePositives:  hits,
		FalseNegatives: len(missing),
	}
}

// SafetyScore returns 0.0 if any forbidden token appears in response, 1.0
// otherwise. Empty mustNot is treated as a pass.
func SafetyScore(mustNot []string, response string) Score {
	if len(mustNot) == 0 {
		return Score{Pass: true, Score: 1.0, Details: "no forbidden tokens"}
	}
	resp := strings.ToLower(response)
	var found []string
	for _, bad := range mustNot {
		needle := strings.ToLower(strings.TrimSpace(bad))
		if needle == "" {
			continue
		}
		if strings.Contains(resp, needle) {
			found = append(found, bad)
		}
	}
	if len(found) > 0 {
		return Score{
			Pass:           false,
			Score:          0.0,
			Details:        fmt.Sprintf("forbidden tokens present: %v", found),
			FalsePositives: len(found),
		}
	}
	return Score{Pass: true, Score: 1.0, Details: "clean"}
}

// AgentTypeScore checks if the routed agent type matches expected (exact match,
// case-insensitive trim). Used for supervisor_routing fixtures.
func AgentTypeScore(expected, got string) Score {
	exp := strings.ToLower(strings.TrimSpace(expected))
	gt := strings.ToLower(strings.TrimSpace(got))
	pass := exp != "" && exp == gt
	s := 0.0
	if pass {
		s = 1.0
	}
	return Score{
		Pass:           pass,
		Score:          s,
		Details:        fmt.Sprintf("expected=%q got=%q", exp, gt),
		TruePositives:  boolInt(pass),
		FalsePositives: boolInt(!pass && gt != ""),
		FalseNegatives: boolInt(!pass && exp != ""),
	}
}

// ToolF1Score computes the F1 score between expected and got tool name sets.
// Pass requires F1 == 1.0 (exact set match).
func ToolF1Score(expected, got []string) Score {
	expSet := toSet(expected)
	gotSet := toSet(got)

	if len(expSet) == 0 && len(gotSet) == 0 {
		return Score{Pass: true, Score: 1.0, Details: "both empty"}
	}

	tp := 0
	for k := range gotSet {
		if _, ok := expSet[k]; ok {
			tp++
		}
	}
	fp := len(gotSet) - tp
	fn := len(expSet) - tp

	var precision, recall, f1 float64
	if tp+fp > 0 {
		precision = float64(tp) / float64(tp+fp)
	}
	if tp+fn > 0 {
		recall = float64(tp) / float64(tp+fn)
	}
	if precision+recall > 0 {
		f1 = 2 * precision * recall / (precision + recall)
	}

	return Score{
		Pass:           f1 >= 1.0,
		Score:          f1,
		Details:        fmt.Sprintf("p=%.2f r=%.2f f1=%.2f tp=%d fp=%d fn=%d", precision, recall, f1, tp, fp, fn),
		TruePositives:  tp,
		FalsePositives: fp,
		FalseNegatives: fn,
	}
}

// CostRegressionScore checks whether observed token usage is within the
// tolerated increase over a stable baseline. The Stage 10 default tolerance is
// 10%, so callers should pass 0.10.
func CostRegressionScore(baselineTokens, observedTokens int, tolerance float64) Score {
	if baselineTokens <= 0 {
		return Score{Pass: false, Score: 0, Details: "baseline_tokens must be positive"}
	}
	if observedTokens < 0 {
		return Score{Pass: false, Score: 0, Details: "observed_tokens must not be negative"}
	}
	limit := float64(baselineTokens) * (1 + tolerance)
	pass := float64(observedTokens) <= limit
	score := 1.0
	if !pass && observedTokens > 0 {
		score = limit / float64(observedTokens)
	}
	return Score{
		Pass:           pass,
		Score:          clamp01(score),
		TruePositives:  boolInt(pass),
		FalsePositives: boolInt(!pass),
		Details: fmt.Sprintf("baseline=%d observed=%d limit=%.0f tolerance=%.2f",
			baselineTokens, observedTokens, limit, tolerance),
	}
}

// LatencyBudgetScore checks whether observed latency stays within the
// per-case budget expressed in milliseconds.
func LatencyBudgetScore(budgetMS, observedMS int) Score {
	if budgetMS <= 0 {
		return Score{Pass: false, Score: 0, Details: "latency_budget_ms must be positive"}
	}
	if observedMS < 0 {
		return Score{Pass: false, Score: 0, Details: "observed_latency_ms must not be negative"}
	}
	pass := observedMS <= budgetMS
	score := 1.0
	if !pass && observedMS > 0 {
		score = float64(budgetMS) / float64(observedMS)
	}
	return Score{
		Pass:           pass,
		Score:          clamp01(score),
		Details:        fmt.Sprintf("budget_ms=%d observed_ms=%d", budgetMS, observedMS),
		TruePositives:  boolInt(pass),
		FalsePositives: boolInt(!pass),
	}
}

// MemoryRecallScore computes a Stage 10 memory retrieval score using
// Precision@5 and Recall@10 over incident IDs. The final score weights recall
// higher because missing a relevant prior incident is more damaging than
// returning one extra neighbor.
func MemoryRecallScore(expected, recalled []string) Score {
	expSet := toSet(expected)
	if len(expSet) == 0 {
		return Score{Pass: false, Score: 0, Details: "expected_incident_ids must not be empty"}
	}
	top5 := firstN(recalled, 5)
	top10 := firstN(recalled, 10)

	precisionDenom := len(top5)
	if precisionDenom == 0 {
		precisionDenom = 5
	}
	precisionHits := countHits(expSet, top5)
	recallHits := countHits(expSet, top10)

	precisionAt5 := float64(precisionHits) / float64(precisionDenom)
	recallAt10 := float64(recallHits) / float64(len(expSet))
	score := 0.4*precisionAt5 + 0.6*recallAt10

	return Score{
		Pass:           recallAt10 >= 0.8 && precisionAt5 >= 0.6,
		Score:          clamp01(score),
		TruePositives:  recallHits,
		FalsePositives: len(top10) - recallHits,
		FalseNegatives: len(expSet) - recallHits,
		Details: fmt.Sprintf("precision_at_5=%.2f recall_at_10=%.2f hits_p5=%d hits_r10=%d expected=%d",
			precisionAt5, recallAt10, precisionHits, recallHits, len(expSet)),
	}
}

// RCACorrectnessScore computes a Stage 10 RCA score from root-cause identity
// and blast-radius recall. Exact root-cause matches get full credit; causes in
// the same broad failure family get partial credit.
func RCACorrectnessScore(expectedCause, predictedCause string, expectedBlast, predictedBlast []string) Score {
	expCause := strings.ToLower(strings.TrimSpace(expectedCause))
	gotCause := strings.ToLower(strings.TrimSpace(predictedCause))
	if expCause == "" {
		return Score{Pass: false, Score: 0, Details: "expected_root_cause must not be empty"}
	}
	if len(toSet(expectedBlast)) == 0 {
		return Score{Pass: false, Score: 0, Details: "expected_blast_radius must not be empty"}
	}

	causeScore := 0.0
	switch {
	case gotCause == "":
		causeScore = 0
	case expCause == gotCause:
		causeScore = 1
	case failureFamily(expCause) == failureFamily(gotCause):
		causeScore = 0.5
	}

	expectedSet := toSet(expectedBlast)
	blastHits := countHits(expectedSet, predictedBlast)
	blastRecall := float64(blastHits) / float64(len(expectedSet))
	score := 0.6*causeScore + 0.4*blastRecall

	return Score{
		Pass:           score >= 0.8,
		Score:          clamp01(score),
		TruePositives:  boolInt(causeScore >= 1) + blastHits,
		FalsePositives: maxInt(0, len(toSet(predictedBlast))-blastHits) + boolInt(gotCause != "" && causeScore < 1),
		FalseNegatives: maxInt(0, len(expectedSet)-blastHits) + boolInt(causeScore < 1),
		Details: fmt.Sprintf("cause=%.2f blast_recall=%.2f blast_hits=%d/%d expected_cause=%q predicted_cause=%q",
			causeScore, blastRecall, blastHits, len(expectedSet), expCause, gotCause),
	}
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// AggregateResults returns the pass rate and mean score across the slice.
func AggregateResults(scores []Score) (passRate, meanScore float64) {
	if len(scores) == 0 {
		return 0, 0
	}
	passed := 0
	sum := 0.0
	for _, s := range scores {
		if s.Pass {
			passed++
		}
		sum += s.Score
	}
	return float64(passed) / float64(len(scores)), sum / float64(len(scores))
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func firstN(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	return items[:n]
}

func countHits(want map[string]struct{}, got []string) int {
	hits := 0
	seen := make(map[string]struct{}, len(got))
	for _, item := range got {
		key := strings.TrimSpace(item)
		if key == "" {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		if _, ok := want[key]; ok {
			hits++
		}
	}
	return hits
}

func toSet(items []string) map[string]struct{} {
	m := make(map[string]struct{}, len(items))
	for _, it := range items {
		k := strings.TrimSpace(it)
		if k == "" {
			continue
		}
		m[k] = struct{}{}
	}
	return m
}

func failureFamily(cause string) string {
	c := strings.ToLower(strings.TrimSpace(cause))
	switch {
	case strings.Contains(c, "connection") || strings.Contains(c, "pool") || strings.Contains(c, "postgres") || strings.Contains(c, "db"):
		return "database"
	case strings.Contains(c, "memory") || strings.Contains(c, "oom") || strings.Contains(c, "leak"):
		return "memory"
	case strings.Contains(c, "network") || strings.Contains(c, "packet") || strings.Contains(c, "ingress") || strings.Contains(c, "502"):
		return "network"
	case strings.Contains(c, "disk") || strings.Contains(c, "io") || strings.Contains(c, "i/o") || strings.Contains(c, "etcd"):
		return "storage"
	case strings.Contains(c, "deploy") || strings.Contains(c, "deployment") || strings.Contains(c, "regression") || strings.Contains(c, "rollback"):
		return "deployment"
	case strings.Contains(c, "kubelet") || strings.Contains(c, "containerd") || strings.Contains(c, "node"):
		return "kubernetes"
	case strings.Contains(c, "rbac") || strings.Contains(c, "permission") || strings.Contains(c, "pvc"):
		return "permission"
	case strings.Contains(c, "timeout") || strings.Contains(c, "deadline") || strings.Contains(c, "grpc"):
		return "timeout"
	case strings.Contains(c, "consumer") || strings.Contains(c, "rebalance") || strings.Contains(c, "lag"):
		return "queue"
	default:
		return c
	}
}
