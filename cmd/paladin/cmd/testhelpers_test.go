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

// executeCmd is a helper that acquires rootCmdMu and runs rootCmd.Execute().
// Use this instead of calling rootCmd.Execute() directly in tests that do not
// already go through captureStdout.
func executeCmd(args []string) error {
	rootCmdMu.Lock()
	defer rootCmdMu.Unlock()
	rootCmd.SetArgs(args)
	return rootCmd.Execute()
}

// captureStdout redirects os.Stdout to a pipe for the duration of fn, then
// returns everything written. t.Cleanup restores os.Stdout so the test
// infrastructure always sees the original writer even if fn panics.
//
// A package-level mutex (rootCmdMu) is held for the duration so that
// concurrent tests that share rootCmd do not race on os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	rootCmdMu.Lock()
	defer rootCmdMu.Unlock()

	r, w, err := os.Pipe()
	require.NoError(t, err)

	orig := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = orig }) // runs even on panic

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.Copy(&buf, r)
	}()

	fn()

	// Reset known boolean flags that persist across Execute() calls.
	// cobra/pflag retain flag values between runs of the same command object.
	resetBoolFlag(doctorCmd.Flags(), "json")
	resetBoolFlag(doctorCmd.Flags(), "quiet")
	rootCmd.SilenceErrors = false

	w.Close() // signal EOF to the drain goroutine
	<-done    // wait for full drain before reading buf
	return buf.String()
}

// resetBoolFlag resets a pflag bool to false without failing if the flag doesn't exist.
func resetBoolFlag(fs interface{ Set(string, string) error }, name string) {
	_ = fs.Set(name, "false")
}
