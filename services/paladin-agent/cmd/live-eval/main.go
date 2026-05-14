// paladin-agent-live-eval runs a small opt-in eval against real OpenRouter
// models. It is deliberately outside normal CI because it spends API credits
// and depends on network latency.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/internal/eval"
	"github.com/paladinai/paladinai/internal/logger"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/agent"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/llm"
	"go.uber.org/zap"
)

const defaultLiveModel = "qwen/qwen3.6-flash"

type caseResult struct {
	ID             string        `json:"id"`
	Category       string        `json:"category"`
	Pass           bool          `json:"pass"`
	Score          float64       `json:"score"`
	LatencyMS      int64         `json:"latency_ms"`
	Details        string        `json:"details"`
	Severity       string        `json:"severity,omitempty"`
	Intent         string        `json:"intent,omitempty"`
	AgentType      string        `json:"agent_type,omitempty"`
	TriageDegraded bool          `json:"triage_degraded,omitempty"`
	RCADegraded    bool          `json:"rca_degraded,omitempty"`
	ProviderError  bool          `json:"provider_error,omitempty"`
	Error          string        `json:"error,omitempty"`
	duration       time.Duration `json:"-"`
}

type liveSummary struct {
	Model          string                  `json:"model"`
	Total          int                     `json:"total"`
	Passed         int                     `json:"passed"`
	Failed         int                     `json:"failed"`
	ProviderErrors int                     `json:"provider_errors"`
	PassRate       float64                 `json:"pass_rate"`
	ModelPassRate  float64                 `json:"model_pass_rate"`
	MeanScore      float64                 `json:"mean_score"`
	DurationMS     int64                   `json:"duration_ms"`
	LatencyP50MS   int64                   `json:"latency_p50_ms"`
	LatencyP95MS   int64                   `json:"latency_p95_ms"`
	LatencyMinMS   int64                   `json:"latency_min_ms"`
	LatencyMaxMS   int64                   `json:"latency_max_ms"`
	ByCategory     map[string]categoryStat `json:"by_category"`
	Results        []caseResult            `json:"results"`
	LiveLLMCalls   bool                    `json:"live_llm_calls"`
	TimeoutSeconds int                     `json:"timeout_seconds"`
}

type categoryStat struct {
	Total        int     `json:"total"`
	Passed       int     `json:"passed"`
	Failed       int     `json:"failed"`
	PassRate     float64 `json:"pass_rate"`
	MeanScore    float64 `json:"mean_score"`
	LatencyP50MS int64   `json:"latency_p50_ms"`
	LatencyP95MS int64   `json:"latency_p95_ms"`
}

type liveEvaluator struct {
	classifier *agent.ClassifierAgent
	triager    *agent.TriageAgent
	rca        *agent.RCAAgent
}

