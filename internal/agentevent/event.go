// Package agentevent defines the structured event type emitted by agent graph
// nodes and provides helpers for publishing events to NATS JetStream.
//
// Events are published to subject "stream.{tenantID}.{sessionID}" and consumed
// by paladin-ws to push real-time progress to the browser WebSocket.
//
// See docs/plans/03.agent-runtime-stage3.md §Decision 11.
package agentevent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// EventType classifies the kind of event emitted by an agent node.
type EventType string

const (
	EventTypeStepStart       EventType = "step_start"
	EventTypeToken           EventType = "token"
	EventTypeToolCall        EventType = "tool_call"
	EventTypeToolResult      EventType = "tool_result"
	EventTypeStepEnd         EventType = "step_end"
	EventTypeError           EventType = "error"
	EventTypeApprovalRequired EventType = "approval_required"
)

// AgentEvent is the structured payload published to NATS for every agent step.
// It is serialized as JSON on the wire.
type AgentEvent struct {
	Type      EventType         `json:"type"`
	Step      string            `json:"step"`               // graph node name
	Content   string            `json:"content,omitempty"`  // token text or description
	ToolName  string            `json:"tool_name,omitempty"` // set for tool_call / tool_result
	Metadata  map[string]string `json:"metadata,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// Subject returns the NATS subject for a given tenant and session.
// Convention: "stream.{tenantID}.{sessionID}"
func Subject(tenantID, sessionID string) string {
	return fmt.Sprintf("stream.%s.%s", tenantID, sessionID)
}

// Publisher is the interface for publishing agent events.
// Implementations: NATSPublisher (production), NoopPublisher (tests), RecordingPublisher (tests).
type Publisher interface {
	Publish(ctx context.Context, tenantID, sessionID string, event AgentEvent) error
}

// ─── NoopPublisher ────────────────────────────────────────────────────────────

// NoopPublisher discards all events. Use in tests or when event streaming is not needed.
type NoopPublisher struct{}

func (NoopPublisher) Publish(_ context.Context, _, _ string, _ AgentEvent) error { return nil }

// ─── RecordingPublisher ───────────────────────────────────────────────────────

// RecordingPublisher captures published events for test assertions.
type RecordingPublisher struct {
	Events []AgentEvent
}

func (r *RecordingPublisher) Publish(_ context.Context, _, _ string, ev AgentEvent) error {
	r.Events = append(r.Events, ev)
	return nil
}

// ─── NATSPublisher ────────────────────────────────────────────────────────────

// NATSConn is the minimal NATS interface required for event publishing.
// Satisfied by *nats.Conn.
type NATSConn interface {
	Publish(subject string, data []byte) error
}

// NATSPublisher publishes AgentEvents to NATS JetStream subjects.
type NATSPublisher struct {
	conn NATSConn
}

// NewNATSPublisher creates an event publisher backed by a NATS connection.
func NewNATSPublisher(conn NATSConn) *NATSPublisher {
	return &NATSPublisher{conn: conn}
}

// Publish serializes the event as JSON and publishes it to the NATS subject
// "stream.{tenantID}.{sessionID}".
func (p *NATSPublisher) Publish(_ context.Context, tenantID, sessionID string, ev AgentEvent) error {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now()
	}
	data, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("agentevent: marshal event: %w", err)
	}
	subject := Subject(tenantID, sessionID)
	if err := p.conn.Publish(subject, data); err != nil {
		return fmt.Errorf("agentevent: publish to %s: %w", subject, err)
	}
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// StepStart returns a step_start event for the given graph node.
func StepStart(step, content string) AgentEvent {
	return AgentEvent{Type: EventTypeStepStart, Step: step, Content: content, Timestamp: time.Now()}
}

// StepEnd returns a step_end event for the given graph node.
func StepEnd(step, content string) AgentEvent {
	return AgentEvent{Type: EventTypeStepEnd, Step: step, Content: content, Timestamp: time.Now()}
}

// ToolCallEvent returns a tool_call event.
func ToolCallEvent(step, toolName, description string) AgentEvent {
	return AgentEvent{
		Type:      EventTypeToolCall,
		Step:      step,
		ToolName:  toolName,
		Content:   description,
		Timestamp: time.Now(),
	}
}

// ToolResultEvent returns a tool_result event.
func ToolResultEvent(step, toolName, summary string) AgentEvent {
	return AgentEvent{
		Type:      EventTypeToolResult,
		Step:      step,
		ToolName:  toolName,
		Content:   summary,
		Timestamp: time.Now(),
	}
}

// ErrorEvent returns an error event.
func ErrorEvent(step, errMsg string) AgentEvent {
	return AgentEvent{Type: EventTypeError, Step: step, Content: errMsg, Timestamp: time.Now()}
}

// ApprovalRequiredEvent returns an approval_required event.
func ApprovalRequiredEvent(step, description string, meta map[string]string) AgentEvent {
	return AgentEvent{
		Type:      EventTypeApprovalRequired,
		Step:      step,
		Content:   description,
		Metadata:  meta,
		Timestamp: time.Now(),
	}
}
