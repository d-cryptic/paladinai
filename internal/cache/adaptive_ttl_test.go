package cache

import (
	"testing"
	"time"
)

func TestRecordHit_IncrementsCounter(t *testing.T) {
	h := NewHitCounter()
	h.RecordHit("key-a")
	h.RecordHit("key-a")
	if got := h.Hits("key-a"); got != 2 {
		t.Errorf("Hits = %d, want 2", got)
	}
}

func TestAdjustTTL_UnusedResponse_HalvesTTL(t *testing.T) {
	h := NewHitCounter()
	base := 2 * time.Hour
	got := h.AdjustTTL("fresh-key", base) // hits == 0
	want := 1 * time.Hour                 // 2h × 0.5
	if got != want {
		t.Errorf("AdjustTTL unused = %v, want %v", got, want)
	}
}

func TestAdjustTTL_StableResponse_ExtendsTTL(t *testing.T) {
	h := NewHitCounter()
	key := "hot-key"
	for i := 0; i < StableThreshold+1; i++ {
		h.RecordHit(key)
	}
	base := 2 * time.Hour
	got := h.AdjustTTL(key, base) // hits > StableThreshold → ×1.5
	want := 3 * time.Hour
	if got != want {
		t.Errorf("AdjustTTL stable = %v, want %v", got, want)
	}
}

func TestAdjustTTL_ModerateHits_NoChange(t *testing.T) {
	h := NewHitCounter()
	key := "warm-key"
	h.RecordHit(key) // 1 hit → between 0 and StableThreshold
	h.RecordHit(key) // 2 hits
	base := 1 * time.Hour
	got := h.AdjustTTL(key, base)
	if got != base {
		t.Errorf("AdjustTTL moderate = %v, want %v", got, base)
	}
}

func TestAdjustTTL_ResetsCounterAfterCall(t *testing.T) {
	h := NewHitCounter()
	key := "reset-key"
	for i := 0; i < 10; i++ {
		h.RecordHit(key)
	}
	_ = h.AdjustTTL(key, time.Hour)
	if got := h.Hits(key); got != 0 {
		t.Errorf("counter not reset after AdjustTTL, got %d", got)
	}
}

func TestAdjustTTL_ClampsToFloor(t *testing.T) {
	h := NewHitCounter()
	tiny := 5 * time.Minute // 5min × 0.5 = 2.5min < 10min floor
	got := h.AdjustTTL("floor-key", tiny)
	if got != AdaptiveTTLFloor {
		t.Errorf("AdjustTTL floor = %v, want %v", got, AdaptiveTTLFloor)
	}
}

func TestAdjustTTL_ClampsToCeiling(t *testing.T) {
	h := NewHitCounter()
	key := "ceiling-key"
	for i := 0; i < StableThreshold+1; i++ {
		h.RecordHit(key)
	}
	big := 6 * time.Hour // 6h × 1.5 = 9h > 8h ceiling
	got := h.AdjustTTL(key, big)
	if got != AdaptiveTTLCeiling {
		t.Errorf("AdjustTTL ceiling = %v, want %v", got, AdaptiveTTLCeiling)
	}
}

func TestHitCounter_IndependentKeys(t *testing.T) {
	h := NewHitCounter()
	h.RecordHit("a")
	h.RecordHit("a")
	h.RecordHit("b")

	if h.Hits("a") != 2 {
		t.Error("expected a=2")
	}
	if h.Hits("b") != 1 {
		t.Error("expected b=1")
	}
}

func TestHitCounter_ZeroForUnknownKey(t *testing.T) {
	h := NewHitCounter()
	if h.Hits("nonexistent") != 0 {
		t.Error("expected 0 for unknown key")
	}
}

func TestAdjustTTL_ExactlyAtThreshold_NoExtension(t *testing.T) {
	h := NewHitCounter()
	key := "at-threshold"
	for i := 0; i < StableThreshold; i++ { // exactly StableThreshold, not >
		h.RecordHit(key)
	}
	base := time.Hour
	got := h.AdjustTTL(key, base)
	if got != base {
		t.Errorf("AdjustTTL at threshold = %v, want %v (no extension)", got, base)
	}
}
