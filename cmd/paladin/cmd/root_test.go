package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestCmd creates a minimal cobra.Command with the persistent flags that
// requireTenant / apiURL / optToken expect. This avoids depending on the global
// rootCmd state.
func newTestCmd(tenantVal, tokenVal, apiURLVal string) *cobra.Command {
	c := &cobra.Command{Use: "test"}
	c.PersistentFlags().String("tenant", tenantVal, "")
	c.PersistentFlags().String("token", tokenVal, "")
	c.PersistentFlags().String("api-url", apiURLVal, "")
	return c
}

func TestRequireTenant_ReturnsValueWhenSet(t *testing.T) {
	cmd := newTestCmd("acme-corp", "", "http://api")
	got, err := requireTenant(cmd)
	require.NoError(t, err)
	assert.Equal(t, "acme-corp", got)
}

func TestRequireTenant_ErrorWhenEmpty(t *testing.T) {
	cmd := newTestCmd("", "", "http://api")
	_, err := requireTenant(cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant")
}

func TestRequireTenant_ErrorWhenFlagAbsent(t *testing.T) {
	// Command with no --tenant flag at all.
	cmd := &cobra.Command{Use: "bare"}
	_, err := requireTenant(cmd)
	require.Error(t, err)
}

func TestOptToken_ReturnsValueWhenSet(t *testing.T) {
	cmd := newTestCmd("t1", "tok-123", "http://api")
	assert.Equal(t, "tok-123", optToken(cmd))
}

func TestOptToken_ReturnsEmptyWhenUnset(t *testing.T) {
	cmd := newTestCmd("t1", "", "http://api")
	assert.Empty(t, optToken(cmd))
}

func TestOptToken_ReturnsEmptyWhenFlagAbsent(t *testing.T) {
	cmd := &cobra.Command{Use: "bare"}
	assert.Empty(t, optToken(cmd))
}

func TestAPIURL_ReturnsValueWhenSet(t *testing.T) {
	cmd := newTestCmd("t1", "", "https://paladin.example.com")
	assert.Equal(t, "https://paladin.example.com", apiURL(cmd))
}

func TestAPIURL_DefaultsWhenFlagAbsent(t *testing.T) {
	cmd := &cobra.Command{Use: "bare"}
	assert.Equal(t, "http://localhost:8080", apiURL(cmd))
}

func TestEnvStr_ReturnsFallbackWhenEnvUnset(t *testing.T) {
	t.Setenv("PALADIN_TEST_VAR", "")
	assert.Equal(t, "default", envStr("PALADIN_TEST_VAR", "default"))
}

func TestEnvStr_ReturnsEnvValueWhenSet(t *testing.T) {
	t.Setenv("PALADIN_TEST_VAR", "from-env")
	assert.Equal(t, "from-env", envStr("PALADIN_TEST_VAR", "default"))
}
