package integrationpkg

import (
	"errors"
	"path/filepath"
	"testing"
)

// integrationsDir resolves to <repo>/integrations relative to this test file.
func integrationsDir(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "integrations"))
	if err != nil {
		t.Fatalf("resolve integrations dir: %v", err)
	}
	return abs
}

func TestLoadAll_ReturnsAllBundledIntegrations(t *testing.T) {
	t.Parallel()

	all, err := LoadAll(integrationsDir(t))
	if err != nil {
		t.Fatalf("LoadAll error: %v", err)
	}

	want := map[string]bool{
		"alertmanager": false,
		"datadog":      false,
		"pagerduty":    false,
		"cloudwatch":   false,
		"kubernetes":   false,
	}
	for _, integ := range all {
		if _, ok := want[integ.Name]; ok {
			want[integ.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("expected integration %q to be loaded", name)
		}
	}

	// Sorted by Name.
	for i := 1; i < len(all); i++ {
		if all[i-1].Name > all[i].Name {
			t.Errorf("LoadAll result not sorted: %s before %s", all[i-1].Name, all[i].Name)
		}
	}
}

func TestLoadByName_Datadog(t *testing.T) {
	t.Parallel()

	got, err := LoadByName(integrationsDir(t), "datadog")
	if err != nil {
		t.Fatalf("LoadByName error: %v", err)
	}
	if got.Name != "datadog" {
		t.Errorf("Name = %q, want datadog", got.Name)
	}
	if got.Version != "1.0.0" {
		t.Errorf("Version = %q, want 1.0.0", got.Version)
	}
	if got.Receiver.Type != "webhook" {
		t.Errorf("Receiver.Type = %q, want webhook", got.Receiver.Type)
	}
	if got.Receiver.Path != "/v1/ingest/datadog" {
		t.Errorf("Receiver.Path = %q, want /v1/ingest/datadog", got.Receiver.Path)
	}
	if got.Auth.Type != "api_key" {
		t.Errorf("Auth.Type = %q, want api_key", got.Auth.Type)
	}
	if len(got.Auth.Fields) != 2 {
		t.Errorf("Auth.Fields len = %d, want 2", len(got.Auth.Fields))
	}
	if len(got.Tools) != 3 {
		t.Errorf("Tools len = %d, want 3", len(got.Tools))
	}
	if _, ok := got.ConfigSchema["site"]; !ok {
		t.Errorf("ConfigSchema missing 'site' key")
	}
}

func TestLoadByName_Pagerduty_HMACHeader(t *testing.T) {
	t.Parallel()

	got, err := LoadByName(integrationsDir(t), "pagerduty")
	if err != nil {
		t.Fatalf("LoadByName error: %v", err)
	}
	if got.Receiver.HMACHeader != "X-PagerDuty-Signature" {
		t.Errorf("HMACHeader = %q, want X-PagerDuty-Signature", got.Receiver.HMACHeader)
	}
}

func TestLoadByName_Kubernetes_NoReceiver(t *testing.T) {
	t.Parallel()

	got, err := LoadByName(integrationsDir(t), "kubernetes")
	if err != nil {
		t.Fatalf("LoadByName error: %v", err)
	}
	if got.Receiver.Type != "none" {
		t.Errorf("Receiver.Type = %q, want none", got.Receiver.Type)
	}
	if got.Auth.Type != "kubeconfig" {
		t.Errorf("Auth.Type = %q, want kubeconfig", got.Auth.Type)
	}
	if len(got.Tools) != 5 {
		t.Errorf("Tools len = %d, want 5", len(got.Tools))
	}
}

func TestLoadByName_Unknown_ReturnsErrNotFound(t *testing.T) {
	t.Parallel()

	_, err := LoadByName(integrationsDir(t), "does-not-exist")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestLoadByName_EmptyName(t *testing.T) {
	t.Parallel()

	_, err := LoadByName(integrationsDir(t), "")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestLoadAll_MissingDir(t *testing.T) {
	t.Parallel()

	_, err := LoadAll(filepath.Join("testdata", "nope"))
	if err == nil {
		t.Fatal("expected error for missing dir")
	}
}
