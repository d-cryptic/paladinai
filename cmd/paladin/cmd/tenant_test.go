package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSlugRE covers the slug regex used by tenant create, migrate, and delete.
// Regex: ^[a-z0-9][a-z0-9-]{0,62}[a-z0-9]$
// Rules: 2-64 chars, lowercase alphanumeric + hyphens, no leading/trailing hyphen.
func TestSlugRE(t *testing.T) {
	valid := []string{
		"ab",                                          // minimum 2 chars
		"a1",                                          // alphanumeric 2-char
		"acme-corp",                                   // typical slug
		"tenant-123",                                  // digits in middle
		"a-b-c-d",                                     // multiple hyphens
		"my-org-prod",                                 // multi-segment
		"x" + strings.Repeat("a", 62) + "x",          // 64 chars (max)
		"a--b",                                        // consecutive hyphens are allowed by current regex
	}
	invalid := []string{
		"a",                       // too short (1 char)
		"",                        // empty
		"-abc",                    // leading hyphen
		"abc-",                    // trailing hyphen
		"ABC",                     // uppercase
		"Acme-Corp",               // mixed case
		"acme_corp",               // underscore not allowed
		"acme corp",               // space not allowed
		strings.Repeat("a", 65),   // 65 chars (max+1)
		"café",                    // non-ASCII
		"ab\nc",                   // newline injection
	}

	for _, s := range valid {
		assert.True(t, slugRE.MatchString(s), "expected valid slug: %q", s)
	}
	for _, s := range invalid {
		assert.False(t, slugRE.MatchString(s), "expected invalid slug: %q", s)
	}
}
