package eval

import (
	"fmt"
	"strings"
)

// Score is the result of evaluating a single test case.
type Score struct {
	TestID   string
	Category Category
	Pass     bool
	Score    float64 // 0.0 - 1.0
	Details  string
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
		Pass:    pass,
		Score:   s,
		Details: fmt.Sprintf("expected=%q got=%q", exp, gt),
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
		Pass:    hits == len(expected),
		Score:   score,
		Details: fmt.Sprintf("hits=%d/%d missing=%v", hits, len(expected), missing),
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
			Pass:    false,
			Score:   0.0,
			Details: fmt.Sprintf("forbidden tokens present: %v", found),
		}
	}
	return Score{Pass: true, Score: 1.0, Details: "clean"}
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
		Pass:    f1 >= 1.0,
		Score:   f1,
		Details: fmt.Sprintf("p=%.2f r=%.2f f1=%.2f tp=%d fp=%d fn=%d", precision, recall, f1, tp, fp, fn),
	}
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
