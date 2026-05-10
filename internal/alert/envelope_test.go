package alert_test

import (
	"testing"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeFingerprint_Deterministic(t *testing.T) {
	labels := map[string]string{
		"alertname": "HighCPU",
		"namespace": "production",
		"instance":  "node-1",
	}

	fp1 := alert.ComputeFingerprint(alert.SourceAlertmanager, labels)
	fp2 := alert.ComputeFingerprint(alert.SourceAlertmanager, labels)

	require.Equal(t, fp1, fp2, "fingerprint must be deterministic")
	assert.Len(t, fp1, 32, "fingerprint should be 32 hex chars")
}

func TestComputeFingerprint_LabelOrderIndependent(t *testing.T) {
	labels1 := map[string]string{
		"alertname": "HighCPU",
		"namespace": "production",
		"instance":  "node-1",
	}
	labels2 := map[string]string{
		"instance":  "node-1",
		"alertname": "HighCPU",
		"namespace": "production",
	}

	fp1 := alert.ComputeFingerprint(alert.SourceAlertmanager, labels1)
	fp2 := alert.ComputeFingerprint(alert.SourceAlertmanager, labels2)

	assert.Equal(t, fp1, fp2, "fingerprint must not depend on label map iteration order")
}

func TestComputeFingerprint_DifferentSources(t *testing.T) {
	labels := map[string]string{"alertname": "HighCPU"}

	fpAM := alert.ComputeFingerprint(alert.SourceAlertmanager, labels)
	fpDD := alert.ComputeFingerprint(alert.SourceDatadog, labels)

	assert.NotEqual(t, fpAM, fpDD, "same labels from different sources should produce different fingerprints")
}


func TestAlertEnvelope_Clone(t *testing.T) {
	original := alert.AlertEnvelope{
		TenantID: "t1",
		Labels:   map[string]string{"env": "prod"},
		Payload:  []byte(`{"key":"value"}`),
	}

	clone := original.Clone()
	clone.Labels["env"] = "staging"
	clone.TenantID = "t2"
	clone.Payload[0] = '!'

	assert.Equal(t, "prod", original.Labels["env"], "clone mutation must not affect original labels")
	assert.Equal(t, "t1", original.TenantID, "clone mutation must not affect original tenant")
	assert.Equal(t, byte('{'), original.Payload[0], "clone mutation must not affect original payload bytes")
}

func TestValidateTenantID(t *testing.T) {
	valid := []string{"tenant-123", "ACME_Corp", "abc", "t1"}
	for _, id := range valid {
		assert.NoError(t, alert.ValidateTenantID(id), "expected %q to be valid", id)
	}

	invalid := []string{"", "foo.bar", "foo>bar", "foo*bar", "has space", "slash/here"}
	for _, id := range invalid {
		assert.Error(t, alert.ValidateTenantID(id), "expected %q to be invalid", id)
	}
}

func TestAlertEnvelope_NATSSubject_ValidTenant(t *testing.T) {
	env := alert.AlertEnvelope{
		TenantID: "tenant-abc",
		Source:   alert.SourceAlertmanager,
	}
	assert.Equal(t, "paladin.alerts.raw.tenant-abc.alertmanager", env.NATSSubject())
}

func TestAlertEnvelope_NATSSubject_InvalidTenant_Panics(t *testing.T) {
	env := alert.AlertEnvelope{
		TenantID: "bad.tenant",
		Source:   alert.SourceAlertmanager,
	}
	assert.Panics(t, func() { env.NATSSubject() }, "invalid tenant_id should panic")
}
