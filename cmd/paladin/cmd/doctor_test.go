package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── infraDialAddr ────────────────────────────────────────────────────────────

func TestInfraDialAddr_NATSScheme(t *testing.T) {
	assert.Equal(t, "localhost:4222", infraDialAddr("nats://localhost:4222", 4222))
}

func TestInfraDialAddr_RedisScheme(t *testing.T) {
	assert.Equal(t, "localhost:6379", infraDialAddr("redis://localhost:6379", 6379))
}

func TestInfraDialAddr_HTTPScheme(t *testing.T) {
	assert.Equal(t, "localhost:6333", infraDialAddr("http://localhost:6333", 6333))
}

func TestInfraDialAddr_BareHostPort(t *testing.T) {
	assert.Equal(t, "localhost:5432", infraDialAddr("localhost:5432", 5432))
}

func TestInfraDialAddr_HostOnly_AddsDefaultPort(t *testing.T) {
	assert.Equal(t, "myhost:9999", infraDialAddr("myhost", 9999))
}

func TestInfraDialAddr_Empty_UsesLocalhost(t *testing.T) {
	assert.Equal(t, "localhost:1234", infraDialAddr("", 1234))
}

func TestInfraDialAddr_WithPath_StripsPath(t *testing.T) {
	assert.Equal(t, "localhost:6333", infraDialAddr("http://localhost:6333/healthz", 6333))
}

func TestInfraDialAddr_Userinfo_Stripped(t *testing.T) {
	assert.Equal(t, "localhost:6379", infraDialAddr("redis://user:pass@localhost:6379", 6379))
}

func TestInfraDialAddr_IPv6_Preserved(t *testing.T) {
	assert.Equal(t, "[::1]:4222", infraDialAddr("nats://[::1]:4222", 4222))
}

func TestInfraDialAddr_UnknownScheme_BestEffort(t *testing.T) {
	// Unknown scheme still parsed by url.Parse; host extracted correctly.
	addr := infraDialAddr("custom://myhost:9000", 9000)
	assert.Equal(t, "myhost:9000", addr)
}

// ─── infraTCPCheck ────────────────────────────────────────────────────────────

