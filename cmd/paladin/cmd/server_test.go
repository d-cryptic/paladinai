package cmd

import (
	"slices"
	"testing"
)

func TestFilterServices_EmptyAllowlist(t *testing.T) {
	got := filterServices(defaultServices, nil)
	if len(got) != len(defaultServices) {
		t.Errorf("empty allowlist: want %d services, got %d", len(defaultServices), len(got))
	}
}

func TestFilterServices_SingleService(t *testing.T) {
	got := filterServices(defaultServices, []string{"paladin-edge"})
	if len(got) != 1 {
		t.Fatalf("want 1 service, got %d", len(got))
	}
	if got[0].name != "paladin-edge" {
		t.Errorf("want paladin-edge, got %s", got[0].name)
	}
}

func TestFilterServices_CaseInsensitive(t *testing.T) {
	got := filterServices(defaultServices, []string{"PALADIN-INGEST", "Paladin-Edge"})
	if len(got) != 2 {
		t.Errorf("want 2 services, got %d: %v", len(got), got)
	}
}

func TestFilterServices_UnknownName(t *testing.T) {
	got := filterServices(defaultServices, []string{"does-not-exist"})
	if len(got) != 0 {
		t.Errorf("want 0 services for unknown name, got %d", len(got))
	}
}

func TestFilterServices_OrderPreserved(t *testing.T) {
	// filterServices returns services in the order they appear in 'all', not 'only'.
	got := filterServices(defaultServices, []string{"paladin-edge", "paladin-hub"})
	if len(got) != 2 {
		t.Fatalf("want 2 services, got %d", len(got))
	}
	// hub comes before edge in defaultServices
	if got[0].name != "paladin-hub" || got[1].name != "paladin-edge" {
		t.Errorf("wrong order: %v", got)
	}
}

func TestResolveBinary_MissingBothPathAndBinDir(t *testing.T) {
	result := resolveBinary("/nonexistent/bindir", "this-binary-does-not-exist-xyz")
	if result != "" {
		t.Errorf("expected empty string for missing binary, got %q", result)
	}
}

func TestLinePrefixWriter_WritesPrefix(t *testing.T) {
	// Verify the writer compiles and Write() returns correct length.
	// Actual prefix output tested via integration.
	w := prefixWriter(nil, "test-service")
	n, err := w.Write([]byte("hello\nworld\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 12 {
		t.Errorf("want n=12, got n=%d", n)
	}
}

func TestDefaultServices_HaveRequiredFields(t *testing.T) {
	for _, svc := range defaultServices {
		if svc.name == "" {
			t.Error("service has empty name")
		}
		if svc.binary == "" {
			t.Errorf("service %s has empty binary", svc.name)
		}
		if svc.port == "" {
			t.Errorf("service %s has empty port", svc.name)
		}
	}
}

func TestDefaultServices_IncludeAllRuntimeServices(t *testing.T) {
	want := []string{
		"paladin-hub",
		"paladin-memory",
		"paladin-auth",
		"paladin-ws",
		"paladin-orchestrator",
		"paladin-agent",
		"paladin-comms",
		"paladin-ingest",
		"paladin-edge",
	}
	got := make([]string, 0, len(defaultServices))
	for _, svc := range defaultServices {
		got = append(got, svc.name)
	}
	if !slices.Equal(want, got) {
		t.Fatalf("default service order mismatch:\nwant %v\n got %v", want, got)
	}
}

func TestDefaultServices_UseCurrentLocalPorts(t *testing.T) {
	want := map[string]string{
		"paladin-hub":          "8082",
		"paladin-memory":       "9010 grpc / 9011 http",
		"paladin-auth":         "9003",
		"paladin-ws":           "9007",
		"paladin-orchestrator": "9008",
		"paladin-agent":        "9006",
		"paladin-comms":        "9009",
		"paladin-ingest":       "9001",
		"paladin-edge":         "9002",
	}
	for _, svc := range defaultServices {
		if svc.port != want[svc.name] {
			t.Errorf("%s port = %q, want %q", svc.name, svc.port, want[svc.name])
		}
	}
}

func TestWithDefaultEnv_AppendsMissingDefaults(t *testing.T) {
	got := withDefaultEnv([]string{"EXISTING=1"}, map[string]string{
		"PALADIN_AGENT_PORT": "9006",
	})
	if !slices.Contains(got, "EXISTING=1") {
		t.Fatalf("base env missing from result: %v", got)
	}
	if !slices.Contains(got, "PALADIN_AGENT_PORT=9006") {
		t.Fatalf("default env missing from result: %v", got)
	}
}

func TestWithDefaultEnv_PreservesCallerValue(t *testing.T) {
	got := withDefaultEnv([]string{"PALADIN_AGENT_PORT=19106"}, map[string]string{
		"PALADIN_AGENT_PORT": "9006",
	})
	if !slices.Contains(got, "PALADIN_AGENT_PORT=19106") {
		t.Fatalf("caller env missing from result: %v", got)
	}
	if slices.Contains(got, "PALADIN_AGENT_PORT=9006") {
		t.Fatalf("default overwrote caller env: %v", got)
	}
}
