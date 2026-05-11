package eval

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFixture(t *testing.T, dir string) {
	t.Helper()
	content := `{"id":"r-1","category":"classification","description":"x","alert":{"title":"t","severity":"P1","status":"firing","labels":{},"annotations":{},"description":""},"expected_severity":"P1"}
{"id":"r-2","category":"safety","description":"y","alert":{"title":"t","severity":"P3","status":"firing","labels":{},"annotations":{},"description":""},"must_not_contain":["bad"]}
{"id":"r-3","category":"summary","description":"z","alert":{"title":"t","severity":"P2","status":"firing","labels":{},"annotations":{},"description":""},"expected_keywords":["foo"]}
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "r.jsonl"), []byte(content), 0o600))
}

func TestRunner_AllPass(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir)

	r := NewRunner(RunConfig{FixturesDir: dir}, nil)
	res, err := r.Run(context.Background(),
		func(_ context.Context, _ TestCase) (string, error) { return "", nil },
		func(_ TestCase, _ string) Score { return Score{Pass: true, Score: 1.0} },
	)
	require.NoError(t, err)
	assert.Equal(t, 3, res.Total)
	assert.Equal(t, 3, res.Passed)
	assert.Equal(t, 0, res.Failed)
	assert.InDelta(t, 1.0, res.PassRate, 1e-9)
	assert.InDelta(t, 1.0, res.MeanScore, 1e-9)
}

func TestRunner_CategoryFilter(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir)

	r := NewRunner(RunConfig{
		FixturesDir: dir,
		Categories:  []Category{CategorySafety},
	}, nil)
	res, err := r.Run(context.Background(),
		func(_ context.Context, _ TestCase) (string, error) { return "", nil },
		func(_ TestCase, _ string) Score { return Score{Pass: true, Score: 1.0} },
	)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Total)
	assert.Equal(t, 2, res.Skipped)
}

func TestRunner_MaxCases(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir)

	r := NewRunner(RunConfig{FixturesDir: dir, MaxCases: 2}, nil)
	res, err := r.Run(context.Background(),
		func(_ context.Context, _ TestCase) (string, error) { return "", nil },
		func(_ TestCase, _ string) Score { return Score{Pass: true, Score: 1.0} },
	)
	require.NoError(t, err)
	assert.Equal(t, 2, res.Total)
	assert.Equal(t, 1, res.Skipped)
}

func TestRunner_RequiresScoreFn(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir)
	r := NewRunner(RunConfig{FixturesDir: dir}, nil)
	_, err := r.Run(context.Background(), nil, nil)
	require.Error(t, err)
}

func TestRunner_AggregatesByCategory(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir)
	r := NewRunner(RunConfig{FixturesDir: dir}, nil)
	res, err := r.Run(context.Background(),
		func(_ context.Context, _ TestCase) (string, error) { return "", nil },
		func(tc TestCase, _ string) Score {
			if tc.Category == CategorySafety {
				return Score{Pass: false, Score: 0.0}
			}
			return Score{Pass: true, Score: 1.0}
		},
	)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Failed)
	assert.Equal(t, 2, res.Passed)

	safety := res.ByCategory[CategorySafety]
	require.NotNil(t, safety)
	assert.Equal(t, 1, safety.Total)
	assert.Equal(t, 0, safety.Passed)
	assert.InDelta(t, 0.0, safety.PassRate, 1e-9)

	cls := res.ByCategory[CategoryClassification]
	require.NotNil(t, cls)
	assert.InDelta(t, 1.0, cls.PassRate, 1e-9)
	assert.InDelta(t, 1.0, cls.MeanScore, 1e-9)
}
