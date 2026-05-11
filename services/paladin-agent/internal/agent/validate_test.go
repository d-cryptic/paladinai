// White-box tests: package agent gives access to unexported validate() and truncate().
package agent

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── truncate ──────────────────────────────────────────────────────────────────

func TestTruncate_ShortStringUnchanged(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "hello", truncate("hello", 10))
}

func TestTruncate_ExactlyMaxUnchanged(t *testing.T) {
	t.Parallel()
	s := strings.Repeat("a", 32)
	assert.Equal(t, s, truncate(s, 32))
}

func TestTruncate_OverMaxClipped(t *testing.T) {
	t.Parallel()
	s := strings.Repeat("b", 50)
	got := truncate(s, 32)
	assert.Equal(t, 32, len(got))
	assert.Equal(t, strings.Repeat("b", 32), got)
}

func TestTruncate_EmptyStringUnchanged(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", truncate("", 10))
}

func TestTruncate_MultibyteSafeAtRune(t *testing.T) {
	t.Parallel()
	// "日本語" = 3 runes, each is 3 UTF-8 bytes (9 bytes total).
	// Truncating at 2 runes should return "日本" (6 bytes), not a broken byte slice.
	s := "日本語"
	got := truncate(s, 2)
	assert.Equal(t, "日本", got)
	// Verify the result is valid UTF-8 (no partial codepoints).
	assert.True(t, len(got) == 6, "should be 6 bytes for 2 Japanese runes")
}

func TestTruncate_MultibyteSafeAtBoundary(t *testing.T) {
	t.Parallel()
	// Mix of ASCII and multibyte: "ab日本語" — 5 runes.
	// Clipping at 4 should return "ab日本" (2 bytes ASCII + 6 bytes CJK = 8 bytes total).
	s := "ab日本語"
	got := truncate(s, 4)
	assert.Equal(t, "ab日本", got)
}

func TestTruncate_MaxZeroReturnsEmpty(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", truncate("hello", 0))
}

// ── TriageResult.validate ─────────────────────────────────────────────────────

func TestValidate_ValidP1(t *testing.T) {
	t.Parallel()
	r := &TriageResult{ConfirmedSeverity: "P1", Summary: "outage", LikelyCause: "disk full"}
	require.NoError(t, r.validate())
	assert.Equal(t, "P1", r.ConfirmedSeverity)
}

func TestValidate_LowercaseSeverityNormalised(t *testing.T) {
	t.Parallel()
	for _, sev := range []string{"p1", "p2", "p3", "p4"} {
		t.Run(sev, func(t *testing.T) {
			t.Parallel()
			r := &TriageResult{ConfirmedSeverity: sev}
			require.NoError(t, r.validate())
			assert.Equal(t, strings.ToUpper(sev), r.ConfirmedSeverity)
		})
	}
}

func TestValidate_UnknownSeverityReturnsError(t *testing.T) {
	t.Parallel()
	for _, sev := range []string{"P0", "P5", "critical", "high", "", "UNKNOWN"} {
		t.Run(sev, func(t *testing.T) {
			t.Parallel()
			r := &TriageResult{ConfirmedSeverity: sev}
			err := r.validate()
			require.Error(t, err, "severity %q should be rejected", sev)
			assert.Contains(t, err.Error(), "unknown severity")
		})
	}
}

func TestValidate_SeverityWithLeadingTrailingSpaceAccepted(t *testing.T) {
	t.Parallel()
	r := &TriageResult{ConfirmedSeverity: "  P2  "}
	require.NoError(t, r.validate())
	assert.Equal(t, "P2", r.ConfirmedSeverity)
}

func TestValidate_SummaryTruncatedAt200(t *testing.T) {
	t.Parallel()
	r := &TriageResult{
		ConfirmedSeverity: "P3",
		Summary:           strings.Repeat("x", 300),
	}
	require.NoError(t, r.validate())
	assert.Equal(t, maxSummaryLen, len([]rune(r.Summary)))
}

func TestValidate_LikelyCauseTruncatedAt400(t *testing.T) {
	t.Parallel()
	r := &TriageResult{
		ConfirmedSeverity: "P3",
		LikelyCause:       strings.Repeat("y", 500),
	}
	require.NoError(t, r.validate())
	assert.Equal(t, maxCauseLen, len([]rune(r.LikelyCause)))
}

func TestValidate_AffectedServicesCappedAt20(t *testing.T) {
	t.Parallel()
	services := make([]string, 25)
	for i := range services {
		services[i] = "svc"
	}
	r := &TriageResult{ConfirmedSeverity: "P2", AffectedServices: services}
	require.NoError(t, r.validate())
	assert.Len(t, r.AffectedServices, maxServiceCount)
}

func TestValidate_ServiceNameTruncatedAt64(t *testing.T) {
	t.Parallel()
	r := &TriageResult{
		ConfirmedSeverity: "P2",
		AffectedServices:  []string{strings.Repeat("s", 100)},
	}
	require.NoError(t, r.validate())
	assert.Equal(t, maxServiceLen, len([]rune(r.AffectedServices[0])))
}

func TestValidate_EmptyAffectedServicesOK(t *testing.T) {
	t.Parallel()
	r := &TriageResult{ConfirmedSeverity: "P4", AffectedServices: nil}
	require.NoError(t, r.validate())
	assert.Nil(t, r.AffectedServices)
}
