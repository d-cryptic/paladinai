package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandlePanicWritesCrashLogAndUserMessage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	oldClock := crashClock
	crashClock = func() time.Time { return time.Date(2026, 5, 14, 12, 34, 56, 0, time.UTC) }
	t.Cleanup(func() { crashClock = oldClock })

	var stderr bytes.Buffer
	code := handlePanic("boom", []string{"paladin", "doctor"}, &stderr)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	output := stderr.String()
	for _, want := range []string{
		"paladin encountered an unexpected error",
		"Error ID: err_20260514T123456Z",
		"Command: paladin doctor",
		"Crash log:",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stderr missing %q:\n%s", want, output)
		}
	}

	path := filepath.Join(configDir(), "logs", "crash-err_20260514T123456Z.log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read crash log: %v", err)
	}
	log := string(data)
	for _, want := range []string{
		"error_id: err_20260514T123456Z",
		"command: paladin doctor",
		"panic: boom",
		"goroutine",
	} {
		if !strings.Contains(log, want) {
			t.Fatalf("log missing %q:\n%s", want, log)
		}
	}
}

func TestShellCommandDefaultsToPaladin(t *testing.T) {
	if got := shellCommand(nil); got != "paladin" {
		t.Fatalf("shellCommand(nil) = %q, want paladin", got)
	}
	if got := shellCommand([]string{"", "  "}); got != "paladin" {
		t.Fatalf("shellCommand(empty) = %q, want paladin", got)
	}
}