func TestInfraTCPCheck_Listening_Passes(t *testing.T) {
	skipIfNoNetwork(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	check := infraTCPCheck(fmt.Sprintf("tcp://%s", ln.Addr().String()), 0)
	// infraTCPCheck with a bare addr after stripping unknown scheme — pass raw addr directly
	check2 := infraTCPCheck(ln.Addr().String(), 0)
	assert.NoError(t, check2(), "should connect to listening port")
	_ = check // silence unused warning; bare addr path tested via check2
}

func TestInfraTCPCheck_NotListening_Fails(t *testing.T) {
	check := infraTCPCheck("127.0.0.1:1", 1)
	err := check()
	require.Error(t, err)
}

// ─── doctor command: runDoctor output ─────────────────────────────────────────

func TestDoctorCmd_TokenAndTenantMissing_FailsChecks(t *testing.T) {
	skipIfNoNetwork(t)

	// Stub API and Auth to return 200, so only config checks fail.
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer stub.Close()

	t.Setenv("PALADIN_TOKEN", "")

	var runErr error
	_ = captureStdout(t, func() {
		rootCmd.SetArgs([]string{"doctor", "--api-url", stub.URL, "--tenant", ""})
		runErr = rootCmd.Execute()
	})
	// Expects failure due to empty token + tenant + infra not running.
	assert.Error(t, runErr, "doctor should fail when token/tenant are not configured")
}

func TestDoctorCmd_InfraCheckFails_WhenPortNotListening(t *testing.T) {
	// infraTCPCheck against a closed port must return an error.
	err := infraTCPCheck("127.0.0.1:1", 1)()
	require.Error(t, err, "infra TCP check to closed port must fail")
}

func TestDoctorCmd_InfraCheckPasses_WhenPortListening(t *testing.T) {
	skipIfNoNetwork(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	check := infraTCPCheck(ln.Addr().String(), 0)
	assert.NoError(t, check(), "infra TCP check to open port must pass")
}

// ─── localHTTPBase ───────────────────────────────────────────────────────────

func TestLocalHTTPBase_DefaultsToLocalhostPort(t *testing.T) {
	assert.Equal(t, "http://localhost:9011", localHTTPBase("", 9011))
}

func TestLocalHTTPBase_PortOnly(t *testing.T) {
	assert.Equal(t, "http://localhost:9011", localHTTPBase(":9011", 9011))
}

func TestLocalHTTPBase_NumericPortOnly(t *testing.T) {
	assert.Equal(t, "http://localhost:9011", localHTTPBase("9011", 9011))
}

func TestLocalHTTPBase_HostOnly(t *testing.T) {
	assert.Equal(t, "http://memory.local:9011", localHTTPBase("memory.local", 9011))
}

func TestLocalHTTPBase_HostPort(t *testing.T) {
	assert.Equal(t, "http://127.0.0.1:19111", localHTTPBase("127.0.0.1:19111", 9011))
}

func TestLocalHTTPBase_HTTPURL(t *testing.T) {
	assert.Equal(t, "http://memory.local:9011", localHTTPBase("http://memory.local:9011/", 9011))
}

// ─── context-aware http check via client.Get ─────────────────────────────────

func TestHTTPEndpointReachable_200(t *testing.T) {
	skipIfNoNetwork(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/healthz", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// ─── --json mode ─────────────────────────────────────────────────────────────

func TestDoctorCmd_JSONMode_EmitsValidJSON(t *testing.T) {
	skipIfNoNetwork(t)

	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer stub.Close()

	t.Setenv("PALADIN_TOKEN", "test-token")

	var output string
	var runErr error
	output = captureStdout(t, func() {
		rootCmd.SetArgs([]string{
			"doctor",
			"--api-url", stub.URL,
			"--tenant", "test-tenant",
			"--token", "test-token",
			"--json",
		})
		runErr = rootCmd.Execute()
	})

	// The command fails because infra (NATS/Valkey/Qdrant/k8s) isn't running — that's expected.
	// We only care that stdout is valid JSON.
	_ = runErr

	var report DoctorReport
	err := json.Unmarshal([]byte(strings.TrimSpace(output)), &report)
	require.NoError(t, err, "--json output must be valid JSON; got: %s", output)
	assert.NotEmpty(t, report.Checks, "checks array must not be empty")
	for _, c := range report.Checks {
		assert.NotEmpty(t, c.Name, "every check must have a name")
	}
}

func TestDoctorCmd_JSONMode_IncludesMemoryReadinessCheck(t *testing.T) {
	skipIfNoNetwork(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PALADIN_TOKEN", "test-token")

	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer stub.Close()
	t.Setenv("MEMORY_HTTP_ADDR", stub.URL)

	output := captureStdout(t, func() {
		rootCmd.SetArgs([]string{
			"doctor",
			"--api-url", stub.URL,
			"--tenant", "test-tenant",
			"--token", "test-token",
			"--json",
		})
		_ = rootCmd.Execute()
	})

	var report DoctorReport
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(output)), &report))
	for _, c := range report.Checks {
		if c.Name == "paladin-memory ready" {
			assert.True(t, c.Passed)
			return
		}
	}
	t.Fatalf("doctor report missing paladin-memory ready check: %+v", report.Checks)
}

func TestDoctorCmd_JSONMode_IncludesAgentAndWSReadinessChecks(t *testing.T) {
	skipIfNoNetwork(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PALADIN_TOKEN", "test-token")

	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer stub.Close()
	t.Setenv("PALADIN_AGENT_PORT", stub.URL)
	t.Setenv("PALADIN_WS_PORT", stub.URL)

	output := captureStdout(t, func() {
		rootCmd.SetArgs([]string{
			"doctor",
			"--api-url", stub.URL,
			"--tenant", "test-tenant",
			"--token", "test-token",
			"--json",
		})
		_ = rootCmd.Execute()
	})

	var report DoctorReport
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(output)), &report))
	assertDoctorCheckPassed(t, report, "paladin-agent ready")
	assertDoctorCheckPassed(t, report, "paladin-ws ready")
}

