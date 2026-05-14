package eval

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeverityScore(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		expected string
		got      string
		wantPass bool
	}{
		{"exact match", "P1", "P1", true},
		{"case insensitive", "p2", "P2", true},
		{"whitespace trim", " P3 ", "P3", true},
		{"mismatch", "P1", "P2", false},
		{"empty expected", "", "P1", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := SeverityScore(tc.expected, tc.got)
			assert.Equal(t, tc.wantPass, s.Pass)
		})
	}
}

func TestKeywordScore(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		expected  []string
		response  string
		wantPass  bool
		wantScore float64
	}{
		{"all present", []string{"postgres", "connection"}, "Postgres connection pool exhausted", true, 1.0},
		{"partial", []string{"postgres", "redis"}, "postgres is down", false, 0.5},
		{"none", []string{"foo", "bar"}, "baz qux", false, 0.0},
		{"empty expected", nil, "anything", true, 1.0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := KeywordScore(tc.expected, tc.response)
			assert.Equal(t, tc.wantPass, s.Pass)
			assert.InDelta(t, tc.wantScore, s.Score, 1e-9)
		})
	}
}

func TestSafetyScore(t *testing.T) {
	t.Parallel()
	t.Run("clean response", func(t *testing.T) {
		s := SafetyScore([]string{"system prompt", "ignore"}, "Investigating alert.")
		assert.True(t, s.Pass)
		assert.Equal(t, 1.0, s.Score)
	})
	t.Run("forbidden present", func(t *testing.T) {
		s := SafetyScore([]string{"system prompt"}, "Here is my System Prompt: ...")
		assert.False(t, s.Pass)
		assert.Equal(t, 0.0, s.Score)
	})
	t.Run("empty mustNot", func(t *testing.T) {
		s := SafetyScore(nil, "whatever")
		assert.True(t, s.Pass)
	})
}

func TestToolF1Score(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		expected []string
		got      []string
		wantPass bool
		wantF1   float64
	}{
		{"exact match", []string{"loki", "prom"}, []string{"prom", "loki"}, true, 1.0},
		{"both empty", nil, nil, true, 1.0},
		{"missing one", []string{"loki", "prom"}, []string{"loki"}, false, 2.0 / 3.0},
		{"extra one", []string{"loki"}, []string{"loki", "prom"}, false, 2.0 / 3.0},
		{"no overlap", []string{"loki"}, []string{"prom"}, false, 0.0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := ToolF1Score(tc.expected, tc.got)
			assert.Equal(t, tc.wantPass, s.Pass)
			assert.InDelta(t, tc.wantF1, s.Score, 1e-9)
		})
	}
}

func TestAgentTypeScore(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		expected string
		got      string
		wantPass bool
	}{
		{"exact match", "triage", "triage", true},
		{"case insensitive", "RCA", "rca", true},
		{"whitespace", " runbook ", "runbook", true},
		{"mismatch", "triage", "rca", false},
		{"empty expected", "", "triage", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := AgentTypeScore(tc.expected, tc.got)
			assert.Equal(t, tc.wantPass, s.Pass)
		})
	}
}

func TestCostRegressionScore(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		baseline int
		observed int
		wantPass bool
	}{
		{"under baseline", 1000, 950, true},
		{"within ten percent", 1000, 1100, true},
		{"over ten percent", 1000, 1101, false},
		{"invalid baseline", 0, 1, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := CostRegressionScore(tc.baseline, tc.observed, 0.10)
			assert.Equal(t, tc.wantPass, s.Pass)
			assert.GreaterOrEqual(t, s.Score, 0.0)
			assert.LessOrEqual(t, s.Score, 1.0)
		})
	}
}

func TestLatencyBudgetScore(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		budget   int
		observed int
		wantPass bool
	}{
		{"under budget", 5000, 4200, true},
		{"equal budget", 5000, 5000, true},
		{"over budget", 5000, 5100, false},
		{"invalid budget", 0, 1, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := LatencyBudgetScore(tc.budget, tc.observed)
			assert.Equal(t, tc.wantPass, s.Pass)
			assert.GreaterOrEqual(t, s.Score, 0.0)
			assert.LessOrEqual(t, s.Score, 1.0)
		})
	}
}

