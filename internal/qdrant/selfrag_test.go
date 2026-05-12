package qdrant

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// ── fake gateway ──────────────────────────────────────────────────────────────

type fakeGateway struct {
	resp *CompletionResponse
	err  error
}

func (f *fakeGateway) Complete(_ context.Context, _ *CompletionRequest) (*CompletionResponse, error) {
	return f.resp, f.err
}

// ── helpers ───────────────────────────────────────────────────────────────────

func makeCandidate(id, text string) Candidate {
	return Candidate{
		ID:      id,
		Score:   1.0,
		Payload: map[string]any{"raw_text": text},
	}
}

// ── ReflectionScore.Passes ────────────────────────────────────────────────────

func TestReflectionScore_Passes(t *testing.T) {
	tests := []struct {
		name string
		s    ReflectionScore
		want bool
	}{
		{"all true", ReflectionScore{IsRelevant: true, IsSupported: true, IsUseful: true}, true},
		{"rel false", ReflectionScore{IsRelevant: false, IsSupported: true, IsUseful: true}, false},
		{"sup false", ReflectionScore{IsRelevant: true, IsSupported: false, IsUseful: true}, false},
		{"use false", ReflectionScore{IsRelevant: true, IsSupported: true, IsUseful: false}, false},
		{"all false", ReflectionScore{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.s.Passes(); got != tc.want {
				t.Errorf("Passes() = %v, want %v", got, tc.want)
			}
		})
	}
}

// ── ScoreChunks ───────────────────────────────────────────────────────────────

