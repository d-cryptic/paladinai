package eval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeJSONL(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func TestLoadFixtures_Empty(t *testing.T) {
	dir := t.TempDir()
	cases, err := LoadFixtures(dir)
	require.NoError(t, err)
	assert.Empty(t, cases)
}

func TestLoadFixtures_MissingDir(t *testing.T) {
	_, err := LoadFixtures("/does/not/exist/" + t.Name())
	assert.Error(t, err)
}

func TestLoadFixtures_ParsesValidJSONL(t *testing.T) {
	dir := t.TempDir()
	content := `{"id":"a-1","category":"classification","description":"x","alert":{"title":"t","severity":"P1","status":"firing","labels":{},"annotations":{},"description":""},"expected_severity":"P1"}
{"id":"a-2","category":"safety","description":"y","alert":{"title":"t2","severity":"P3","status":"firing","labels":{},"annotations":{},"description":""},"must_not_contain":["bad"]}
`
	writeJSONL(t, dir, "a.jsonl", content)

	cases, err := LoadFixtures(dir)
	require.NoError(t, err)
	require.Len(t, cases, 2)
	assert.Equal(t, "a-1", cases[0].ID)
	assert.Equal(t, CategorySafety, cases[1].Category)
}

func TestLoadFixtures_SkipsBlankAndComments(t *testing.T) {
	dir := t.TempDir()
	content := `
// a comment
{"id":"a-1","category":"classification","description":"x","alert":{"title":"t","severity":"P1","status":"firing","labels":{},"annotations":{},"description":""},"expected_severity":"P1"}

`
	writeJSONL(t, dir, "a.jsonl", content)
	cases, err := LoadFixtures(dir)
	require.NoError(t, err)
	require.Len(t, cases, 1)
}

func TestLoadFixtures_DetectsDuplicateIDs(t *testing.T) {
	dir := t.TempDir()
	content := `{"id":"dup","category":"classification","description":"x","alert":{"title":"t","severity":"P1","status":"firing","labels":{},"annotations":{},"description":""},"expected_severity":"P1"}
{"id":"dup","category":"classification","description":"x","alert":{"title":"t","severity":"P1","status":"firing","labels":{},"annotations":{},"description":""},"expected_severity":"P1"}
`
	writeJSONL(t, dir, "a.jsonl", content)
	_, err := LoadFixtures(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestLoadFixtures_InvalidCategory(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "a.jsonl",
		`{"id":"x","category":"unknown","description":"x","alert":{"title":"t","severity":"P1","status":"firing","labels":{},"annotations":{},"description":""},"expected_severity":"P1"}`)
	_, err := LoadFixtures(dir)
	require.Error(t, err)
}

func TestLoadFixtures_NoExpectations(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "a.jsonl",
		`{"id":"x","category":"classification","description":"x","alert":{"title":"t","severity":"P1","status":"firing","labels":{},"annotations":{},"description":""}}`)
	_, err := LoadFixtures(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected outputs")
}

func TestLoadFixtures_RealFixtures(t *testing.T) {
	cases, err := LoadFixtures("../../test/fixtures")
	require.NoError(t, err)
	assert.NotEmpty(t, cases)

	cats := make(map[Category]int)
	for _, c := range cases {
		cats[c.Category]++
	}
	assert.Greater(t, cats[CategoryClassification], 0)
	assert.Greater(t, cats[CategorySafety], 0)
	assert.Greater(t, cats[CategorySummary], 0)
	assert.Greater(t, cats[CategoryAdversarial], 0)
	assert.GreaterOrEqual(t, cats[CategorySupervisorRouting], 200, "expect 200+ supervisor routing cases")
	assert.GreaterOrEqual(t, cats[CategoryCostRegression], 50, "expect 50+ cost regression cases")
	assert.GreaterOrEqual(t, cats[CategoryLatencyBudget], 50, "expect 50+ latency budget cases")
	assert.GreaterOrEqual(t, cats[CategoryMemoryRecall], 150, "expect 150+ memory recall cases")
}

func TestLoadFixtures_SupervisorRoutingCategory(t *testing.T) {
	dir := t.TempDir()
	content := `{"id":"sup-001","category":"supervisor_routing","description":"[prod/us-east-1] DB down","alert":{"title":"PostgreSQL Primary Down","severity":"P1","status":"firing","labels":{},"annotations":{}},"expected_agent_type":"triage","expected_intent":"service_down","expected_severity":"P1"}` + "\n"
	writeJSONL(t, dir, "supervisor.jsonl", content)

	cases, err := LoadFixtures(dir)
	require.NoError(t, err)
	require.Len(t, cases, 1)
	assert.Equal(t, CategorySupervisorRouting, cases[0].Category)
	assert.Equal(t, "triage", cases[0].ExpectedAgentType)
}

func TestLoadFixtures_CostRegressionCategory(t *testing.T) {
	dir := t.TempDir()
	content := `{"id":"cost-001","category":"cost_regression","description":"token budget","baseline_tokens":1000,"observed_tokens":1080}` + "\n"
	writeJSONL(t, dir, "cost.jsonl", content)

	cases, err := LoadFixtures(dir)
	require.NoError(t, err)
	require.Len(t, cases, 1)
	assert.Equal(t, CategoryCostRegression, cases[0].Category)
	assert.Equal(t, 1000, cases[0].BaselineTokens)
	assert.Equal(t, 1080, cases[0].ObservedTokens)
}

func TestLoadFixtures_LatencyBudgetCategory(t *testing.T) {
	dir := t.TempDir()
	content := `{"id":"lat-001","category":"latency_budget","description":"latency budget","latency_budget_ms":5000,"observed_latency_ms":4200}` + "\n"
	writeJSONL(t, dir, "latency.jsonl", content)

	cases, err := LoadFixtures(dir)
	require.NoError(t, err)
	require.Len(t, cases, 1)
	assert.Equal(t, CategoryLatencyBudget, cases[0].Category)
	assert.Equal(t, 5000, cases[0].LatencyBudgetMS)
	assert.Equal(t, 4200, cases[0].ObservedLatencyMS)
}

func TestLoadFixtures_CostRegressionRequiresBaseline(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "cost.jsonl",
		`{"id":"cost-001","category":"cost_regression","description":"token budget","observed_tokens":1080}`)

	_, err := LoadFixtures(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "baseline_tokens")
}

func TestLoadFixtures_MemoryRecallCategory(t *testing.T) {
	dir := t.TempDir()
	content := `{"id":"mem-001","category":"memory_recall","description":"memory recall","expected_incident_ids":["inc-1","inc-2"],"recalled_incident_ids":["inc-1","inc-2","inc-x"]}` + "\n"
	writeJSONL(t, dir, "memory.jsonl", content)

	cases, err := LoadFixtures(dir)
	require.NoError(t, err)
	require.Len(t, cases, 1)
	assert.Equal(t, CategoryMemoryRecall, cases[0].Category)
	assert.Equal(t, []string{"inc-1", "inc-2"}, cases[0].ExpectedIncidentIDs)
}

func TestLoadFixtures_MemoryRecallRequiresExpectedIncidents(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, dir, "memory.jsonl",
		`{"id":"mem-001","category":"memory_recall","description":"memory recall","recalled_incident_ids":["inc-1"]}`)

	_, err := LoadFixtures(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected_incident_ids")
}