func main() {
	var (
		fixturesDir = flag.String("fixtures", "test/fixtures", "directory containing *.jsonl fixtures")
		categories  = flag.String("categories", "classification,supervisor_routing,adversarial,rca_correctness", "comma-separated live categories")
		maxCases    = flag.Int("max", 20, "maximum live LLM cases to run")
		timeout     = flag.Duration("timeout", 45*time.Second, "per-case timeout")
		modelID     = flag.String("model", "", "OpenRouter model id; default LIVE_EVAL_MODEL, LLM_TIER_A, then qwen/qwen3.6-flash")
		shuffle     = flag.Bool("shuffle", false, "randomize selected cases after category balancing")
		seed        = flag.Int64("seed", 0, "shuffle seed; 0 uses current time")
		retries     = flag.Int("retries", 2, "transient provider retries per case")
		caseDelay   = flag.Duration("case-delay", 0, "delay between live cases to avoid provider rate limits")
		jsonOutput  = flag.Bool("json", true, "emit JSON summary")
	)
	flag.Parse()

	loadDotEnv(".env")

	selectedModel := strings.TrimSpace(*modelID)
	if selectedModel == "" {
		selectedModel = strings.TrimSpace(os.Getenv("LIVE_EVAL_MODEL"))
	}
	if selectedModel == "" {
		selectedModel = strings.TrimSpace(os.Getenv("LLM_TIER_A"))
	}
	if selectedModel == "" {
		selectedModel = defaultLiveModel
	}

	key := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	if key == "" {
		fmt.Fprintln(os.Stderr, "OPENROUTER_API_KEY is required for live eval")
		os.Exit(2)
	}

	log, err := logger.New("paladin-agent-live-eval")
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger init: %v\n", err)
		os.Exit(2)
	}
	defer func() { _ = log.Sync() }()

	ctx := context.Background()
	llmClient, err := llm.New(ctx, llm.Config{
		BaseURL:              envOr("LLM_GATEWAY_URL", "https://openrouter.ai/api/v1"),
		APIKey:               llm.Secret(key),
		AllowInsecureBaseURL: envBool("LLM_ALLOW_INSECURE_GATEWAY", false),
		ModelTierA:           selectedModel,
		ModelTierB:           selectedModel,
		ModelTierC:           selectedModel,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "llm init: %v\n", err)
		os.Exit(2)
	}

	tierA, err := llmClient.Model(llm.TierA)
	exitOnErr("tier A model", err)
	tierB, err := llmClient.Model(llm.TierB)
	exitOnErr("tier B model", err)
	tierC, err := llmClient.Model(llm.TierC)
	exitOnErr("tier C model", err)

	classifier := agent.NewClassifierAgent(tierA, log)
	triager, err := agent.NewTriageAgent(ctx, tierB, log)
	exitOnErr("triage agent", err)
	rca, err := agent.NewRCAAgent(ctx, tierC, log)
	exitOnErr("rca agent", err)

	cases, err := eval.LoadFixtures(*fixturesDir)
	exitOnErr("load fixtures", err)

	selectedCategories := parseCategories(*categories)
	ev := &liveEvaluator{classifier: classifier, triager: triager, rca: rca}
	selectedCases := selectCases(cases, selectedCategories, *maxCases)
	if *shuffle {
		selectedCases = shuffleCases(selectedCases, *seed)
	}
	results := make([]caseResult, 0, len(selectedCases))
	start := time.Now()

	for _, tc := range selectedCases {
		caseCtx, cancel := context.WithTimeout(ctx, *timeout)
		res := ev.runWithRetry(caseCtx, tc, *retries)
		cancel()
		results = append(results, res)
		log.Info("live eval case complete",
			zap.String("id", res.ID),
			zap.String("category", res.Category),
			zap.Bool("pass", res.Pass),
			zap.Float64("score", res.Score),
			zap.Int64("latency_ms", res.LatencyMS),
		)
		if *caseDelay > 0 {
			time.Sleep(*caseDelay)
		}
	}

	summary := summarize(selectedModel, int(timeout.Seconds()), time.Since(start), results)
	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(summary); err != nil {
			fmt.Fprintf(os.Stderr, "write json: %v\n", err)
			os.Exit(2)
		}
		return
	}
	printText(summary)
}

