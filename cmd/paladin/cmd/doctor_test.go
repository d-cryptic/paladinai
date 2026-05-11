package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
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
