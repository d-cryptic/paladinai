package cmd

import (
	"encoding/json"
	"os"
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
	assert.Equal(t, defaultAPIURL, apiURL(cmd))
}

func TestOutputFormat_DefaultsToTable(t *testing.T) {
	cmd := newTestCmd("t1", "", "http://api")
	assert.Equal(t, "table", outputFormat(cmd))
}

func TestOutputFormat_UsesOutputFlag(t *testing.T) {
	cmd := newTestCmd("t1", "", "http://api")
	cmd.PersistentFlags().StringP("output", "o", "table", "")
	require.NoError(t, cmd.PersistentFlags().Set("output", "json"))
	assert.Equal(t, "json", outputFormat(cmd))
}

func TestOutputFormat_CIModeForcesJSON(t *testing.T) {
	cmd := newTestCmd("t1", "", "http://api")
	cmd.PersistentFlags().StringP("output", "o", "table", "")
	cmd.PersistentFlags().Bool("ci", false, "")
	require.NoError(t, cmd.PersistentFlags().Set("ci", "true"))
	assert.Equal(t, "json", outputFormat(cmd))
}

func TestOutputFormat_CIEnvForcesJSON(t *testing.T) {
	t.Setenv("PALADIN_CI", "1")
	cmd := newTestCmd("t1", "", "http://api")
	cmd.PersistentFlags().StringP("output", "o", "table", "")
	assert.Equal(t, "json", outputFormat(cmd))
}

func TestRootAPIURLFlagDefaultsToLocalEdge(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("api-url")
	require.NotNil(t, flag)
	assert.Equal(t, defaultAPIURL, flag.DefValue)
}

func TestRootLongDocumentsAuthEnvURL(t *testing.T) {
	assert.Contains(t, rootCmd.Long, "PALADIN_AUTH_URL")
	assert.Contains(t, rootCmd.Long, "http://localhost:9003")
}

func TestEnvStr_ReturnsFallbackWhenEnvUnset(t *testing.T) {
	t.Setenv("PALADIN_TEST_VAR", "")
	assert.Equal(t, "default", envStr("PALADIN_TEST_VAR", "default"))
}

func TestEnvStr_ReturnsEnvValueWhenSet(t *testing.T) {
	t.Setenv("PALADIN_TEST_VAR", "from-env")
	assert.Equal(t, "from-env", envStr("PALADIN_TEST_VAR", "default"))
}

func TestIsCIMode_FlagSet(t *testing.T) {
	c := newTestCmd("", "", "http://api")
	c.PersistentFlags().Bool("ci", false, "")
	_ = c.PersistentFlags().Set("ci", "true")
	if !isCIMode(c) {
		t.Error("expected isCIMode=true when --ci flag is set")
	}
}

func TestIsCIMode_EnvVar(t *testing.T) {
	t.Setenv("PALADIN_CI", "1")
	c := newTestCmd("", "", "http://api")
	if !isCIMode(c) {
		t.Error("expected isCIMode=true when PALADIN_CI env is set")
	}
}

func TestIsCIMode_NotSet(t *testing.T) {
	t.Setenv("PALADIN_CI", "")
	c := newTestCmd("", "", "http://api")
	if isCIMode(c) {
		t.Error("expected isCIMode=false when nothing is set")
	}
}

func TestTelemetryEnabled_Default(t *testing.T) {
	t.Setenv("PALADIN_NO_TELEMETRY", "")
	if !telemetryEnabled() {
		t.Error("expected telemetry=true by default")
	}
}

func TestTelemetryEnabled_OptOut(t *testing.T) {
	t.Setenv("PALADIN_NO_TELEMETRY", "1")
	if telemetryEnabled() {
		t.Error("expected telemetry=false when PALADIN_NO_TELEMETRY=1")
	}
}

func TestRunRoot_NonTTYShowsHelp(t *testing.T) {
	oldIsTerminal := rootIsTerminal
	t.Cleanup(func() { rootIsTerminal = oldIsTerminal })
	rootIsTerminal = func(uintptr) bool { return false }

	out := captureStdout(t, func() {
		require.NoError(t, runRoot(rootCmd, nil))
	})

	assert.Contains(t, out, "Available Commands:")
	assert.Contains(t, out, "dashboard")
}

func TestRunRoot_TTYLaunchesDashboardPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	oldIsTerminal := rootIsTerminal
	t.Cleanup(func() { rootIsTerminal = oldIsTerminal })
	rootIsTerminal = func(uintptr) bool { return true }

	cmd := newTestCmd("", "", "http://api")
	err := runRoot(cmd, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant ID is required")
}

func TestMaybeShowFirstRunTourWritesMarker(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cmd := newTestCmd("", "", "")

	out := captureStdout(t, func() {
		require.NoError(t, maybeShowFirstRunTour(cmd))
	})

	assert.Contains(t, out, "Welcome to PaladinAI")
	assert.FileExists(t, firstRunPath())
}

func TestMaybeShowFirstRunTourSkipsWhenMarkerExists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(configDir(), 0o700))
	require.NoError(t, os.WriteFile(firstRunPath(), []byte("shown\n"), 0o600))
	cmd := newTestCmd("", "", "")

	out := captureStdout(t, func() {
		require.NoError(t, maybeShowFirstRunTour(cmd))
	})

	assert.Empty(t, out)
}

func TestRunRoot_TourShowsQuickTour(t *testing.T) {
	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"--tour"})
		require.NoError(t, rootCmd.Execute())
	})

	assert.Contains(t, out, "Welcome to PaladinAI")
	assert.Contains(t, out, "live incident feed")
	assert.Contains(t, out, "keyboard shortcuts")
}

func TestWriteTour_JSON(t *testing.T) {
	cmd := newTestCmd("", "", "")
	cmd.PersistentFlags().Bool("ci", true, "")
	require.NoError(t, cmd.PersistentFlags().Set("ci", "true"))

	out := captureStdout(t, func() {
		require.NoError(t, writeTour(cmd))
	})

	var result tourResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	require.Len(t, result.Steps, 4)
	assert.Contains(t, result.Steps[0], "incident feed")
}

func TestRunRoot_SimpleModeUsesAlertListPath(t *testing.T) {
	cmd := newTestCmd("", "", "http://api")
	cmd.Flags().Bool("simple", true, "")

	err := runRoot(cmd, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant ID is required")
}

func TestRunRoot_JSONStreamUsesTailPath(t *testing.T) {
	cmd := newTestCmd("", "", "http://api")
	cmd.Flags().Bool("json-stream", true, "")

	err := runRoot(cmd, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant ID is required")
}
