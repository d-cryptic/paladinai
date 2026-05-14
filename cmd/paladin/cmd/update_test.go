package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateCmd_PrintsCommandWithoutRunning(t *testing.T) {
	restoreUpdateTestGlobals(t)
	Version = "v2.0.0"
	Commit = "abc123"
	updateLookPath = func(string) (string, error) {
		return "/opt/homebrew/bin/brew", nil
	}
	updateRunCmd = func(context.Context, string, ...string) error {
		t.Fatal("update command must not run without --yes")
		return nil
	}

	srv := latestVersionServer(t, `{"version":"v2.1.0"}`)
	defer srv.Close()

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"update", "--latest-url", srv.URL})
		require.NoError(t, rootCmd.Execute())
	})

	assert.Contains(t, out, "update available")
	assert.Contains(t, out, "current: v2.0.0")
	assert.Contains(t, out, "latest:  v2.1.0")
	assert.Contains(t, out, "command: brew upgrade paladinai/tap/paladin")
}

func TestUpdateCmd_YesRunsSelectedCommand(t *testing.T) {
	restoreUpdateTestGlobals(t)
	Version = "v2.0.0"
	var gotName string
	var gotArgs []string
	updateLookPath = func(string) (string, error) {
		return "", errors.New("brew missing")
	}
	updateRunCmd = func(_ context.Context, name string, args ...string) error {
		gotName = name
		gotArgs = append([]string{}, args...)
		return nil
	}

	srv := latestVersionServer(t, `{"version":"v2.1.0"}`)
	defer srv.Close()

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"update", "--yes", "--latest-url", srv.URL})
		require.NoError(t, rootCmd.Execute())
	})

	assert.Equal(t, "sh", gotName)
	require.Equal(t, []string{"-c", "curl -fsSL https://get.paladinai.io | sh"}, gotArgs)
	assert.Contains(t, out, "paladin update completed")
}

func TestUpdateCmd_JSONUpToDate(t *testing.T) {
	restoreUpdateTestGlobals(t)
	Version = "v2.1.0"

	srv := latestVersionServer(t, `{"version":"v2.1.0"}`)
	defer srv.Close()

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"--ci", "update", "--latest-url", srv.URL})
		require.NoError(t, rootCmd.Execute())
	})

	var result updateResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "v2.1.0", result.Current)
	assert.Equal(t, "v2.1.0", result.Latest)
	assert.False(t, result.UpdateAvailable)
	assert.False(t, result.Updated)
	assert.Contains(t, result.Message, "up to date")
}

func TestUpdateCommand_UsesInstallScriptWhenBrewMissing(t *testing.T) {
	restoreUpdateTestGlobals(t)
	updateLookPath = func(string) (string, error) {
		return "", errors.New("not found")
	}

	name, args, display := updateCommand()

	assert.Equal(t, "sh", name)
	assert.Equal(t, []string{"-c", "curl -fsSL https://get.paladinai.io | sh"}, args)
	assert.True(t, strings.Contains(display, "get.paladinai.io"))
}

func latestVersionServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
}

func restoreUpdateTestGlobals(t *testing.T) {
	t.Helper()
	oldVersion := Version
	oldCommit := Commit
	oldLookPath := updateLookPath
	oldRunCmd := updateRunCmd
	t.Cleanup(func() {
		Version = oldVersion
		Commit = oldCommit
		updateLookPath = oldLookPath
		updateRunCmd = oldRunCmd
	})
}
