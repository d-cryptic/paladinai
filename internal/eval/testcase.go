// Package eval provides a model-agnostic evaluation framework for PaladinAI
// agents. Test cases are stored as JSONL fixtures and scored by category.
package eval

// Category is the eval category — affects scoring weights.
type Category string

const (
	// CategoryClassification covers severity and intent routing tests.
	CategoryClassification Category = "classification"
	// CategorySupervisorRouting covers supervisor agent-type routing tests.
	CategorySupervisorRouting Category = "supervisor_routing"
	// CategoryToolUse covers tool selection accuracy.
	CategoryToolUse Category = "tool_use"
	// CategorySummary covers alert summary quality.
	CategorySummary Category = "summary"
	// CategorySafety covers prompt injection / jailbreak resistance.
	CategorySafety Category = "safety"
	// CategoryAdversarial covers edge cases and malformed inputs.
	CategoryAdversarial Category = "adversarial"
	// CategoryCostRegression covers token-budget regressions against stable baselines.
	CategoryCostRegression Category = "cost_regression"
	// CategoryLatencyBudget covers per-case latency budget regressions.
	CategoryLatencyBudget Category = "latency_budget"
)

// Valid returns true if c is a recognised category.
func (c Category) Valid() bool {
	switch c {
	case CategoryClassification, CategorySupervisorRouting, CategoryToolUse,
		CategorySummary, CategorySafety, CategoryAdversarial,
		CategoryCostRegression, CategoryLatencyBudget:
		return true
	}
	return false
}

// AlertInput is the alert portion of a test case.
type AlertInput struct {
	Title       string            `json:"title"`
	Severity    string            `json:"severity"`          // P1/P2/P3/P4
	Status      string            `json:"status"`            // firing/resolved
	Service     string            `json:"service,omitempty"` // shorthand used in summary contexts
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Description string            `json:"description"`
}

// SummaryContext is the incident context for summary test cases.
type SummaryContext struct {
	IncidentID       string       `json:"incident_id"`
	Severity         string       `json:"severity"`
	Alerts           []AlertInput `json:"alerts"`
	DurationMinutes  int          `json:"duration_minutes"`
	AffectedServices []string     `json:"affected_services"`
}

// TestCase is one eval entry from a JSONL file.
type TestCase struct {
	ID          string         `json:"id"`
	Category    Category       `json:"category"`
	Description string         `json:"description"`
	Alert       AlertInput     `json:"alert"`
	Context     SummaryContext `json:"context,omitempty"`

	// Expected outputs (at least one must be set).
	ExpectedSeverity  string   `json:"expected_severity,omitempty"`
	ExpectedIntent    string   `json:"expected_intent,omitempty"`
	ExpectedAgentType string   `json:"expected_agent_type,omitempty"` // supervisor_routing fixtures
	ExpectedTools     []string `json:"expected_tools,omitempty"`      // tool_use fixtures
	ExpectedToolNames []string `json:"expected_tool_names,omitempty"` // legacy alias
	ExpectedKeywords  []string `json:"expected_keywords,omitempty"`
	MustNotContain    []string `json:"must_not_contain,omitempty"`

	// Regression budgets for Stage 10 CI gates.
	BaselineTokens    int `json:"baseline_tokens,omitempty"`
	ObservedTokens    int `json:"observed_tokens,omitempty"`
	LatencyBudgetMS   int `json:"latency_budget_ms,omitempty"`
	ObservedLatencyMS int `json:"observed_latency_ms,omitempty"`
}

// AllExpectedTools returns expected tools from either field (new or legacy).
func (tc TestCase) AllExpectedTools() []string {
	if len(tc.ExpectedTools) > 0 {
		return tc.ExpectedTools
	}
	return tc.ExpectedToolNames
}

// HasExpectations returns true if the case has at least one expected output
// declared. Used by the loader to validate fixtures.
func (tc TestCase) HasExpectations() bool {
	return tc.ExpectedSeverity != "" ||
		tc.ExpectedIntent != "" ||
		tc.ExpectedAgentType != "" ||
		len(tc.AllExpectedTools()) > 0 ||
		len(tc.ExpectedKeywords) > 0 ||
		len(tc.MustNotContain) > 0 ||
		(tc.Category == CategoryCostRegression && tc.BaselineTokens > 0) ||
		(tc.Category == CategoryLatencyBudget && tc.LatencyBudgetMS > 0)
}
