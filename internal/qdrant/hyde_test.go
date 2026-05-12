package qdrant

import (
	"context"
	"errors"
	"testing"
)

func TestGenerateHypothetical_Success(t *testing.T) {
	t.Parallel()

	fake := &fakeGateway{
		resp: &CompletionResponse{Text: "hypothetical answer text"},
	}
	h := NewHyDERetriever(fake)

	got, used, err := h.GenerateHypothetical(context.Background(), "why did the api fail")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hypothetical answer text" {
		t.Errorf("hypothetical text = %q, want %q", got, "hypothetical answer text")
	}
	if !used {
		t.Errorf("hydeUsed = false, want true")
	}
}

func TestGenerateHypothetical_GatewayError(t *testing.T) {
	t.Parallel()

	fake := &fakeGateway{err: errors.New("boom")}
	h := NewHyDERetriever(fake)

	const query = "why did the api fail"
	got, used, err := h.GenerateHypothetical(context.Background(), query)
	if err != nil {
		t.Fatalf("expected fail-open (nil error), got %v", err)
	}
	if got != query {
		t.Errorf("fallback text = %q, want raw query %q", got, query)
	}
	if used {
		t.Errorf("hydeUsed = true, want false on gateway error")
	}
}

func TestConditionalHyDE_LowScore(t *testing.T) {
	t.Parallel()

	if !ConditionalHyDE("anything", 0.4) {
		t.Errorf("ConditionalHyDE(low score) = false, want true")
	}
}

func TestConditionalHyDE_HighScore_TechnicalQuery(t *testing.T) {
	t.Parallel()

	if ConditionalHyDE("payments-api error", 0.9) {
		t.Errorf("ConditionalHyDE(technical query, high score) = true, want false")
	}
}

func TestConditionalHyDE_HighScore_NaturalLanguage(t *testing.T) {
	t.Parallel()

	if !ConditionalHyDE("why is the auth service down", 0.8) {
		t.Errorf("ConditionalHyDE(natural-language question, high score) = false, want true")
	}
}
