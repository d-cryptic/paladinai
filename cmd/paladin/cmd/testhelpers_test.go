// Package-level test helpers shared across cmd test files.
// Tests using captureStdout MUST remain serial (no t.Parallel) because it
// mutates the process-level os.Stdout global.
package cmd

import (
	"bytes"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// rootCmdMu serialises all tests that drive rootCmd.Execute to prevent
// concurrent mutation of os.Stdout and cobra's internal state.
var rootCmdMu sync.Mutex

// captureStdout redirects os.Stdout to a pipe for the duration of fn, then
// returns everything written. t.Cleanup restores os.Stdout so the test
// infrastructure always sees the original writer even if fn panics.
//
// A package-level mutex (rootCmdMu) is held for the duration so that
// concurrent tests that share rootCmd do not race on os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	stdout, _ := captureOutput(t, fn)
	return stdout
}

func captureOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()
	rootCmdMu.Lock()
	defer rootCmdMu.Unlock()

	stdoutR, stdoutW, err := os.Pipe()
	require.NoError(t, err)
	stderrR, stderrW, err := os.Pipe()
	require.NoError(t, err)

	origStdout := os.Stdout
	origStderr := os.Stderr
	os.Stdout = stdoutW
	os.Stderr = stderrW
	t.Cleanup(func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
	}) // runs even on panic

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	stdoutDone := make(chan struct{})
	stderrDone := make(chan struct{})
	go func() {
		defer close(stdoutDone)
		_, _ = io.Copy(&stdoutBuf, stdoutR)
	}()
	go func() {
		defer close(stderrDone)
		_, _ = io.Copy(&stderrBuf, stderrR)
	}()

	fn()

	// Reset known boolean flags that persist across Execute() calls.
	// cobra/pflag retain flag values between runs of the same command object.
	resetBoolFlag(doctorCmd.Flags(), "json")
	resetBoolFlag(doctorCmd.Flags(), "quiet")
	resetBoolFlag(initCmd.Flags(), "dry-run")
	resetBoolFlag(initCmd.Flags(), "tui")
	resetBoolFlag(versionCmd.Flags(), "check")
	resetBoolFlag(updateCmd.Flags(), "dry-run")
	resetBoolFlag(updateCmd.Flags(), "yes")
	resetBoolFlag(rootCmd.PersistentFlags(), "ci")
	resetBoolFlag(rootCmd.PersistentFlags(), "simple")
	resetBoolFlag(rootCmd.PersistentFlags(), "tour")
	rootCmd.SilenceErrors = false
	rootCmd.SilenceUsage = false
	doctorCmd.SilenceUsage = false

	stdoutW.Close() // signal EOF to the drain goroutine
	stderrW.Close()
	<-stdoutDone // wait for full drain before reading buffers
	<-stderrDone
	return stdoutBuf.String(), stderrBuf.String()
}

// resetBoolFlag resets a pflag bool to false without failing if the flag doesn't exist.
func resetBoolFlag(fs interface{ Set(string, string) error }, name string) {
	_ = fs.Set(name, "false")
}
