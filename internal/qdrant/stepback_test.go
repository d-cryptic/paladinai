package qdrant

import (
	"context"
	"errors"
	"testing"
)

func TestGenerateStepBack_Success(t *testing.T) {
	gw := &fakeGateway{
		resp: &CompletionResponse{Text: "common causes of memory exhaustion"},
	}
	s := NewStepBackPrompter(gw)
	out, used, err := s.GenerateStepBack(context.Background(), "why is my pod OOMing?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !used {
		t.Errorf("expected used=true")
	}
	if out != "common causes of memory exhaustion" {
		t.Errorf("unexpected stepback: %q", out)
	}
}

func TestGenerateStepBack_GatewayError(t *testing.T) {
	gw := &fakeGateway{err: errors.New("boom")}
	s := NewStepBackPrompter(gw)
	original := "why is my pod OOMing?"
	out, used, err := s.GenerateStepBack(context.Background(), original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if used {
		t.Errorf("expected used=false on gateway error")
	}
	if out != original {
		t.Errorf("expected original query, got %q", out)
	}
}

func TestIsSymptomQuery_Why(t *testing.T) {
	if !IsSymptomQuery("why is the pod crashing") {
		t.Errorf("expected 'why is' query to be a symptom query")
	}
}

func TestIsSymptomQuery_Lookup(t *testing.T) {
	if IsSymptomQuery("oomkill threshold config") {
		t.Errorf("expected lookup query not to be a symptom query")
	}
}

func TestIsSymptomQuery_HowToFix(t *testing.T) {
	if !IsSymptomQuery("how do I fix connection pool exhaustion") {
		t.Errorf("expected 'how do I fix' query to be a symptom query")
	}
}
