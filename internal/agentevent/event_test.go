package agentevent_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/paladinai/paladinai/internal/agentevent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Subject ─────────────────────────────────────────────────────────────────

func TestSubject_Format(t *testing.T) {
	assert.Equal(t, "stream.tenant-1.sess-abc", agentevent.Subject("tenant-1", "sess-abc"))
}

func TestSubject_EmptyParts(t *testing.T) {
	assert.Equal(t, "stream..", agentevent.Subject("", ""))
}

// ─── Event helpers ────────────────────────────────────────────────────────────

func TestStepStart_CorrectType(t *testing.T) {
	ev := agentevent.StepStart("classify", "Classifying alert severity")
	assert.Equal(t, agentevent.EventTypeStepStart, ev.Type)
	assert.Equal(t, "classify", ev.Step)
	assert.Equal(t, "Classifying alert severity", ev.Content)
	assert.False(t, ev.Timestamp.IsZero())
}

func TestStepEnd_CorrectType(t *testing.T) {
	ev := agentevent.StepEnd("triage", "Triage complete")
	assert.Equal(t, agentevent.EventTypeStepEnd, ev.Type)
	assert.Equal(t, "triage", ev.Step)
}

func TestToolCallEvent_CorrectFields(t *testing.T) {
	ev := agentevent.ToolCallEvent("triage", "mcp-prometheus", "Querying Prometheus (last 2h)")
	assert.Equal(t, agentevent.EventTypeToolCall, ev.Type)
	assert.Equal(t, "mcp-prometheus", ev.ToolName)
	assert.Contains(t, ev.Content, "Prometheus")
}

func TestToolResultEvent_CorrectFields(t *testing.T) {
	ev := agentevent.ToolResultEvent("triage", "mcp-prometheus", "returned 340ms latency spike")
	assert.Equal(t, agentevent.EventTypeToolResult, ev.Type)
	assert.Equal(t, "mcp-prometheus", ev.ToolName)
}

func TestErrorEvent_CorrectType(t *testing.T) {
	ev := agentevent.ErrorEvent("rca", "context deadline exceeded")
	assert.Equal(t, agentevent.EventTypeError, ev.Type)
	assert.Equal(t, "rca", ev.Step)
	assert.Equal(t, "context deadline exceeded", ev.Content)
}

func TestApprovalRequiredEvent_IncludesMetadata(t *testing.T) {
	meta := map[string]string{"gate_id": "gate-123", "runbook_id": "rb-42"}
	ev := agentevent.ApprovalRequiredEvent("runbook", "Delete pod requires approval", meta)
	assert.Equal(t, agentevent.EventTypeApprovalRequired, ev.Type)
	assert.Equal(t, "gate-123", ev.Metadata["gate_id"])
}

// ─── AgentEvent JSON marshaling ───────────────────────────────────────────────

func TestAgentEvent_JSONRoundTrip(t *testing.T) {
	ev := agentevent.AgentEvent{
		Type:      agentevent.EventTypeToolCall,
		Step:      "triage",
		ToolName:  "mcp-k8s",
		Content:   "Listing pods",
		Metadata:  map[string]string{"namespace": "prod"},
		Timestamp: time.Now().Truncate(time.Second),
	}
	b, err := json.Marshal(ev)
	require.NoError(t, err)

	var decoded agentevent.AgentEvent
	require.NoError(t, json.Unmarshal(b, &decoded))
	assert.Equal(t, ev.Type, decoded.Type)
	assert.Equal(t, ev.ToolName, decoded.ToolName)
	assert.Equal(t, ev.Metadata["namespace"], decoded.Metadata["namespace"])
}

func TestAgentEvent_EmptyFieldsOmitted(t *testing.T) {
	ev := agentevent.StepStart("classify", "")
	b, err := json.Marshal(ev)
	require.NoError(t, err)
	assert.NotContains(t, string(b), `"tool_name"`)
	assert.NotContains(t, string(b), `"metadata"`)
	assert.NotContains(t, string(b), `"content":""`)
}

// ─── NoopPublisher ────────────────────────────────────────────────────────────

func TestNoopPublisher_NeverErrors(t *testing.T) {
	pub := agentevent.NoopPublisher{}
	ev := agentevent.StepStart("test", "content")
	assert.NoError(t, pub.Publish(context.Background(), "t1", "s1", ev))
}

// ─── RecordingPublisher ───────────────────────────────────────────────────────

func TestRecordingPublisher_CapturesEvents(t *testing.T) {
	pub := &agentevent.RecordingPublisher{}
	ctx := context.Background()

	require.NoError(t, pub.Publish(ctx, "t1", "s1", agentevent.StepStart("classify", "starting")))
	require.NoError(t, pub.Publish(ctx, "t1", "s1", agentevent.StepEnd("classify", "done")))
	require.Len(t, pub.Events, 2)
	assert.Equal(t, agentevent.EventTypeStepStart, pub.Events[0].Type)
	assert.Equal(t, agentevent.EventTypeStepEnd, pub.Events[1].Type)
}

// ─── NATSPublisher ────────────────────────────────────────────────────────────

// fakeNATSConn records published subjects and payloads.
type fakeNATSConn struct {
	published []struct {
		subject string
		data    []byte
	}
	err error
}

func (f *fakeNATSConn) Publish(subject string, data []byte) error {
	f.published = append(f.published, struct {
		subject string
		data    []byte
	}{subject: subject, data: data})
	return f.err
}

func TestNATSPublisher_PublishesToCorrectSubject(t *testing.T) {
	conn := &fakeNATSConn{}
	pub := agentevent.NewNATSPublisher(conn)

	ev := agentevent.StepStart("classify", "starting")
	require.NoError(t, pub.Publish(context.Background(), "tenant-1", "sess-123", ev))

	require.Len(t, conn.published, 1)
	assert.Equal(t, "stream.tenant-1.sess-123", conn.published[0].subject)
}

func TestNATSPublisher_SerializesValidJSON(t *testing.T) {
	conn := &fakeNATSConn{}
	pub := agentevent.NewNATSPublisher(conn)

	ev := agentevent.ToolCallEvent("triage", "mcp-prometheus", "querying")
	require.NoError(t, pub.Publish(context.Background(), "t1", "s1", ev))

	require.Len(t, conn.published, 1)
	var decoded agentevent.AgentEvent
	require.NoError(t, json.Unmarshal(conn.published[0].data, &decoded))
	assert.Equal(t, agentevent.EventTypeToolCall, decoded.Type)
	assert.Equal(t, "mcp-prometheus", decoded.ToolName)
}

func TestNATSPublisher_SetsTimestampIfZero(t *testing.T) {
	conn := &fakeNATSConn{}
	pub := agentevent.NewNATSPublisher(conn)

	ev := agentevent.AgentEvent{Type: agentevent.EventTypeStepStart, Step: "test"}
	require.True(t, ev.Timestamp.IsZero())

	require.NoError(t, pub.Publish(context.Background(), "t1", "s1", ev))

	var decoded agentevent.AgentEvent
	require.NoError(t, json.Unmarshal(conn.published[0].data, &decoded))
	assert.False(t, decoded.Timestamp.IsZero())
}

func TestNATSPublisher_NATSError_ReturnsWrappedError(t *testing.T) {
	conn := &fakeNATSConn{err: assert.AnError}
	pub := agentevent.NewNATSPublisher(conn)

	ev := agentevent.StepStart("test", "")
	err := pub.Publish(context.Background(), "t1", "s1", ev)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stream.t1.s1")
}