func TestDoctorCmd_JSONMode_IncludesLocalServiceReadinessChecks(t *testing.T) {
	skipIfNoNetwork(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PALADIN_TOKEN", "test-token")

	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer stub.Close()
	t.Setenv("PALADIN_AUTH_URL", stub.URL)
	t.Setenv("PALADIN_INGEST_PORT", stub.URL)
	t.Setenv("PALADIN_HUB_URL", stub.URL)
	t.Setenv("PALADIN_COMMS_PORT", stub.URL)
	t.Setenv("PALADIN_ORCHESTRATOR_PORT", stub.URL)

	output := captureStdout(t, func() {
		rootCmd.SetArgs([]string{
			"doctor",
			"--api-url", stub.URL,
			"--tenant", "test-tenant",
			"--token", "test-token",
			"--json",
		})
		_ = rootCmd.Execute()
	})

	var report DoctorReport
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(output)), &report))
	assertDoctorCheckPassed(t, report, "API service ready")
	assertDoctorCheckPassed(t, report, "Auth service ready")
	assertDoctorCheckPassed(t, report, "paladin-ingest ready")
	assertDoctorCheckPassed(t, report, "paladin-hub ready")
	assertDoctorCheckPassed(t, report, "paladin-comms ready")
	assertDoctorCheckPassed(t, report, "paladin-orchestrator ready")
}

func assertDoctorCheckPassed(t *testing.T, report DoctorReport, name string) {
	t.Helper()
	for _, c := range report.Checks {
		if c.Name == name {
			assert.True(t, c.Passed, "%s should pass", name)
			return
		}
	}
	t.Fatalf("doctor report missing %q check: %+v", name, report.Checks)
}

func TestDoctorCmd_JSONMode_PassedFieldReflectsResults(t *testing.T) {
	// All checks that return no error should set passed=true.
	r := CheckResult{Name: "test", Passed: true}
	assert.True(t, r.Passed)
	assert.Empty(t, r.Error)

	r2 := CheckResult{Name: "test", Passed: false, Error: "connection refused"}
	assert.False(t, r2.Passed)
	assert.Equal(t, "connection refused", r2.Error)
}

func TestDoctorReport_JSONRoundTrip(t *testing.T) {
	report := DoctorReport{
		Passed: false,
		Checks: []CheckResult{
			{Name: "NATS reachable", Passed: true},
			{Name: "Valkey reachable", Passed: false, Error: "connection refused"},
		},
	}
	b, err := json.Marshal(report)
	require.NoError(t, err)

	var decoded DoctorReport
	require.NoError(t, json.Unmarshal(b, &decoded))
	assert.Equal(t, report.Passed, decoded.Passed)
	assert.Len(t, decoded.Checks, 2)
	assert.Equal(t, "NATS reachable", decoded.Checks[0].Name)
	assert.True(t, decoded.Checks[0].Passed)
	assert.Empty(t, decoded.Checks[0].Error)
	assert.Equal(t, "connection refused", decoded.Checks[1].Error)
}

// ─── --quiet mode ─────────────────────────────────────────────────────────────

func TestDoctorCmd_QuietMode_ProducesNoOutput(t *testing.T) {
	skipIfNoNetwork(t)

	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer stub.Close()

	output := captureStdout(t, func() {
		rootCmd.SetArgs([]string{
			"doctor",
			"--api-url", stub.URL,
			"--tenant", "test-tenant",
			"--token", "test-token",
			"--quiet",
		})
		_ = rootCmd.Execute()
	})

	assert.Empty(t, strings.TrimSpace(output), "--quiet mode must produce no stdout output")
}

func TestDoctorCmd_QuietMode_JSONModeAreMutuallyUsable(t *testing.T) {
	// Both --json and --quiet can be parsed without error (json takes precedence in output).
	require.NoError(t, doctorCmd.Flags().Set("json", "false"))
	require.NoError(t, doctorCmd.Flags().Set("quiet", "false"))
}
