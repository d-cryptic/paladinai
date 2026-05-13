package publisher_test

// adapter_test.go exercises the natsAdapter (the internal bridge between
// *inats.Client and the JetStream interface) by spinning up an in-process
// NATS server with JetStream enabled, similar to the integration test in
// internal/nats. This brings natsAdapter.Publish from 0% to 100%.

import (
	"context"
	"net"
	"testing"
	"time"

	natsd "github.com/nats-io/nats-server/v2/server"
	"github.com/paladinai/paladinai/internal/alert"
	inats "github.com/paladinai/paladinai/internal/nats"
	"github.com/paladinai/paladinai/services/paladin-ingest/internal/publisher"
	"go.uber.org/zap"
)

func startInProcessNATS(t *testing.T) string {
	t.Helper()

	// Check that we can bind a port before starting the server.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("network unavailable, skipping in-process NATS test")
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	opts := &natsd.Options{
		Host:           "127.0.0.1",
		Port:           port,
		JetStream:      true,
		StoreDir:       t.TempDir(),
		NoLog:          true,
		NoSigs:         true,
		MaxControlLine: 4096,
	}
	srv, err := natsd.NewServer(opts)
	if err != nil {
		t.Fatalf("nats server create: %v", err)
	}
	go srv.Start()
	if !srv.ReadyForConnections(5 * time.Second) {
		t.Fatal("nats server not ready after 5s")
	}
	t.Cleanup(srv.Shutdown)

	return srv.ClientURL()
}

func TestNew_PublishAlert_ViaRealNATS(t *testing.T) {
	url := startInProcessNATS(t)

	log := zap.NewNop()
	client, err := inats.Connect(url, log)
	if err != nil {
		t.Fatalf("inats.Connect: %v", err)
	}
	t.Cleanup(client.Close)

	pub := publisher.New(client, log)

	labels := map[string]string{"alertname": "AdapterTest", "env": "test"}
	env := alert.AlertEnvelope{
		TenantID:    "adapter-tenant",
		Source:      alert.SourceAlertmanager,
		Fingerprint: alert.ComputeFingerprint(alert.SourceAlertmanager, labels),
		Labels:      labels,
		StartsAt:    time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pub.PublishAlert(ctx, env); err != nil {
		t.Fatalf("PublishAlert via real NATS: %v", err)
	}
}

func TestNew_PublishAlert_NATSError_Propagates(t *testing.T) {
	url := startInProcessNATS(t)

	log := zap.NewNop()
	client, err := inats.Connect(url, log)
	if err != nil {
		t.Fatalf("inats.Connect: %v", err)
	}

	// Close the underlying connection immediately to force subsequent publishes to fail.
	client.Conn().Close()

	pub := publisher.New(client, log)

	labels := map[string]string{"alertname": "ClosedTest"}
	env := alert.AlertEnvelope{
		TenantID:    "err-tenant",
		Source:      alert.SourceAlertmanager,
		Fingerprint: alert.ComputeFingerprint(alert.SourceAlertmanager, labels),
		Labels:      labels,
		StartsAt:    time.Now(),
	}

	err = pub.PublishAlert(context.Background(), env)
	if err == nil {
		t.Error("expected error when publishing on closed NATS client")
	}
}