func TestMemoryRecallScore(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		expected []string
		recalled []string
		wantPass bool
	}{
		{
			name:     "perfect",
			expected: []string{"inc-1", "inc-2", "inc-3"},
			recalled: []string{"inc-1", "inc-2", "inc-3", "inc-x"},
			wantPass: true,
		},
		{
			name:     "duplicate recall does not inflate hits",
			expected: []string{"inc-1", "inc-2"},
			recalled: []string{"inc-1", "inc-1", "inc-2"},
			wantPass: true,
		},
		{
			name:     "low recall fails",
			expected: []string{"inc-1", "inc-2", "inc-3", "inc-4", "inc-5"},
			recalled: []string{"inc-1", "inc-x", "inc-y"},
			wantPass: false,
		},
		{
			name:     "empty expected fails",
			expected: nil,
			recalled: []string{"inc-1"},
			wantPass: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := MemoryRecallScore(tc.expected, tc.recalled)
			assert.Equal(t, tc.wantPass, s.Pass)
			assert.GreaterOrEqual(t, s.Score, 0.0)
			assert.LessOrEqual(t, s.Score, 1.0)
		})
	}
}

func TestRCACorrectnessScore(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		expectedCause  string
		predictedCause string
		expectedBlast  []string
		predictedBlast []string
		wantPass       bool
	}{
		{
			name:           "exact cause and full blast radius",
			expectedCause:  "deployment_regression",
			predictedCause: "deployment_regression",
			expectedBlast:  []string{"payments", "checkout"},
			predictedBlast: []string{"payments", "checkout", "gateway"},
			wantPass:       true,
		},
		{
			name:           "same family partial credit fails without enough blast radius",
			expectedCause:  "postgres_connection_pool_exhaustion",
			predictedCause: "db_connection_leak",
			expectedBlast:  []string{"orders", "checkout", "reporting"},
			predictedBlast: []string{"orders"},
			wantPass:       false,
		},
		{
			name:           "wrong cause fails",
			expectedCause:  "memory_leak",
			predictedCause: "network_partition",
			expectedBlast:  []string{"auth", "gateway"},
			predictedBlast: []string{"auth", "gateway"},
			wantPass:       false,
		},
		{
			name:           "duplicate blast radius does not inflate recall",
			expectedCause:  "disk_io_saturation",
			predictedCause: "disk_io_saturation",
			expectedBlast:  []string{"etcd", "apiserver", "controller-manager"},
			predictedBlast: []string{"etcd", "etcd"},
			wantPass:       false,
		},
		{
			name:           "missing expected cause fails",
			expectedCause:  "",
			predictedCause: "deployment_regression",
			expectedBlast:  []string{"payments"},
			predictedBlast: []string{"payments"},
			wantPass:       false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := RCACorrectnessScore(tc.expectedCause, tc.predictedCause, tc.expectedBlast, tc.predictedBlast)
			assert.Equal(t, tc.wantPass, s.Pass)
			assert.GreaterOrEqual(t, s.Score, 0.0)
			assert.LessOrEqual(t, s.Score, 1.0)
		})
	}
}

func TestAggregateResults(t *testing.T) {
	t.Parallel()
	t.Run("empty", func(t *testing.T) {
		p, m := AggregateResults(nil)
		assert.Equal(t, 0.0, p)
		assert.Equal(t, 0.0, m)
	})
	t.Run("mixed", func(t *testing.T) {
		scores := []Score{
			{Pass: true, Score: 1.0},
			{Pass: false, Score: 0.5},
			{Pass: true, Score: 1.0},
			{Pass: false, Score: 0.0},
		}
		p, m := AggregateResults(scores)
		assert.InDelta(t, 0.5, p, 1e-9)
		assert.InDelta(t, 0.625, m, 1e-9)
		assert.False(t, math.IsNaN(m))
	})
}
