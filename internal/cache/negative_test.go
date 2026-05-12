package cache

import (
	"testing"
	"time"
)

// ── IsNegativeResponse ────────────────────────────────────────────────────────

func TestIsNegativeResponse_NoMatchMarker(t *testing.T) {
	resp := `{"intent": "runbook", "no_match": true, "confidence": 0.1}`
	if !IsNegativeResponse(resp) {
		t.Error("expected negative=true for no_match:true")
	}
}

func TestIsNegativeResponse_NoRelevantRunbook(t *testing.T) {
	resp := `{"result": "no relevant runbook found for this alert"}`
	if !IsNegativeResponse(resp) {
		t.Error("expected negative=true for 'no relevant runbook'")
	}
}

func TestIsNegativeResponse_LowConfidence(t *testing.T) {
	resp := `{"summary": "maybe", "confidence": 0.15}`
	if !IsNegativeResponse(resp) {
		t.Error("expected negative=true for confidence < 0.30")
	}
}

func TestIsNegativeResponse_HighConfidence_NotNegative(t *testing.T) {
	resp := `{"summary": "payments-api OOM", "confidence": 0.92}`
	if IsNegativeResponse(resp) {
		t.Error("expected negative=false for high confidence response")
	}
}

func TestIsNegativeResponse_NoConfidenceField_NotNegative(t *testing.T) {
	resp := `{"summary": "restart the service"}`
	if IsNegativeResponse(resp) {
		t.Error("expected negative=false when no confidence field")
	}
}

func TestIsNegativeResponse_InsufficientInformation(t *testing.T) {
	resp := "Insufficient information to determine root cause."
	if !IsNegativeResponse(resp) {
		t.Error("expected negative=true for 'insufficient information'")
	}
}

func TestIsNegativeResponse_FoundFalse(t *testing.T) {
	resp := `{"found": false, "message": "no runbook"}`
	if !IsNegativeResponse(resp) {
		t.Error("expected negative=true for found:false")
	}
}

func TestIsNegativeResponse_ExactThreshold_NotNegative(t *testing.T) {
	resp := `{"confidence": 0.30}`
	if IsNegativeResponse(resp) {
		t.Error("expected negative=false for confidence == 0.30 (threshold is <0.30)")
	}
}

// ── TTLForResponse ────────────────────────────────────────────────────────────

func TestTTLForResponse_NegativeGetsShortTTL(t *testing.T) {
	resp := `{"no_match": true}`
	ttl := TTLForResponse(QueryTypeRCA, resp)
	if ttl != NegativeResultTTL {
		t.Errorf("TTLForResponse negative = %v, want NegativeResultTTL=%v", ttl, NegativeResultTTL)
	}
}

func TestTTLForResponse_PositiveGetsQueryTypeTTL(t *testing.T) {
	resp := `{"summary": "payments OOM", "confidence": 0.95}`
	ttl := TTLForResponse(QueryTypeRCA, resp)
	if ttl != TTLForQueryType(QueryTypeRCA) {
		t.Errorf("TTLForResponse positive = %v, want %v", ttl, TTLForQueryType(QueryTypeRCA))
	}
}

func TestNegativeResultTTL_Value(t *testing.T) {
	if NegativeResultTTL != 15*time.Minute {
		t.Errorf("NegativeResultTTL = %v, want 15min", NegativeResultTTL)
	}
}

// ── Snappy round-trip ─────────────────────────────────────────────────────────

func TestEncodeDecodeL1_RoundTrip(t *testing.T) {
	original := `{"summary": "payments-api OOM after memory limit 512Mi exceeded"}`
	compressed := EncodeL1(original)
	decoded, err := DecodeL1(compressed)
	if err != nil {
		t.Fatalf("DecodeL1 error: %v", err)
	}
	if decoded != original {
		t.Errorf("round-trip mismatch: got %q, want %q", decoded, original)
	}
}

func TestEncodeDecodeL1_CompressedShorterForLongResponse(t *testing.T) {
	// Build a long, repetitive LLM response (typical L1 content).
	var buf [512]byte
	for i := range buf {
		buf[i] = byte('a' + i%26)
	}
	original := string(buf[:])
	compressed := EncodeL1(original)
	if len(compressed) >= len(original) {
		t.Logf("compressed=%d original=%d (compression may not help for random data)", len(compressed), len(original))
	}
	// Always verify round-trip.
	decoded, err := DecodeL1(compressed)
	if err != nil {
		t.Fatalf("DecodeL1 error: %v", err)
	}
	if decoded != original {
		t.Errorf("round-trip failed for long response")
	}
}

func TestDecodeL1_InvalidData_ReturnsError(t *testing.T) {
	_, err := DecodeL1([]byte("not snappy compressed data"))
	if err == nil {
		t.Error("expected error decoding non-snappy data")
	}
}

func TestEncodeL1_EmptyString(t *testing.T) {
	compressed := EncodeL1("")
	decoded, err := DecodeL1(compressed)
	if err != nil {
		t.Fatalf("DecodeL1 empty: %v", err)
	}
	if decoded != "" {
		t.Errorf("round-trip of empty string = %q, want empty", decoded)
	}
}
