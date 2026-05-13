package main

import "testing"

func TestAgentHTTPPort_DefaultsToComposePort(t *testing.T) {
	t.Setenv("PALADIN_AGENT_PORT", "")

	if got := agentHTTPPort(); got != "9006" {
		t.Fatalf("agentHTTPPort() = %q, want 9006", got)
	}
}

func TestAgentHTTPPort_UsesEnvOverride(t *testing.T) {
	t.Setenv("PALADIN_AGENT_PORT", "19106")

	if got := agentHTTPPort(); got != "19106" {
		t.Fatalf("agentHTTPPort() = %q, want 19106", got)
	}
}
