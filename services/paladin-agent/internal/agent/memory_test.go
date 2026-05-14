package agent_test

import (
	"context"
	"testing"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaticMemorySpecialist_KafkaRecurring(t *testing.T) {
	env := &alert.AlertEnvelope{
		Severity:    alert.SeverityP2,
		Title:       "Kafka consumer lag",
		Description: "Consumer group rebalance loop recurring",
		Labels:      map[string]string{"service": "event-consumer", "job": "kafka"},
		Annotations: map[string]string{"description": "similar prior incidents"},
	}

	result, err := agent.StaticMemorySpecialist{}.RecallMemory(context.Background(), env)
	require.NoError(t, err)

	assert.Contains(t, result.Signals, "kafka_rebalance_or_lag")
	assert.Contains(t, result.Signals, "recurring_incident")
	assert.Contains(t, result.MemoryTypes, "episodic")
	assert.Contains(t, result.MemoryTypes, "procedural")
	assert.True(t, result.NeedsHuman)
}

func TestStaticMemorySpecialist_CacheSessionPattern(t *testing.T) {
	env := &alert.AlertEnvelope{
		Severity:    alert.SeverityP3,
		Title:       "Stale session cache pattern",
		Description: "stale user sessions in cache",
		Labels:      map[string]string{"service": "session-cache"},
		Annotations: map[string]string{},
	}

	result, err := agent.StaticMemorySpecialist{}.RecallMemory(context.Background(), env)
	require.NoError(t, err)

	assert.Contains(t, result.Signals, "cache_or_session_pattern")
	assert.Contains(t, result.MemoryTypes, "working")
	assert.False(t, result.NeedsHuman)
}
