package main

import "testing"

func TestCommsHTTPPort_DefaultsToComposePort(t *testing.T) {
	t.Setenv("PALADIN_COMMS_PORT", "")

	if got := commsHTTPPort(); got != "9009" {
		t.Fatalf("commsHTTPPort() = %q, want 9009", got)
	}
}

func TestCommsHTTPPort_UsesEnvOverride(t *testing.T) {
	t.Setenv("PALADIN_COMMS_PORT", "19109")

	if got := commsHTTPPort(); got != "19109" {
		t.Fatalf("commsHTTPPort() = %q, want 19109", got)
	}
}
