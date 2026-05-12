// negative.go implements negative caching for LLM responses (Stage 9 §17)
// and Snappy compression helpers for L1 entries (Stage 9 §15).
//
// Negative caching: LLM responses with low confidence or explicit "no_match"
// flags are still cached — but with a short TTL (15min) so they expire before
// meaningful runbook updates are ready. This avoids paying the LLM call cost
// for repeated un-answerable queries.
//
// Snappy compression: L1 entries are compressed before storage (saves ~50%
// Valkey memory on 1-5KB LLM responses) and decompressed on retrieval.
package cache

import (
	"strings"
	"time"

	"github.com/klauspost/compress/snappy"
)

// NegativeResultTTL is the short TTL applied to low-confidence LLM responses.
const NegativeResultTTL = 15 * time.Minute

// NegativeThreshold is the confidence level below which a response is treated
// as a negative result and cached with NegativeResultTTL.
const NegativeThreshold = 0.30

// negativeMarkers are substrings that explicitly indicate a negative result.
// These are matched case-insensitively in structured LLM response text.
var negativeMarkers = []string{
	`"no_match"`,
	`"no_match": true`,
	`"found": false`,
	`"found":false`,
	"no relevant runbook",
	"insufficient information",
	"unable to determine",
}

// IsNegativeResponse reports whether an LLM response text is a negative result
// that should be cached with NegativeResultTTL. Returns true when:
//   - The response text contains a known negative marker string, OR
//   - The extracted confidence value is below NegativeThreshold.
func IsNegativeResponse(resp string) bool {
	lower := strings.ToLower(resp)
	for _, marker := range negativeMarkers {
		if strings.Contains(lower, strings.ToLower(marker)) {
			return true
		}
	}
	return extractConfidence(resp) < NegativeThreshold
}

// extractConfidence parses the "confidence" field from a JSON-like response.
// Returns 1.0 (optimistic default) when no confidence field is found.
func extractConfidence(resp string) float64 {
	// Quick scan: look for "confidence": N.NN pattern.
	lower := strings.ToLower(resp)
	idx := strings.Index(lower, `"confidence"`)
	if idx == -1 {
		return 1.0 // no confidence field → not a negative result
	}
	// Scan forward past the colon to find the numeric value.
	colon := strings.Index(resp[idx:], ":")
	if colon == -1 {
		return 1.0
	}
	start := idx + colon + 1
	// Skip whitespace.
	for start < len(resp) && (resp[start] == ' ' || resp[start] == '\t') {
		start++
	}
	// Read digits and decimal point.
	end := start
	for end < len(resp) && (resp[end] >= '0' && resp[end] <= '9' || resp[end] == '.') {
		end++
	}
	if start >= end {
		return 1.0
	}
	var val float64
	for _, ch := range resp[start:end] {
		if ch == '.' {
			continue
		}
		val = val*10 + float64(ch-'0')
	}
	// Count decimal places.
	if dotPos := strings.Index(resp[start:end], "."); dotPos != -1 {
		decimals := end - (start + dotPos + 1)
		div := 1.0
		for range make([]struct{}, decimals) {
			div *= 10
		}
		val /= div
	}
	return val
}

// TTLForResponse returns the appropriate cache TTL for an LLM response.
// Negative responses use NegativeResultTTL; others use the query-type TTL.
func TTLForResponse(qt QueryType, resp string) time.Duration {
	if IsNegativeResponse(resp) {
		return NegativeResultTTL
	}
	return TTLForQueryType(qt)
}

// ─── Snappy L1 compression ─────────────────────────────────────────────────────

// EncodeL1 Snappy-compresses an LLM response for storage in Valkey L1.
// The compressed bytes are safe to store as Redis byte values.
func EncodeL1(resp string) []byte {
	return snappy.Encode(nil, []byte(resp))
}

// DecodeL1 decompresses a Snappy-compressed L1 value back to a string.
// Returns the original compressed bytes as a string on decode error (defensive).
func DecodeL1(data []byte) (string, error) {
	decoded, err := snappy.Decode(nil, data)
	if err != nil {
		return string(data), err
	}
	return string(decoded), nil
}