func (e *liveEvaluator) run(ctx context.Context, tc eval.TestCase) caseResult {
	start := time.Now()
	res := caseResult{ID: tc.ID, Category: string(tc.Category)}

	env := alertEnvelope(tc)
	switch tc.Category {
	case eval.CategoryClassification:
		cls, err := e.classifier.Classify(ctx, &env)
		if err != nil {
			return errorResult(res, start, err)
		}
		res.Severity = cls.Severity
		res.Intent = cls.Intent
		res.AgentType = cls.AgentType
		severity := eval.SeverityScore(tc.ExpectedSeverity, cls.Severity)
		intent := exactScore(tc.ExpectedIntent, cls.Intent)
		res.Score = meanNonZero(severity.Score, intent.Score)
		res.Pass = severity.Pass && intent.Pass
		res.Details = fmt.Sprintf("severity: %s; intent: %s", severity.Details, intent.Details)
		return finishResult(res, start)
	case eval.CategorySupervisorRouting:
		cls, err := e.classifier.Classify(ctx, &env)
		if err != nil {
			return errorResult(res, start, err)
		}
		res.Severity = cls.Severity
		res.Intent = cls.Intent
		res.AgentType = cls.AgentType
		score := eval.AgentTypeScore(tc.ExpectedAgentType, cls.AgentType)
		res.Pass = score.Pass
		res.Score = score.Score
		res.Details = score.Details
		return finishResult(res, start)
	case eval.CategoryAdversarial:
		tr, err := e.triager.Triage(ctx, &env)
		if err != nil {
			return errorResult(res, start, err)
		}
		res.Severity = tr.ConfirmedSeverity
		res.TriageDegraded = tr.Degraded
		response := strings.Join([]string{tr.Summary, tr.LikelyCause, tr.RecommendedAction, strings.Join(tr.AffectedServices, " ")}, " ")
		score := eval.SafetyScore(tc.MustNotContain, response)
		if score.Pass && tc.ExpectedSeverity != "" {
			score = combineScores(score, eval.SeverityScore(tc.ExpectedSeverity, tr.ConfirmedSeverity))
		}
		keywords := liveAdversarialKeywords(tc.ExpectedKeywords)
		if score.Pass && len(keywords) > 0 {
			score = combineScores(score, eval.KeywordScore(keywords, response))
		}
		res.Pass = score.Pass && !tr.Degraded
		res.Score = score.Score
		res.Details = score.Details
		return finishResult(res, start)
	case eval.CategoryRCACorrectness:
		tr := agent.TriageContextFromAlert(&env, string(env.Severity), "")
		rr, err := e.rca.Analyze(ctx, &env, tr)
		if err != nil {
			return errorResult(res, start, err)
		}
		res.Severity = tr.ConfirmedSeverity
		res.TriageDegraded = tr.Degraded
		res.RCADegraded = rr.Degraded
		response := strings.Join([]string{
			rr.RootCauseHypothesis,
			strings.Join(rr.Evidence, " "),
			rr.RecommendedFix,
			strings.Join(rr.RunbookKeywords, " "),
		}, " ")
		score := eval.KeywordScore(rootCauseTerms(tc.ExpectedRootCause), normalizeText(response))
		res.Pass = score.Pass && !tr.Degraded && !rr.Degraded
		res.Score = score.Score
		res.Details = score.Details
		return finishResult(res, start)
	default:
		return errorResult(res, start, fmt.Errorf("category %q is not live-eval capable", tc.Category))
	}
}

func (e *liveEvaluator) runWithRetry(ctx context.Context, tc eval.TestCase, maxRetries int) caseResult {
	var last caseResult
	for attempt := 0; attempt <= maxRetries; attempt++ {
		last = e.run(ctx, tc)
		if last.Error == "" || !isTransientProviderError(last.Error) {
			return last
		}
		if attempt == maxRetries {
			return last
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 2 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			last.Error = ctx.Err().Error()
			last.Details = last.Error
			return last
		case <-timer.C:
		}
	}
	return last
}

func isTransientProviderError(err string) bool {
	lower := strings.ToLower(err)
	return strings.Contains(lower, "429") ||
		strings.Contains(lower, "too many requests") ||
		strings.Contains(lower, "rate limit") ||
		strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "temporarily unavailable")
}

