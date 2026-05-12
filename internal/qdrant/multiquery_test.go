package qdrant

import (
	"context"
	"errors"
	"testing"
)

func TestExpand_Success(t *testing.T) {
	gw := &fakeGateway{
		resp: &CompletionResponse{
			Text: "VARIANT_1: foo\nVARIANT_2: bar\nVARIANT_3: baz",
		},
	}
	m := NewMultiQueryExpander(gw)
	out, err := m.Expand(context.Background(), "original")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 4 {
		t.Fatalf("expected 4 queries, got %d: %v", len(out), out)
	}
	if out[0] != "original" {
		t.Errorf("expected first query to be original, got %q", out[0])
	}
	if out[1] != "foo" || out[2] != "bar" || out[3] != "baz" {
		t.Errorf("unexpected variants: %v", out[1:])
	}
}

func TestExpand_GatewayError(t *testing.T) {
	gw := &fakeGateway{err: errors.New("boom")}
	m := NewMultiQueryExpander(gw)
	out, err := m.Expand(context.Background(), "original")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out[0] != "original" {
		t.Fatalf("expected [original], got %v", out)
	}
}

func TestExpand_PartialParse(t *testing.T) {
	gw := &fakeGateway{
		resp: &CompletionResponse{
			Text: "VARIANT_1: only one\nVARIANT_2: and two\nsome other line",
		},
	}
	m := NewMultiQueryExpander(gw)
	out, err := m.Expand(context.Background(), "q")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 queries, got %d: %v", len(out), out)
	}
}

func TestShouldExpandQuery_LongNatural(t *testing.T) {
	if !ShouldExpandQuery("why is the payments service timing out after deploy") {
		t.Errorf("expected long natural-language query to be expandable")
	}
}

func TestShouldExpandQuery_ShortExact(t *testing.T) {
	if ShouldExpandQuery("payments-api") {
		t.Errorf("expected short exact identifier not to be expandable")
	}
}
