// Package sanitizer strips prompt-injection patterns from alert label values and
// annotations before they reach the LLM triage pipeline.
//
// Threat model: an attacker controls alert labels or annotations (e.g. via a
// misconfigured Alertmanager rule that routes attacker-controlled metrics).
// They may embed instructions such as "ignore previous instructions" or
// "system: you are now a different agent" to hijack the LLM response.
//
// This package applies defence-in-depth: it does not guarantee perfect
// injection prevention (that requires prompt construction discipline in the
// agent), but it removes the most common patterns before data enters the
// system and logs every sanitization event so operators can investigate.
package sanitizer

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// MaxLabelValueBytes is the maximum allowed byte length for a label value.
	// Values exceeding this limit are truncated. Protects against prompt
	// stuffing via extremely long values.
	MaxLabelValueBytes = 512

	// replacement is the placeholder substituted for stripped injection content.
	replacement = "[REDACTED]"
)

// injectionPatterns is the ordered list of compiled regexps that identify
// prompt-injection attempts. Patterns are case-insensitive and designed to
// match common jailbreak variants while preserving legitimate alert metadata.
var injectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ignore\s+(all\s+)?previous\s+instructions?`),
	regexp.MustCompile(`(?i)ignore\s+the\s+(above|prior|earlier)\s+instructions?`),
	regexp.MustCompile(`(?i)\bsystem\s*:`),
	regexp.MustCompile(`(?i)\[system\]`),
	regexp.MustCompile(`(?i)<\s*system\s*>`),
	regexp.MustCompile(`(?i)you\s+are\s+now\s+a`),
	regexp.MustCompile(`(?i)act\s+as\s+(a|an|the)\s+`),
	regexp.MustCompile(`(?i)\bjailbreak\b`),
	regexp.MustCompile(`(?i)\bpretend\s+(you\s+are|to\s+be)\b`),
	regexp.MustCompile(`(?i)\bDAN\b`),            // "Do Anything Now" jailbreak variant
	regexp.MustCompile(`\x00`),                   // null bytes
	regexp.MustCompile(`\r\n|\n\n\n`),            // multi-line prompt delimiters
	regexp.MustCompile(`(?i)###\s*(instruction|system|prompt)`), // markdown delimiter injection
}

// SanitizeString replaces known injection patterns with [REDACTED] and
// truncates the result to MaxLabelValueBytes. It also strips non-printable
// control characters (except tab and newline).
// Returns the cleaned string and whether any sanitization was applied.
func SanitizeString(s string) (string, bool) {
	original := s

	// Remove non-printable control characters (except \t and \n).
	s = removeControlChars(s)

	// Apply injection pattern replacements.
	for _, re := range injectionPatterns {
		s = re.ReplaceAllString(s, replacement)
	}

	// Truncate to byte limit.
	if len(s) > MaxLabelValueBytes {
		s = truncateToBytes(s, MaxLabelValueBytes)
	}

	return s, s != original
}

// SanitizeMap returns a new map with every value sanitized via SanitizeString.
// Keys are never modified. Returns the sanitized map and the set of keys whose
// values were changed (nil if no changes occurred).
func SanitizeMap(m map[string]string) (map[string]string, []string) {
	if len(m) == 0 {
		return m, nil
	}

	out := make(map[string]string, len(m))
	var changed []string
	for k, v := range m {
		cleaned, dirty := SanitizeString(v)
		out[k] = cleaned
		if dirty {
			changed = append(changed, k)
		}
	}
	return out, changed
}

// removeControlChars strips non-printable control characters from s, preserving
// tab (\t) and newline (\n) which may appear legitimately in descriptions.
func removeControlChars(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\t' && r != '\n' {
			return -1 // drop the rune
		}
		return r
	}, s)
}

// truncateToBytes clips s to at most maxBytes bytes without splitting UTF-8
// codepoints. It trims at most 3 trailing bytes — the maximum number of
// continuation bytes in a 4-byte UTF-8 sequence.
func truncateToBytes(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	t := s[:maxBytes]
	// Walk back at most 3 bytes to find a valid UTF-8 boundary.
	for i := 0; i < 4 && len(t) > 0; i++ {
		if utf8.ValidString(t) {
			return t
		}
		t = t[:len(t)-1]
	}
	return t
}
