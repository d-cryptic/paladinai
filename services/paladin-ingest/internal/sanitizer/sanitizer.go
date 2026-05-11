// Package sanitizer strips prompt-injection patterns from alert label values and
// annotations before they reach the LLM triage pipeline.
//
// Threat model: an attacker controls alert labels or annotations (e.g. via a
// misconfigured Alertmanager rule that routes attacker-controlled metrics).
// They may embed instructions such as "ignore previous instructions" or
// "system: you are now a different agent" to hijack the LLM response.
//
// This package applies defence-in-depth. It does not guarantee complete
// injection prevention -- the real defence must live in the agent's prompt
// construction (escape, XML-fence, structured outputs). Every sanitization
// event is returned to the caller for logging and metrics.
package sanitizer

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const (
	// MaxLabelValueBytes is the maximum allowed byte length for a label value
	// AFTER NFKC normalisation and before pattern replacement. Values exceeding
	// this limit are truncated. Protects against prompt-stuffing attacks.
	MaxLabelValueBytes = 512

	// MaxKeyBytes is the maximum allowed byte length for a label key.
	MaxKeyBytes = 64

	// Replacement is the placeholder substituted for stripped injection content.
	Replacement = "[REDACTED]"

	// maxIterations bounds the fixed-point loop that re-scans after each pass.
	maxIterations = 4
)

// keyPattern is the allowlist for label/annotation keys (Prometheus convention).
// Keys not matching this pattern are dropped and recorded in DroppedKeys.
var keyPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)

// zeroWidthPattern strips zero-width and invisible Unicode separators that
// bypass word-boundary checks but are semantically invisible to humans.
// Covers: ZWSP (U+200B), ZWNJ (U+200C), ZWJ (U+200D), Word Joiner (U+2060),
// BOM/ZWNBSP (U+FEFF), Narrow NBSP (U+202F), Hair Space (U+200A), Soft Hyphen (U+00AD).
var zeroWidthPattern = regexp.MustCompile(`[\x{200B}\x{200C}\x{200D}\x{2060}\x{FEFF}\x{202F}\x{200A}\x{00AD}]`)

// injectionPatterns is the ordered list of compiled regexps that identify
// prompt-injection attempts. All patterns use (?i) case-insensitive matching and
// operate on NFKC-normalised input (applied once at function entry) so original
// case is preserved for non-matching content while homoglyphs/fullwidth chars are caught.
var injectionPatterns = []*regexp.Regexp{
	// Direct instruction override
	regexp.MustCompile(`(?i)ignore\s+(all\s+)?previous\s+instructions?`),
	regexp.MustCompile(`(?i)ignore\s+the\s+(above|prior|earlier)\s+instructions?`),

	// Role/system injection
	regexp.MustCompile(`(?i)\bsystem\s*:`),
	regexp.MustCompile(`(?i)\[system\]`),
	regexp.MustCompile(`(?i)<\s*system\s*>`),

	// ChatML / Qwen / Llama control tokens (directly relevant -- we use Qwen3, DeepSeek)
	regexp.MustCompile(`(?i)<\|im_start\|>`),
	regexp.MustCompile(`(?i)<\|im_end\|>`),
	regexp.MustCompile(`(?i)<\|system\|>`),
	regexp.MustCompile(`(?i)<\|endoftext\|>`),
	regexp.MustCompile(`(?i)\[inst\]`),
	regexp.MustCompile(`(?i)\[/inst\]`),
	regexp.MustCompile(`(?i)<<sys>>`),
	regexp.MustCompile(`(?i)<</sys>>`),

	// Persona / role switch
	regexp.MustCompile(`(?i)you\s+are\s+now\s+a`),
	regexp.MustCompile(`(?i)\bjailbreak\b`),
	regexp.MustCompile(`(?i)\bpretend\s+(you\s+are|to\s+be)\b`),
	regexp.MustCompile(`(?i)\bdan\s+mode\b`),

	// Tool-call / function-call hijacking
	regexp.MustCompile(`(?i)"role"\s*:\s*"system"`),
	regexp.MustCompile(`(?i)tool_call\s*:`),

	// Markdown delimiter injection
	regexp.MustCompile(`(?i)###\s*(instruction|system|prompt)`),

	// Null bytes
	regexp.MustCompile(`\x00`),
}

// SanitizedMap wraps the sanitized output and the audit record.
type SanitizedMap struct {
	Values      map[string]string
	ChangedKeys []string // value was mutated
	DroppedKeys []string // key failed the allowlist
}

// SanitizeString normalises, strips injection patterns (to a fixed point),
// removes control characters, and truncates to MaxLabelValueBytes.
// Returns the cleaned string and true if any change was made.
// NFKC normalisation is applied once at entry so that the output is always
// in a consistent normalised form, preserving original case for clean content.
func SanitizeString(s string) (string, bool) {
	original := s

	// 1. Truncate FIRST to prevent regex scans over unbounded input.
	if len(s) > MaxLabelValueBytes {
		s = truncateToBytes(s, MaxLabelValueBytes)
	}

	// 2. Remove zero-width / invisible Unicode separators.
	s = zeroWidthPattern.ReplaceAllString(s, "")

	// 3. Remove non-printable control characters (preserve \t and \n).
	s = removeControlChars(s)

	// 4. NFKC-normalise once so fullwidth/homoglyph variants map to ASCII.
	//    This happens before pattern matching so the (?i) patterns catch them.
	s = norm.NFKC.String(s)

	// 5. Apply injection patterns in a bounded fixed-point loop to catch nested
	//    patterns that reassemble after the first pass.
	for iter := 0; iter < maxIterations; iter++ {
		next := applyPatterns(s)
		if next == s {
			break
		}
		s = next
	}

	return s, s != original
}

// SanitizeMap sanitizes both keys and values. Keys not matching the Prometheus
// naming convention are dropped and recorded in DroppedKeys.
// Values are sanitized via SanitizeString; changed keys are recorded in ChangedKeys.
func SanitizeMap(m map[string]string) SanitizedMap {
	if len(m) == 0 {
		return SanitizedMap{Values: m}
	}

	out := make(map[string]string, len(m))
	result := SanitizedMap{}
	for k, v := range m {
		// Validate key against Prometheus naming convention.
		if len(k) > MaxKeyBytes || !keyPattern.MatchString(k) {
			result.DroppedKeys = append(result.DroppedKeys, k)
			continue
		}

		cleaned, dirty := SanitizeString(v)
		out[k] = cleaned
		if dirty {
			result.ChangedKeys = append(result.ChangedKeys, k)
		}
	}
	result.Values = out
	return result
}

// applyPatterns runs all injection regexps on s (NFKC-normalised, original case).
// Each pattern uses (?i) so case variants are caught without losing original case
// for non-matching content.
func applyPatterns(s string) string {
	for _, re := range injectionPatterns {
		s = re.ReplaceAllString(s, Replacement)
	}
	return s
}

// removeControlChars strips non-printable control characters from s, preserving
// tab (\t) and newline (\n) which may appear in descriptions.
func removeControlChars(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\t' && r != '\n' {
			return -1
		}
		return r
	}, s)
}

// truncateToBytes clips s to at most maxBytes bytes without splitting UTF-8
// codepoints. Trims trailing bytes one at a time until the result is valid UTF-8.
func truncateToBytes(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	t := s[:maxBytes]
	for i := 0; i < 4 && len(t) > 0; i++ {
		if utf8.ValidString(t) {
			return t
		}
		t = t[:len(t)-1]
	}
	return t
}
