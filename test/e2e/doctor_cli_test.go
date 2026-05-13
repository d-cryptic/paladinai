//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDoctorCLI_TenantSlugWithJWTClaimPasses(t *testing.T) {
	waitForReadyz(t, edgeURL(), 10*time.Second)
	waitForReadyz(t, authURL(), 10*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	slug := fmt.Sprintf("e2e-doctor-%d", time.Now().UnixNano())
	tenantID := createTenant(t, ctx, slug)
	token := issueToken(t, ctx, tenantID, "doctor-user")

	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/paladin", "doctor",
		"--json",
		"--api-url", edgeURL(),
		"--tenant", slug,
		"--token", token,
	)
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(),
		"PALADIN_API_URL="+edgeURL(),
		"PALADIN_AUTH_URL="+authURL(),
		"PALADIN_TENANT="+slug,
		"PALADIN_TOKEN="+token,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	var report struct {
		Checks []struct {
			Name   string `json:"name"`
			Passed bool   `json:"passed"`
			Error  string `json:"error,omitempty"`
		} `json:"checks"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &report), "stdout:\n%s\nstderr:\n%s\nerr:%v", stdout.String(), stderr.String(), err)
	requireDoctorCheckPassed(t, report.Checks, "API service ready")
	requireDoctorCheckPassed(t, report.Checks, "Auth service ready")
	requireDoctorCheckPassed(t, report.Checks, "Token configured")
	requireDoctorCheckPassed(t, report.Checks, "Tenant configured")
	requireDoctorCheckPassed(t, report.Checks, "MCP servers registered")
	requireDoctorCheckPassed(t, report.Checks, "NATS reachable")
	requireDoctorCheckPassed(t, report.Checks, "Valkey reachable")
	requireDoctorCheckPassed(t, report.Checks, "paladin-ingest ready")
	requireDoctorCheckPassed(t, report.Checks, "paladin-hub ready")
	requireDoctorCheckPassed(t, report.Checks, "paladin-memory ready")
	requireDoctorCheckPassed(t, report.Checks, "paladin-agent ready")
	requireDoctorCheckPassed(t, report.Checks, "paladin-ws ready")
	requireDoctorCheckPassed(t, report.Checks, "paladin-comms ready")
	requireDoctorCheckPassed(t, report.Checks, "paladin-orchestrator ready")
}

func createTenant(t *testing.T, ctx context.Context, slug string) string {
	t.Helper()
	body := fmt.Sprintf(`{"slug":%q,"name":"Doctor E2E Tenant"}`, slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authURL()+"/api/v1/tenants", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Secret", adminSecret())

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var out struct {
		Tenant struct {
			ID string `json:"id"`
		} `json:"tenant"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.NotEmpty(t, out.Tenant.ID)
	return out.Tenant.ID
}

func issueToken(t *testing.T, ctx context.Context, tenantID, userID string) string {
	t.Helper()
	body := fmt.Sprintf(`{"tenant_id":%q,"user_id":%q}`, tenantID, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authURL()+"/api/v1/tokens", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Secret", adminSecret())

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var out struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.NotEmpty(t, out.Token)
	return out.Token
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		require.NotEqual(t, wd, parent, "could not find repo root from %s", wd)
		wd = parent
	}
}

func requireDoctorCheckPassed(t *testing.T, checks []struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Error  string `json:"error,omitempty"`
}, name string) {
	t.Helper()
	for _, check := range checks {
		if check.Name == name {
			require.True(t, check.Passed, "%s failed: %s", check.Name, check.Error)
			return
		}
	}
	t.Fatalf("doctor report missing %q check: %+v", name, checks)
}
