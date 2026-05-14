package main

import (
	"context"
	"testing"
)

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

func TestAcquireSlot_RespectsContextCancel(t *testing.T) {
	sem := make(chan struct{}, 1)
	sem <- struct{}{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if acquireSlot(ctx, sem) {
		t.Fatal("expected acquireSlot to return false when context is canceled")
	}
}

func TestAcquireSlot_AcquiresWhenCapacityAvailable(t *testing.T) {
	sem := make(chan struct{}, 1)

	if !acquireSlot(context.Background(), sem) {
		t.Fatal("expected acquireSlot to acquire available capacity")
	}
}
