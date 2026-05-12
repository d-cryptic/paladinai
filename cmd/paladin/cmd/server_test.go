package cmd

import (
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
