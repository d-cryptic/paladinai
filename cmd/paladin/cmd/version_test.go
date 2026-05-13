package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionCmd_OutputContainsVersion(t *testing.T) {
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
