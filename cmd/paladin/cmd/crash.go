package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

const crashReportURL = "https://github.com/paladinai/cli/issues"

var crashClock = func() time.Time { return time.Now().UTC() }

// ExecuteWithRecovery runs the CLI and converts unexpected panics into a
// user-facing error plus a local crash log.
func ExecuteWithRecovery() (exitCode int) {
	defer func() {
		if recovered := recover(); recovered != nil {
			exitCode = handlePanic(recovered, os.Args, os.Stderr)
		}
	}()
	if err := Execute(); err != nil {
		return 1
	}
	return 0
}

func handlePanic(recovered any, args []string, stderr io.Writer) int {
	errorID := crashErrorID(crashClock())
	logPath, logErr := writeCrashLog(errorID, recovered, args)

	fmt.Fprintln(stderr, "paladin encountered an unexpected error.")
	fmt.Fprintln(stderr)
	fmt.Fprintf(stderr, "Please report this at %s with the following:\n", crashReportURL)
	fmt.Fprintln(stderr)
	fmt.Fprintf(stderr, "  Version: paladin %s (%s) %s/%s\n", Version, Commit, runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(stderr, "  Command: %s\n", shellCommand(args))
	fmt.Fprintf(stderr, "  Error ID: %s\n", errorID)
	if logErr != nil {
		fmt.Fprintf(stderr, "  Crash log: unavailable (%v)\n", logErr)
	} else {
		fmt.Fprintf(stderr, "  Crash log: %s\n", logPath)
	}
	return 1
}

func writeCrashLog(errorID string, recovered any, args []string) (string, error) {
	dir := filepath.Join(configDir(), "logs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "crash-"+errorID+".log")
	body := fmt.Sprintf("error_id: %s\nversion: %s\ncommit: %s\ncommand: %s\npanic: %v\n\n%s",
		errorID, Version, Commit, shellCommand(args), recovered, debug.Stack())
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func crashErrorID(t time.Time) string {
	return "err_" + t.UTC().Format("20060102T150405Z")
}

func shellCommand(args []string) string {
	if len(args) == 0 {
		return "paladin"
	}
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if arg != "" {
			parts = append(parts, arg)
		}
	}
	if len(parts) == 0 {
		return "paladin"
	}
	return strings.Join(parts, " ")
}
