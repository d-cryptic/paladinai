package cache

import (
	"testing"
	"time"
)

func TestEstimateSaving_OpusRCA(t *testing.T) {
	saving := EstimateSaving(ProfileOpusRCA)
	// (8000/1e6)*15 + (600/1e6)*75 = 0.12 + 0.045 = 0.165
	if saving < 0.16 || saving > 0.17 {
		t.Errorf("EstimateSaving(Opus) = %.6f, want ~0.165", saving)
	}
}

func TestEstimateSaving_QwenClassify(t *testing.T) {
	saving := EstimateSaving(ProfileQwenClassify)
	// (1000/1e6)*0.06 + (50/1e6)*0.06 = 0.00006 + 0.000003 = 0.000063
	if saving <= 0 {
		t.Errorf("EstimateSaving(Qwen) = %.8f, want >0", saving)
	}
}

func TestCostAttributor_RecordAndTotalSaved(t *testing.T) {
	ca := NewCostAttributor()
	ca.Record(CacheHitEvent{
		TenantID:     "tenant-a",
		TierHit:      "l1",
		ModelProfile: ProfileSonnetTriage,
		HitAt:        time.Now(),
	})
	ca.Record(CacheHitEvent{
		TenantID:     "tenant-a",
		TierHit:      "l2",
		CostSavedUSD: 0.050,
		HitAt:        time.Now(),
	})

	total := ca.TotalSaved("tenant-a")
	if total <= 0.050 {
		t.Errorf("TotalSaved = %.6f, want > 0.050", total)
	}
}

func TestCostAttributor_HitCount(t *testing.T) {
	ca := NewCostAttributor()
	for i := 0; i < 5; i++ {
		ca.Record(CacheHitEvent{TenantID: "t1", ModelProfile: ProfileQwenClassify, HitAt: time.Now()})
	}
	if ca.HitCount("t1") != 5 {
		t.Errorf("HitCount = %d, want 5", ca.HitCount("t1"))
	}
}

func TestCostAttributor_IndependentTenants(t *testing.T) {
	ca := NewCostAttributor()
	ca.Record(CacheHitEvent{TenantID: "a", CostSavedUSD: 0.01, HitAt: time.Now()})
	ca.Record(CacheHitEvent{TenantID: "b", CostSavedUSD: 0.02, HitAt: time.Now()})

	if ca.TotalSaved("a") != 0.01 {
		t.Errorf("tenant a total = %.4f, want 0.01", ca.TotalSaved("a"))
	}
	if ca.TotalSaved("b") != 0.02 {
		t.Errorf("tenant b total = %.4f, want 0.02", ca.TotalSaved("b"))
	}
}

func TestCostAttributor_Flush_ResetsCounters(t *testing.T) {
	ca := NewCostAttributor()
	ca.Record(CacheHitEvent{TenantID: "t1", CostSavedUSD: 1.0, HitAt: time.Now()})

	snapshot := ca.Flush()
	if snapshot["t1"] != 1.0 {
		t.Errorf("flush snapshot = %.2f, want 1.0", snapshot["t1"])
	}
	if ca.TotalSaved("t1") != 0.0 {
		t.Errorf("total after flush = %.2f, want 0.0", ca.TotalSaved("t1"))
	}
	if ca.HitCount("t1") != 0 {
		t.Errorf("hit count after flush = %d, want 0", ca.HitCount("t1"))
	}
}

func TestCostAttributor_ZeroForUnknownTenant(t *testing.T) {
	ca := NewCostAttributor()
	if ca.TotalSaved("nobody") != 0 {
		t.Error("expected 0 for unknown tenant")
	}
	if ca.HitCount("nobody") != 0 {
		t.Error("expected 0 hit count for unknown tenant")
	}
}

func TestCostAttributor_ExplicitCostSavedUSD_OverridesProfile(t *testing.T) {
	ca := NewCostAttributor()
	ca.Record(CacheHitEvent{
		TenantID:     "t2",
		CostSavedUSD: 9.99,
		ModelProfile: ProfileOpusRCA, // should be ignored when CostSavedUSD is set
		HitAt:        time.Now(),
	})
	if ca.TotalSaved("t2") != 9.99 {
		t.Errorf("explicit cost override = %.2f, want 9.99", ca.TotalSaved("t2"))
	}
}