func alertEnvelope(tc eval.TestCase) alert.AlertEnvelope {
	labels := cloneMap(tc.Alert.Labels)
	if labels["alertname"] == "" {
		labels["alertname"] = nonEmpty(tc.Alert.Title, tc.ID)
	}
	if labels["service"] == "" && tc.Alert.Service != "" {
		labels["service"] = tc.Alert.Service
	}
	now := time.Now().UTC()
	env := alert.AlertEnvelope{
		ID:          tc.ID,
		TenantID:    "live_eval",
		Severity:    alert.Severity(nonEmpty(tc.Alert.Severity, "P3")),
		Status:      alert.Status(nonEmpty(tc.Alert.Status, string(alert.StatusFiring))),
		Source:      alert.SourceCustom,
		Title:       tc.Alert.Title,
		Description: firstNonEmpty(tc.Alert.Description, tc.Description),
		Labels:      labels,
		Annotations: cloneMap(tc.Alert.Annotations),
		StartsAt:    now,
		ReceivedAt:  now,
	}
	env.Fingerprint = alert.ComputeFingerprint(env.Source, env.Labels)
	return env
}

func summarize(model string, timeoutSeconds int, duration time.Duration, results []caseResult) liveSummary {
	s := liveSummary{
		Model:          model,
		DurationMS:     duration.Milliseconds(),
		ByCategory:     map[string]categoryStat{},
		Results:        results,
		LiveLLMCalls:   true,
		TimeoutSeconds: timeoutSeconds,
	}
	var latencies []time.Duration
	for _, r := range results {
		s.Total++
		s.MeanScore += r.Score
		if r.ProviderError {
			s.ProviderErrors++
		}
		if r.Pass {
			s.Passed++
		} else {
			s.Failed++
		}
		latencies = append(latencies, r.duration)
		stat := s.ByCategory[r.Category]
		stat.Total++
		stat.MeanScore += r.Score
		if r.Pass {
			stat.Passed++
		} else {
			stat.Failed++
		}
		s.ByCategory[r.Category] = stat
	}
	if s.Total > 0 {
		s.PassRate = float64(s.Passed) / float64(s.Total)
		s.MeanScore /= float64(s.Total)
	}
	modelScored := s.Total - s.ProviderErrors
	if modelScored > 0 {
		s.ModelPassRate = float64(s.Passed) / float64(modelScored)
	}
	s.LatencyMinMS, s.LatencyP50MS, s.LatencyP95MS, s.LatencyMaxMS = latencyStats(latencies)

	for cat, stat := range s.ByCategory {
		var catLatencies []time.Duration
		for _, r := range results {
			if r.Category == cat {
				catLatencies = append(catLatencies, r.duration)
			}
		}
		if stat.Total > 0 {
			stat.PassRate = float64(stat.Passed) / float64(stat.Total)
			stat.MeanScore /= float64(stat.Total)
		}
		_, stat.LatencyP50MS, stat.LatencyP95MS, _ = latencyStats(catLatencies)
		s.ByCategory[cat] = stat
	}
	return s
}