func TestScoreChunks_EmptyInput(t *testing.T) {
	r := NewSelfRAGReflector(&fakeGateway{resp: &CompletionResponse{}})
	passing, scores, err := r.ScoreChunks(context.Background(), "query", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(passing) != 0 || len(scores) != 0 {
		t.Errorf("want empty results, got passing=%d scores=%d", len(passing), len(scores))
	}
}

func TestScoreChunks_GatewayError_FailOpen(t *testing.T) {
	r := NewSelfRAGReflector(&fakeGateway{err: errors.New("gateway down")})
	candidates := []Candidate{
		makeCandidate("a", "text A"),
		makeCandidate("b", "text B"),
	}
	passing, scores, err := r.ScoreChunks(context.Background(), "q", candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(passing) != 2 {
		t.Errorf("fail-open: want 2 passing, got %d", len(passing))
	}
	for _, s := range scores {
		if !s.Passes() {
			t.Error("fail-open: all scores should pass")
		}
	}
}

func TestScoreChunks_AllPass(t *testing.T) {
	resp := `CHUNK_1: IsREL=true IsSUP=true IsUSE=true Reason=directly relevant
CHUNK_2: IsREL=true IsSUP=true IsUSE=true Reason=also useful`

	r := NewSelfRAGReflector(&fakeGateway{resp: &CompletionResponse{Text: resp}})
	candidates := []Candidate{
		makeCandidate("id1", "runbook for postgres"),
		makeCandidate("id2", "failover procedure"),
	}
	passing, scores, err := r.ScoreChunks(context.Background(), "postgres failover", candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(passing) != 2 {
		t.Errorf("want 2 passing, got %d", len(passing))
	}
	if len(scores) != 2 {
		t.Errorf("want 2 scores, got %d", len(scores))
	}
	if scores[0].PointID != "id1" || scores[1].PointID != "id2" {
		t.Error("scores point IDs should match candidate IDs")
	}
}

func TestScoreChunks_OneFiltered(t *testing.T) {
	resp := `CHUNK_1: IsREL=true IsSUP=true IsUSE=true Reason=good
CHUNK_2: IsREL=false IsSUP=true IsUSE=true Reason=unrelated to query`

	r := NewSelfRAGReflector(&fakeGateway{resp: &CompletionResponse{Text: resp}})
	candidates := []Candidate{
		makeCandidate("good", "relevant runbook"),
		makeCandidate("bad", "unrelated auth documentation"),
	}
	passing, scores, err := r.ScoreChunks(context.Background(), "postgres failover", candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(passing) != 1 {
		t.Errorf("want 1 passing, got %d", len(passing))
	}
	if passing[0].ID != "good" {
		t.Errorf("want passing[0].ID=good, got %s", passing[0].ID)
	}
	if len(scores) != 2 {
		t.Errorf("want 2 scores (all), got %d", len(scores))
	}
	if scores[1].IsRelevant {
		t.Error("second score should have IsRelevant=false")
	}
}

func TestScoreChunks_AllFiltered(t *testing.T) {
	resp := `CHUNK_1: IsREL=false IsSUP=false IsUSE=false Reason=totally unrelated`
	r := NewSelfRAGReflector(&fakeGateway{resp: &CompletionResponse{Text: resp}})
	candidates := []Candidate{makeCandidate("x", "irrelevant")}
	passing, _, err := r.ScoreChunks(context.Background(), "q", candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(passing) != 0 {
		t.Errorf("want 0 passing when all filtered, got %d", len(passing))
	}
}

func TestScoreChunks_CaseInsensitiveBoolean(t *testing.T) {
	resp := `CHUNK_1: IsREL=TRUE IsSUP=False IsUSE=TRUE Reason=partial`
	r := NewSelfRAGReflector(&fakeGateway{resp: &CompletionResponse{Text: resp}})
	candidates := []Candidate{makeCandidate("c1", "some text")}
	_, scores, err := r.ScoreChunks(context.Background(), "q", candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !scores[0].IsRelevant {
		t.Error("want IsRelevant=true")
	}
	if scores[0].IsSupported {
		t.Error("want IsSupported=false")
	}
}

// ── buildReflectionPrompt ─────────────────────────────────────────────────────

func TestBuildReflectionPrompt_ContainsQuery(t *testing.T) {
	query := "postgres primary failover procedure"
	candidates := []Candidate{makeCandidate("a", "failover steps")}
	prompt := buildReflectionPrompt(query, candidates)
	if !strings.Contains(prompt, query) {
		t.Errorf("prompt should contain query %q", query)
	}
	if !strings.Contains(prompt, "CHUNK_1") {
		t.Error("prompt should contain CHUNK_1 label")
	}
	if !strings.Contains(prompt, "failover steps") {
		t.Error("prompt should contain chunk text")
	}
}

func TestBuildReflectionPrompt_TruncatesLongText(t *testing.T) {
	longText := strings.Repeat("a", 1000)
	candidates := []Candidate{makeCandidate("x", longText)}
	prompt := buildReflectionPrompt("q", candidates)
	// After truncation, raw text should appear truncated with "..."
	if !strings.Contains(prompt, "...") {
		t.Error("long text should be truncated with ...")
	}
	// Prompt should not include all 1000 'a's.
	if strings.Count(prompt, "a") >= 1000 {
		t.Error("prompt should truncate long chunk text")
	}
}

func TestBuildReflectionPrompt_MissingRawText(t *testing.T) {
	// Candidate with no raw_text payload.
	c := Candidate{ID: "empty", Score: 1.0, Payload: map[string]any{}}
	prompt := buildReflectionPrompt("q", []Candidate{c})
	if !strings.Contains(prompt, "no text payload") {
		t.Error("should include placeholder for missing raw_text")
	}
}

// ── parseReflectionScores ─────────────────────────────────────────────────────

func TestParseReflectionScores_MissingLine_FallsBackToAllTrue(t *testing.T) {
	response := "" // no CHUNK lines
	candidates := []Candidate{makeCandidate("a", "text")}
	scores := parseReflectionScores(response, candidates)
	if len(scores) != 1 {
		t.Fatalf("want 1 score, got %d", len(scores))
	}
	if !scores[0].Passes() {
		t.Error("missing line: fallback should be all-true (fail-open per chunk)")
	}
}

func TestParseReflectionScores_OutOfRangeIndex(t *testing.T) {
	// CHUNK_5 but only 2 candidates — should be ignored.
	response := "CHUNK_5: IsREL=false IsSUP=false IsUSE=false Reason=oob"
	candidates := []Candidate{
		makeCandidate("a", "a"),
		makeCandidate("b", "b"),
	}
	scores := parseReflectionScores(response, candidates)
	for i, s := range scores {
		if !s.Passes() {
			t.Errorf("score[%d] should fall back to all-true, got %+v", i, s)
		}
	}
}

func TestParseReflectionScores_ReasonPreserved(t *testing.T) {
	response := "CHUNK_1: IsREL=true IsSUP=true IsUSE=false Reason=duplicate of chunk 2"
	candidates := []Candidate{makeCandidate("x", "text")}
	scores := parseReflectionScores(response, candidates)
	if scores[0].Reason != "duplicate of chunk 2" {
		t.Errorf("want reason %q, got %q", "duplicate of chunk 2", scores[0].Reason)
	}
}
