package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
