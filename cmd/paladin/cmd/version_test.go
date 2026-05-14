package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionCmd_OutputContainsVersion(t *testing.T) {
	oldVersion := Version
	oldCommit := Commit
	t.Cleanup(func() {
		Version = oldVersion
		Commit = oldCommit
	})
	Version = "v2.0.0-test"
	Commit = "abc123"

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"version"})
		_ = rootCmd.Execute()
	})

	assert.True(t, strings.Contains(out, "v2.0.0-test"), "output should contain version")
	assert.True(t, strings.Contains(out, "abc123"), "output should contain commit")
}

func TestWriteVersionResult_CIModeJSON(t *testing.T) {
	cmd := &cobra.Command{Use: "version"}
	cmd.Flags().Bool("ci", true, "")

	out := captureStdout(t, func() {
		require.NoError(t, writeVersionResult(cmd, versionResult{Version: "v2.0.0-test", Commit: "abc123"}))
	})

	var result versionResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "v2.0.0-test", result.Version)
	assert.Equal(t, "abc123", result.Commit)
	assert.NotContains(t, out, "paladin ")
}

func TestVersionCmd_CheckReportsUpdate(t *testing.T) {
	oldVersion := Version
	oldCommit := Commit
	t.Cleanup(func() {
		Version = oldVersion
		Commit = oldCommit
	})
	Version = "v2.0.0"
	Commit = "abc123"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version":"v2.1.0","upgrade":"brew upgrade paladinai/tap/paladin"}`))
	}))
	defer srv.Close()

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"version", "--check", "--latest-url", srv.URL})
		require.NoError(t, rootCmd.Execute())
	})

	assert.Contains(t, out, "paladin v2.0.0 (abc123)")
	assert.Contains(t, out, "latest  v2.1.0")
	assert.Contains(t, out, "update available: v2.0.0 -> v2.1.0")
	assert.Contains(t, out, "brew upgrade paladinai/tap/paladin")
}

func TestVersionCmd_CheckJSON(t *testing.T) {
	oldVersion := Version
	oldCommit := Commit
	t.Cleanup(func() {
		Version = oldVersion
		Commit = oldCommit
	})
	Version = "v2.0.0"
	Commit = "abc123"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version":"v2.1.0"}`))
	}))
	defer srv.Close()

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"--ci", "version", "--check", "--latest-url", srv.URL})
		require.NoError(t, rootCmd.Execute())
	})

	var result versionResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "v2.0.0", result.Version)
	assert.Equal(t, "v2.1.0", result.Latest)
	assert.True(t, result.UpdateAvailable)
	assert.NotEmpty(t, result.Upgrade)
}

func TestFetchLatestVersionRejectsOversizedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("x", maxLatestVersionBytes+1)))
	}))
	defer srv.Close()

	_, err := fetchLatestVersion(t.Context(), srv.URL, "v2.0.0")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "latest version response exceeds")
}

func TestRunBackgroundUpdateCheckSkipsWhenRecent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Date(2026, 5, 14, 6, 0, 0, 0, time.UTC)
	oldClock := updateCheckClock
	t.Cleanup(func() { updateCheckClock = oldClock })
	updateCheckClock = func() time.Time { return now }

	require.NoError(t, saveConfig(&PaladinConfig{
		OutputFormat:    "table",
		LastUpdateCheck: now.Add(-time.Hour),
	}))

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	defer srv.Close()

	var stderr bytes.Buffer
	require.NoError(t, runBackgroundUpdateCheck(t.Context(), srv.URL, "v2.0.0", &stderr))
	assert.Equal(t, 0, calls)
	assert.Empty(t, stderr.String())
}

func TestRunBackgroundUpdateCheckWritesNoticeAndTimestamp(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Date(2026, 5, 14, 6, 0, 0, 0, time.UTC)
	oldClock := updateCheckClock
	t.Cleanup(func() { updateCheckClock = oldClock })
	updateCheckClock = func() time.Time { return now }

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version":"v2.1.0","upgrade":"paladin update"}`))
	}))
	defer srv.Close()

	var stderr bytes.Buffer
	require.NoError(t, runBackgroundUpdateCheck(t.Context(), srv.URL, "v2.0.0", &stderr))

	assert.Contains(t, stderr.String(), "paladin update available: v2.0.0 -> v2.1.0")
	cfg, err := loadConfig()
	require.NoError(t, err)
	assert.True(t, cfg.LastUpdateCheck.Equal(now))
}

func TestIsNewerVersion(t *testing.T) {
	assert.True(t, isNewerVersion("v2.1.0", "v2.0.9"))
	assert.True(t, isNewerVersion("2.0.1", "2.0.0"))
	assert.False(t, isNewerVersion("v2.0.0", "v2.0.0"))
	assert.False(t, isNewerVersion("v1.9.9", "v2.0.0"))
	assert.False(t, isNewerVersion("v2.1.0", "dev"))
}
