package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
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

func TestWriteTenantCreateResult_CIModeJSON(t *testing.T) {
	cmd := tenantDeleteTestCmd(t, false, true)

	out := captureTenantStdout(t, func() {
		require.NoError(t, writeTenantCreateResult(cmd, []byte(`{"id":"tenant-1","slug":"acme-corp"}`)))
	})

	var result tenantActionResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "create", result.Action)
	assert.Equal(t, "tenant-1", result.TenantID)
	assert.Equal(t, "acme-corp", result.Slug)
	assert.JSONEq(t, `{"id":"tenant-1","slug":"acme-corp"}`, string(result.Response))
	assert.NotContains(t, out, "Tenant created")
}

func TestWriteTenantStateActionResult_CIModeJSON(t *testing.T) {
	cmd := tenantDeleteTestCmd(t, false, true)

	out := captureTenantStdout(t, func() {
		require.NoError(t, writeTenantStateActionResult(cmd, "tenant-1", "suspend", []byte(`{"state":"suspended"}`)))
	})

	var result tenantActionResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "suspend", result.Action)
	assert.Equal(t, "tenant-1", result.TenantID)
	assert.JSONEq(t, `{"state":"suspended"}`, string(result.Response))
	assert.NotContains(t, out, "Tenant")
}

func TestWriteTenantMigrateResult_CIModeJSON(t *testing.T) {
	cmd := tenantDeleteTestCmd(t, false, true)

	out := captureTenantStdout(t, func() {
		require.NoError(t, writeTenantMigrateResult(cmd, "acme-corp", "silo", []byte(`{"job_id":"job-1"}`)))
	})

	var result tenantActionResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "migrate", result.Action)
	assert.Equal(t, "acme-corp", result.Slug)
	assert.Equal(t, "silo", result.Tier)
	assert.JSONEq(t, `{"job_id":"job-1"}`, string(result.Response))
	assert.NotContains(t, out, "migration")
}

func TestWriteTenantDeleteResult_CIModeJSON(t *testing.T) {
	cmd := tenantDeleteTestCmd(t, false, true)

	out := captureTenantStdout(t, func() {
		require.NoError(t, writeTenantDeleteResult(cmd, "acme-corp", nil))
	})

	var result tenantActionResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "delete", result.Action)
	assert.Equal(t, "acme-corp", result.Slug)
	assert.Empty(t, result.Response)
	assert.NotContains(t, out, "Tenant")
}

func TestWriteTenantCreateResult_HumanOutput(t *testing.T) {
	cmd := tenantDeleteTestCmd(t, false, false)

	out := captureTenantStdout(t, func() {
		require.NoError(t, writeTenantCreateResult(cmd, []byte(`{"id":"tenant-1","slug":"acme-corp"}`)))
	})

	assert.Contains(t, out, "Tenant created: tenant-1 (slug: acme-corp)")
}

func tenantDeleteTestCmd(t *testing.T, yes, ci bool) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "delete"}
	cmd.Flags().Bool("yes", yes, "")
	cmd.Flags().Bool("ci", ci, "")
	return cmd
}

func captureTenantStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)

	origOut := os.Stdout
	os.Stdout = w

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.Copy(&buf, r)
	}()

	fn()

	os.Stdout = origOut
	require.NoError(t, w.Close())
	<-done
	return buf.String()
}
