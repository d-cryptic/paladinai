// Package eval provides a model-agnostic evaluation framework for PaladinAI
// agents. Test cases are stored as JSONL fixtures and scored by category.
package eval

// Category is the eval category — affects scoring weights.
type Category string

const (
	// CategoryClassification covers severity and intent routing tests.
	CategoryClassification Category = "classification"
	// CategoryToolUse covers tool selection accuracy.
	CategoryToolUse Category = "tool_use"
	// CategorySummary covers alert summary quality.
	CategorySummary Category = "summary"
	// CategorySafety covers prompt injection / jailbreak resistance.
	CategorySafety Category = "safety"
	// CategoryAdversarial covers edge cases and malformed inputs.
	CategoryAdversarial Category = "adversarial"
)

// Valid returns true if c is a recognised category.
func (c Category) Valid() bool {
	switch c {
	case CategoryClassification, CategoryToolUse, CategorySummary,
		CategorySafety, CategoryAdversarial:
		return true
	}
	return false
}

// AlertInput is the alert portion of a test case.
type AlertInput struct {
	Title       string            `json:"title"`
	Severity    string            `json:"severity"` // P1/P2/P3/P4
	Status      string            `json:"status"`   // firing/resolved
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Description string            `json:"description"`
}

// TestCase is one eval entry from a JSONL file.
type TestCase struct {
	ID          string     `json:"id"`
	Category    Category   `json:"category"`
	Description string     `json:"description"`
	Alert       AlertInput `json:"alert"`

	// Expected outputs (at least one must be set).
	ExpectedSeverity  string   `json:"expected_severity,omitempty"`
	ExpectedIntent    string   `json:"expected_intent,omitempty"`
	ExpectedToolNames []string `json:"expected_tool_names,omitempty"`
	ExpectedKeywords  []string `json:"expected_keywords,omitempty"`
	MustNotContain    []string `json:"must_not_contain,omitempty"`
}

// HasExpectations returns true if the case has at least one expected output
// declared. Used by the loader to validate fixtures.
func (tc TestCase) HasExpectations() bool {
	return tc.ExpectedSeverity != "" ||
		tc.ExpectedIntent != "" ||
		len(tc.ExpectedToolNames) > 0 ||
		len(tc.ExpectedKeywords) > 0 ||
		len(tc.MustNotContain) > 0
}
