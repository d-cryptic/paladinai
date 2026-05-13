package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// serviceSpec describes one service process in the all-in-one stack.
type serviceSpec struct {
	name   string
	binary string // binary name under bin/ or $PATH
	port   string // informational only, for startup message
}

// defaultServices is the ordered list of services started by `paladin server`.
// Ingest and edge start last so NATS consumers are ready before traffic arrives.
var defaultServices = []serviceSpec{
	{name: "paladin-hub", binary: "paladin-hub", port: "8082"},
	{name: "paladin-memory", binary: "paladin-memory", port: "8083"},
	{name: "paladin-auth", binary: "paladin-auth", port: "8084"},
	{name: "paladin-orchestrator", binary: "paladin-orchestrator", port: "8085"},
	{name: "paladin-agent", binary: "paladin-agent", port: "8086"},
	{name: "paladin-ingest", binary: "paladin-ingest", port: "9001"},
	{name: "paladin-edge", binary: "paladin-edge", port: "9002"},
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start all PaladinAI services in one process (dev mode)",
	Long: `Start all PaladinAI microservices as child processes within a single terminal session.

This command is intended for local development and smoke testing only.
For production deployments, use Docker Compose or Kubernetes (see docs/plans/).

All services inherit the current environment. Set external service URLs via:
  NATS_URL          (default: nats://localhost:4222)
  DATABASE_URL      (default: postgres://localhost:5432/paladin)
  VALKEY_URL        (default: redis://localhost:6379)
  OPENROUTER_API_KEY (required for agent)

Services started:
  paladin-hub         :8082
  paladin-memory      :8083
  paladin-auth        :8084
  paladin-orchestrator :8085
  paladin-agent       :8086
  paladin-ingest      :9001
  paladin-edge        :9002`,
	RunE: runServer,
}

func init() {
	serverCmd.Flags().String("bin-dir", "", "Directory containing service binaries (default: ./bin)")
	serverCmd.Flags().Duration("startup-delay", 500*time.Millisecond, "Delay between starting each service")
	serverCmd.Flags().StringSlice("only", nil, "Start only these services (comma-separated names)")
	rootCmd.AddCommand(serverCmd)
}

func runServer(cmd *cobra.Command, _ []string) error {
	binDir, _ := cmd.Flags().GetString("bin-dir")
	if binDir == "" {
		binDir = resolveBinDir()
	}
	delay, _ := cmd.Flags().GetDuration("startup-delay")
	only, _ := cmd.Flags().GetStringSlice("only")

	services := filterServices(defaultServices, only)
	if len(services) == 0 {
		return fmt.Errorf("no services to start (check --only filter)")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Fprintf(os.Stdout, "Starting %d PaladinAI service(s)...\n", len(services))
	fmt.Fprintln(os.Stdout, "Press Ctrl+C to stop all services.")

	var (
		mu   sync.Mutex
		cmds []*exec.Cmd
		wg   sync.WaitGroup
	)

	for i, svc := range services {
		binPath := resolveBinary(binDir, svc.binary)
		if binPath == "" {
			fmt.Fprintf(os.Stderr, "  [SKIP] %s — binary not found in %s or $PATH\n", svc.name, binDir)
			continue
		}

		c := exec.CommandContext(ctx, binPath) //nolint:gosec
		c.Stdout = prefixWriter(os.Stdout, svc.name)
		c.Stderr = prefixWriter(os.Stderr, svc.name)
		c.Env = os.Environ()

		if err := c.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "  [ERROR] %s: %v\n", svc.name, err)
			continue
		}

		fmt.Fprintf(os.Stdout, "  [OK] %-24s started (pid %d, port %s)\n", svc.name, c.Process.Pid, svc.port)

		mu.Lock()
		cmds = append(cmds, c)
		mu.Unlock()

		wg.Add(1)
		go func(svc serviceSpec, c *exec.Cmd) {
			defer wg.Done()
			if err := c.Wait(); err != nil {
				// Context cancellation produces an expected error — don't log it.
				if ctx.Err() == nil {
					fmt.Fprintf(os.Stderr, "  [EXIT] %s exited unexpectedly: %v\n", svc.name, err)
				}
			}
		}(svc, c)

		// Stagger startup so dependencies are ready before dependents start.
		if i < len(services)-1 {
			select {
			case <-ctx.Done():
				goto shutdown
			case <-time.After(delay):
			}
		}
	}

	fmt.Fprintf(os.Stdout, "\nAll services running. Waiting for shutdown signal...\n")
	<-ctx.Done()

shutdown:
	fmt.Fprintf(os.Stdout, "\nShutting down...\n")

	// Signal all processes. exec.CommandContext already sends SIGKILL on cancel;
	// we send SIGTERM first for graceful shutdown.
	mu.Lock()
	for _, c := range cmds {
		if c.Process != nil {
			_ = c.Process.Signal(syscall.SIGTERM)
		}
	}
	mu.Unlock()

	// Give services up to 10 seconds to exit gracefully.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Fprintln(os.Stdout, "All services stopped.")
	case <-time.After(10 * time.Second):
		fmt.Fprintln(os.Stderr, "Timeout waiting for graceful shutdown; forcing exit.")
		mu.Lock()
		for _, c := range cmds {
			if c.Process != nil {
				_ = c.Process.Kill()
			}
		}
		mu.Unlock()
	}

	return nil
}

// resolveBinDir returns the best-guess bin/ directory relative to the binary.
func resolveBinDir() string {
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "..", "bin")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "bin"
}

// resolveBinary finds the binary in binDir first, then $PATH.
func resolveBinary(binDir, name string) string {
	candidate := filepath.Join(binDir, name)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}

// filterServices returns only services in the allowlist (case-insensitive).
// If allowlist is empty all services are returned.
func filterServices(all []serviceSpec, only []string) []serviceSpec {
	if len(only) == 0 {
		return all
	}
	set := make(map[string]bool, len(only))
	for _, n := range only {
		set[strings.TrimSpace(strings.ToLower(n))] = true
	}
	out := make([]serviceSpec, 0, len(only))
	for _, svc := range all {
		if set[strings.ToLower(svc.name)] {
			out = append(out, svc)
		}
	}
	return out
}

// prefixWriter returns a writer that prepends "[name] " to each line.
type linePrefixWriter struct {
	dst    *os.File
	prefix string
	buf    []byte
}

func prefixWriter(dst *os.File, name string) *linePrefixWriter {
	return &linePrefixWriter{dst: dst, prefix: fmt.Sprintf("[%-20s] ", name)}
}

const linePrefixWriterMaxBuf = 64 * 1024

func (w *linePrefixWriter) Write(p []byte) (n int, err error) {
	w.buf = append(w.buf, p...)
	for {
		idx := bytes.IndexByte(w.buf, '\n')
		if idx < 0 {
			break
		}
		line := string(w.buf[:idx+1])
		w.buf = w.buf[idx+1:]
		if w.dst != nil {
			_, _ = fmt.Fprint(w.dst, w.prefix+line)
		}
	}
	// Prevent unbounded growth from a misbehaving child that never writes newlines.
	if len(w.buf) > linePrefixWriterMaxBuf {
		if w.dst != nil {
			_, _ = fmt.Fprint(w.dst, w.prefix+string(w.buf))
		}
		w.buf = w.buf[:0]
	}
	return len(p), nil
}
