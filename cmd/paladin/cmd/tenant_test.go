package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSlugRE covers the slug regex used by tenant create, migrate, and delete.
// Regex: ^[a-z0-9][a-z0-9-]{0,62}[a-z0-9]$
// Rules: 2-64 chars, lowercase alphanumeric + hyphens, no leading/trailing hyphen.
func TestSlugRE(t *testing.T) {
	valid := []string{
		"ab",                                // minimum 2 chars
		"a1",                                // alphanumeric 2-char
		"acme-corp",                         // typical slug
		"tenant-123",                        // digits in middle
		"a-b-c-d",                           // multiple hyphens
		"my-org-prod",                       // multi-segment
		"x" + strings.Repeat("a", 62) + "x", // 64 chars (max)
		"a--b",                              // consecutive hyphens are allowed by current regex
	}
	invalid := []string{
		"a",                     // too short (1 char)
		"",                      // empty
		"-abc",                  // leading hyphen
		"abc-",                  // trailing hyphen
		"ABC",                   // uppercase
		"Acme-Corp",             // mixed case
		"acme_corp",             // underscore not allowed
		"acme corp",             // space not allowed
		strings.Repeat("a", 65), // 65 chars (max+1)
		"café",                  // non-ASCII
		"ab\nc",                 // newline injection
	}

	for _, s := range valid {
		assert.True(t, slugRE.MatchString(s), "expected valid slug: %q", s)
	}
	for _, s := range invalid {
		assert.False(t, slugRE.MatchString(s), "expected invalid slug: %q", s)
	}
}

func TestConfirmTenantDelete_CIModeRequiresYes(t *testing.T) {
	cmd := tenantDeleteTestCmd(t, false, true)

	err := confirmTenantDelete(cmd, "acme-corp")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--yes")
	assert.Contains(t, err.Error(), "CI mode")
}

func TestConfirmTenantDelete_YesSkipsPromptInCIMode(t *testing.T) {
	cmd := tenantDeleteTestCmd(t, true, true)

	require.NoError(t, confirmTenantDelete(cmd, "acme-corp"))
}

func tenantDeleteTestCmd(t *testing.T, yes, ci bool) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "delete"}
	cmd.Flags().Bool("yes", yes, "")
	cmd.Flags().Bool("ci", ci, "")
	return cmd
}
