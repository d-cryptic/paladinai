// Package-level test helpers shared across cmd test files.
// Tests using captureStdout MUST remain serial (no t.Parallel) because it
// mutates the process-level os.Stdout global.
package cmd

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// captureStdout redirects os.Stdout to a pipe for the duration of fn, then
// returns everything written. t.Cleanup restores os.Stdout so the test
// infrastructure always sees the original writer even if fn panics.
//
// On panic the drain goroutine is leaked (the write-end is never closed), but
// the test will fail anyway and the process will exit, so this is acceptable.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
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

	w.Close() // signal EOF to the drain goroutine
	<-done    // wait for full drain before reading buf
	return buf.String()
}