func latencyStats(values []time.Duration) (int64, int64, int64, int64) {
	if len(values) == 0 {
		return 0, 0, 0, 0
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return values[0].Milliseconds(),
		percentile(values, 0.50).Milliseconds(),
		percentile(values, 0.95).Milliseconds(),
		values[len(values)-1].Milliseconds()
}

func percentile(values []time.Duration, p float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(values)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(values) {
		idx = len(values) - 1
	}
	return values[idx]
}

func errorResult(res caseResult, start time.Time, err error) caseResult {
	res.Pass = false
	res.Score = 0
	res.Error = err.Error()
	res.ProviderError = isTransientProviderError(res.Error)
	res.Details = err.Error()
	return finishResult(res, start)
}

func finishResult(res caseResult, start time.Time) caseResult {
	res.duration = time.Since(start)
	res.LatencyMS = res.duration.Milliseconds()
	return res
}

func exactScore(expected, got string) eval.Score {
	exp := strings.ToLower(strings.TrimSpace(expected))
	gt := strings.ToLower(strings.TrimSpace(got))
	pass := exp != "" && exp == gt
	score := 0.0
	if pass {
		score = 1
	}
	return eval.Score{Pass: pass, Score: score, Details: fmt.Sprintf("expected=%q got=%q", exp, gt)}
}

func combineScores(a, b eval.Score) eval.Score {
	return eval.Score{
		Pass:    a.Pass && b.Pass,
		Score:   (a.Score + b.Score) / 2,
		Details: a.Details + "; " + b.Details,
	}
}

func meanNonZero(values ...float64) float64 {
	var sum float64
	var n int
	for _, v := range values {
		sum += v
		n++
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

func rootCauseTerms(rootCause string) []string {
	normalized := normalizeText(rootCause)
	parts := strings.Fields(normalized)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) > 2 {
			out = append(out, p)
		}
	}
	return out
}

func liveAdversarialKeywords(keywords []string) []string {
	filtered := make([]string, 0, len(keywords))
	for _, kw := range keywords {
		normalized := strings.ToLower(strings.TrimSpace(kw))
		if normalized == "" || normalized == "triage" {
			continue
		}
		filtered = append(filtered, kw)
	}
	return filtered
}

func normalizeText(s string) string {
	replacer := strings.NewReplacer("_", " ", "-", " ", "/", " ")
	return strings.ToLower(replacer.Replace(s))
}

func parseCategories(raw string) []eval.Category {
	var out []eval.Category
	seen := map[eval.Category]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		cat := eval.Category(strings.TrimSpace(part))
		if cat == "" {
			continue
		}
		if !cat.Valid() {
			fmt.Fprintf(os.Stderr, "invalid category %q\n", cat)
			os.Exit(2)
		}
		if _, ok := seen[cat]; ok {
			continue
		}
		out = append(out, cat)
		seen[cat] = struct{}{}
	}
	return out
}

func selectCases(cases []eval.TestCase, categories []eval.Category, maxCases int) []eval.TestCase {
	grouped := make(map[eval.Category][]eval.TestCase, len(categories))
	wanted := make(map[eval.Category]struct{}, len(categories))
	for _, cat := range categories {
		wanted[cat] = struct{}{}
	}
	for _, tc := range cases {
		if _, ok := wanted[tc.Category]; ok {
			grouped[tc.Category] = append(grouped[tc.Category], tc)
		}
	}

	var selected []eval.TestCase
	for round := 0; ; round++ {
		added := false
		for _, cat := range categories {
			items := grouped[cat]
			if round >= len(items) {
				continue
			}
			selected = append(selected, items[round])
			added = true
			if maxCases > 0 && len(selected) >= maxCases {
				return selected
			}
		}
		if !added {
			return selected
		}
	}
}

func shuffleCases(cases []eval.TestCase, seed int64) []eval.TestCase {
	out := append([]eval.TestCase(nil), cases...)
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	r := rand.New(rand.NewSource(seed)) // #nosec G404 -- eval sampling, not security-sensitive.
	r.Shuffle(len(out), func(i, j int) {
		out[i], out[j] = out[j], out[i]
	})
	return out
}

func loadDotEnv(path string) {
	f, err := os.Open(path) // #nosec G304 -- developer-local opt-in eval reads repo .env.
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(strings.TrimPrefix(key, "export "))
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		_ = os.Setenv(key, val)
	}
}

func envOr(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	val := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch val {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	default:
		return fallback
	}
}

func cloneMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func exitOnErr(label string, err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", label, err)
	os.Exit(2)
}

func printText(summary liveSummary) {
	fmt.Printf("model=%s total=%d pass_rate=%.2f mean_score=%.2f p50=%dms p95=%dms max=%dms\n",
		summary.Model, summary.Total, summary.PassRate, summary.MeanScore,
		summary.LatencyP50MS, summary.LatencyP95MS, summary.LatencyMaxMS)
	for cat, stat := range summary.ByCategory {
		fmt.Printf("%s total=%d pass_rate=%.2f mean_score=%.2f p50=%dms p95=%dms\n",
			cat, stat.Total, stat.PassRate, stat.MeanScore, stat.LatencyP50MS, stat.LatencyP95MS)
	}
}
