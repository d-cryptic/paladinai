package qdrant

import (
	"context"
	"errors"
	"testing"
)

func TestEvaluate_HighScore_ReturnsUse(t *testing.T) {
	t.Parallel()

	fake := &fakeGateway{
		resp: &CompletionResponse{Text: "SCORE: 0.85\nREASONING: highly relevant"},
	}
	c := NewCRAGEvaluator(fake)

	got, err := c.Evaluate(context.Background(), "q", []Candidate{makeCandidate("1", "text")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Decision != CRAGUse {
		t.Errorf("Decision = %v, want CRAGUse", got.Decision)
	}
	if got.Score < 0.84 || got.Score > 0.86 {
		t.Errorf("Score = %v, want ~0.85", got.Score)
	}
	if got.Reasoning != "highly relevant" {
		t.Errorf("Reasoning = %q, want %q", got.Reasoning, "highly relevant")
	}
}

func TestEvaluate_MidScore_ReturnsSupplement(t *testing.T) {
	t.Parallel()

	fake := &fakeGateway{
		resp: &CompletionResponse{Text: "SCORE: 0.55\nREASONING: partially relevant"},
	}
	c := NewCRAGEvaluator(fake)

	got, err := c.Evaluate(context.Background(), "q", []Candidate{makeCandidate("1", "text")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Decision != CRAGSupplement {
		t.Errorf("Decision = %v, want CRAGSupplement", got.Decision)
	}
}

func TestEvaluate_LowScore_ReturnsReplace(t *testing.T) {
	t.Parallel()

	fake := &fakeGateway{
		resp: &CompletionResponse{Text: "SCORE: 0.15\nREASONING: not relevant"},
	}
	c := NewCRAGEvaluator(fake)

	got, err := c.Evaluate(context.Background(), "q", []Candidate{makeCandidate("1", "text")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Decision != CRAGReplace {
		t.Errorf("Decision = %v, want CRAGReplace", got.Decision)
	}
}

func TestEvaluate_GatewayError_FailOpen(t *testing.T) {
	t.Parallel()

	fake := &fakeGateway{err: errors.New("boom")}
	c := NewCRAGEvaluator(fake)

	got, err := c.Evaluate(context.Background(), "q", []Candidate{makeCandidate("1", "text")})
	if err != nil {
		t.Fatalf("expected fail-open (nil error), got %v", err)
	}
	if got.Decision != CRAGUse {
		t.Errorf("Decision = %v, want CRAGUse on gateway error", got.Decision)
	}
	if got.Score != 1.0 {
		t.Errorf("Score = %v, want 1.0 on gateway error", got.Score)
	}
}
