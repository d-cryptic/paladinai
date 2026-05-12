package integrationpkg

import (
	"testing"
)

func TestCredentialKey_Formatting(t *testing.T) {
	tests := []struct {
		integration string
		key         string
		want        string
	}{
		{"datadog", "api_key", "PALADIN_DATADOG_API_KEY"},
		{"pagerduty", "api_key", "PALADIN_PAGERDUTY_API_KEY"},
		{"slack", "bot_token", "PALADIN_SLACK_BOT_TOKEN"},
		{"prometheus", "bearer_token", "PALADIN_PROMETHEUS_BEARER_TOKEN"},
		{"grafana", "api_key", "PALADIN_GRAFANA_API_KEY"},
		{"alert-manager", "token", "PALADIN_ALERT_MANAGER_TOKEN"},
	}
	for _, tc := range tests {
		got := CredentialKey(tc.integration, tc.key)
		if got != tc.want {
			t.Errorf("CredentialKey(%q, %q) = %q, want %q", tc.integration, tc.key, got, tc.want)
		}
	}
}

func TestResolveCredential_Found(t *testing.T) {
	t.Setenv("PALADIN_DATADOG_API_KEY", "dd-secret-value")
	val, ok := ResolveCredential("datadog", "api_key")
	if !ok {
		t.Error("expected credential to be found")
	}
	if val != "dd-secret-value" {
		t.Errorf("ResolveCredential = %q, want dd-secret-value", val)
	}
}

func TestResolveCredential_NotFound(t *testing.T) {
	t.Setenv("PALADIN_DATADOG_API_KEY", "")
	_, ok := ResolveCredential("datadog", "api_key")
	if ok {
		t.Error("expected credential to be missing")
	}
}

func TestMustResolveCredential_Found(t *testing.T) {
	t.Setenv("PALADIN_SLACK_BOT_TOKEN", "xoxb-test")
	val, err := MustResolveCredential("slack", "bot_token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "xoxb-test" {
		t.Errorf("MustResolveCredential = %q, want xoxb-test", val)
	}
}

func TestMustResolveCredential_Missing_ReturnsError(t *testing.T) {
	t.Setenv("PALADIN_MISSING_KEY", "")
	_, err := MustResolveCredential("missing", "key")
	if err == nil {
		t.Error("expected error for missing credential")
	}
}

func TestResolveAll_ReturnsMatchingKeys(t *testing.T) {
	t.Setenv("PALADIN_PROMETHEUS_BEARER_TOKEN", "prom-token")
	t.Setenv("PALADIN_PROMETHEUS_URL", "http://prom:9090")
	t.Setenv("PALADIN_GRAFANA_API_KEY", "grafana-key") // different integration

	creds := ResolveAll("prometheus")
	if creds["bearer_token"] != "prom-token" {
		t.Errorf("bearer_token = %q, want prom-token", creds["bearer_token"])
	}
	if creds["url"] != "http://prom:9090" {
		t.Errorf("url = %q, want http://prom:9090", creds["url"])
	}
	if _, ok := creds["api_key"]; ok {
		t.Error("grafana api_key should not appear in prometheus ResolveAll")
	}
}

func TestResolveAll_EmptyWhenNoMatch(t *testing.T) {
	creds := ResolveAll("nonexistent-integration-xyz")
	if len(creds) != 0 {
		t.Errorf("expected empty map, got %v", creds)
	}
}
