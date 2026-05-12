package qdrant

import (
	"strings"
	"testing"
	"time"
)

// ── ExtractiveCompress ────────────────────────────────────────────────────────

func TestExtractiveCompress_ShortChunk_Passthrough(t *testing.T) {
	text := "Short text under 200 tokens."
	got := ExtractiveCompress("query", text, DefaultCompressConfig)
	if got != text {
		t.Errorf("expected passthrough for short chunk, got: %q", got)
	}
}

func TestExtractiveCompress_FewSentences_Passthrough(t *testing.T) {
	text := "First sentence. Second sentence. Third sentence."
	got := ExtractiveCompress("query", text, DefaultCompressConfig)
	if got != text {
		t.Errorf("expected passthrough for ≤3 sentences, got: %q", got)
	}
}

func TestExtractiveCompress_ReturnsSubset(t *testing.T) {
	// Build a long chunk with many sentences.
	var sb strings.Builder
	for i := 0; i < 20; i++ {
		sb.WriteString("This is a generic filler sentence with no useful information. ")
	}
	sb.WriteString("The payments-api connection pool is exhausted due to a misconfigured max-connections setting. ")
	sb.WriteString("Restart the service to apply the new pool size. ")
	sb.WriteString("Monitor with prometheus metrics after the fix. ")
	text := sb.String()

	cfg := CompressConfig{MinTokens: 10, MaxOutputTokens: 60, Context: 1}
	got := ExtractiveCompress("payments-api connection pool", text, cfg)

	if len(got) >= len(text) {
		t.Error("expected compressed output to be shorter than input")
	}
	if !strings.Contains(got, "connection pool") {
		t.Error("expected query-relevant sentence to be retained")
	}
}

func TestExtractiveCompress_DefaultConfig(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 30; i++ {
		sb.WriteString("This sentence is irrelevant to the query topic. ")
	}
	sb.WriteString("OOMKill occurs when container memory limit is exceeded. ")
	text := sb.String()

	cfg := CompressConfig{MinTokens: 10, MaxOutputTokens: 50, Context: 0}
	got := ExtractiveCompress("OOMKill memory limit", text, cfg)
	if strings.TrimSpace(got) == "" {
		t.Error("expected non-empty compressed output")
	}
}

func TestExtractiveCompress_ZeroConfig_UsesDefaults(t *testing.T) {
	text := "Short."
	got := ExtractiveCompress("query", text, CompressConfig{})
	if got != text {
		t.Errorf("zero-config should use defaults (passthrough short text), got %q", got)
	}
}

// ── FreshnessScore ────────────────────────────────────────────────────────────

func TestFreshnessScore_JustModified(t *testing.T) {
	score := FreshnessScore(DocTypeRunbook, time.Now())
	if score < 0.99 {
		t.Errorf("freshness just modified = %.4f, want ≈1.0", score)
	}
}

func TestFreshnessScore_TwoWeeksOld_Runbook(t *testing.T) {
	twoWeeksAgo := time.Now().Add(-14 * 24 * time.Hour)
	score := FreshnessScore(DocTypeRunbook, twoWeeksAgo)
	// exp(-0.05*14) ≈ 0.496
	if score < 0.40 || score > 0.60 {
		t.Errorf("runbook 14 days old freshness = %.4f, want ~0.496", score)
	}
}

func TestFreshnessScore_Postmortem_NoDecay(t *testing.T) {
	oldDate := time.Now().Add(-365 * 24 * time.Hour)
	score := FreshnessScore(DocTypePostmortem, oldDate)
	if score != 1.0 {
		t.Errorf("postmortem freshness = %.4f, want 1.0 (no decay)", score)
	}
}

func TestFreshnessScore_Architecture_SlowDecay(t *testing.T) {
	twoMonthsAgo := time.Now().Add(-60 * 24 * time.Hour)
	score := FreshnessScore(DocTypeArchitecture, twoMonthsAgo)
	// exp(-0.01*60) ≈ 0.549
	if score < 0.40 || score > 0.70 {
		t.Errorf("architecture 60 days old freshness = %.4f, want ~0.549", score)
	}
}

func TestFreshnessScore_UnknownDocType_FallsBackToRunbook(t *testing.T) {
	fixed := time.Now().Add(-14 * 24 * time.Hour)
	unknown := FreshnessScore("custom-type", fixed)
	runbook := FreshnessScore(DocTypeRunbook, fixed)
	diff := unknown - runbook
	if diff < -0.001 || diff > 0.001 {
		t.Errorf("unknown doc type freshness %.6f, runbook freshness %.6f: differ by more than 0.001", unknown, runbook)
	}
}

func TestFreshnessScore_FutureDate_ClampsToZeroDays(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	score := FreshnessScore(DocTypeRunbook, future)
	if score < 0.99 {
		t.Errorf("future modification date should clamp to 0 days, got freshness %.4f", score)
	}
}

// ── ApplyFreshness ────────────────────────────────────────────────────────────

func TestApplyFreshness_WeightsCorrectly(t *testing.T) {
	rerankerScore := float32(0.9)
	result := ApplyFreshness(rerankerScore, DocTypePostmortem, time.Now())
	// 0.85*0.9 + 0.15*1.0 = 0.765 + 0.15 = 0.915
	expected := float32(0.915)
	if result < expected-0.001 || result > expected+0.001 {
		t.Errorf("ApplyFreshness = %.4f, want ~%.4f", result, expected)
	}
}

func TestApplyFreshness_OldRunbook_LowersScore(t *testing.T) {
	rerankerScore := float32(0.9)
	oldDate := time.Now().Add(-100 * 24 * time.Hour)
	fresh := ApplyFreshness(rerankerScore, DocTypeRunbook, time.Now())
	old := ApplyFreshness(rerankerScore, DocTypeRunbook, oldDate)
	if old >= fresh {
		t.Errorf("old runbook score %.4f should be < fresh score %.4f", old, fresh)
	}
}

// ── internal helpers ──────────────────────────────────────────────────────────

func TestApproxTokenCount(t *testing.T) {
	text := "hello world foo bar baz"
	got := approxTokenCount(text)
	if got < 5 || got > 10 {
		t.Errorf("approxTokenCount(%q) = %d, want roughly 6-7", text, got)
	}
}

func TestSplitSentences_Basic(t *testing.T) {
	text := "First sentence. Second sentence! Third?"
	got := splitSentences(text)
	if len(got) != 3 {
		t.Errorf("splitSentences got %d sentences, want 3", len(got))
	}
}

func TestTokenise_LowercasesAndSplits(t *testing.T) {
	terms := tokenise("Hello, World! Foo-Bar")
	if len(terms) < 3 {
		t.Errorf("expected ≥3 tokens from 'Hello, World! Foo-Bar', got %d", len(terms))
	}
	for _, t2 := range terms {
		if strings.ToLower(t2) != t2 {
			t.Errorf("token %q is not lowercase", t2)
		}
	}
}
