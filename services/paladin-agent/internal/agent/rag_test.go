package agent_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/internal/qdrant"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/agent"
)

// fakeRetriever is an in-memory RunbookRetriever for unit tests.
type fakeRetriever struct {
	chunks []qdrant.RunbookChunk
	err    error
}

func (f *fakeRetriever) Search(_ context.Context, _, _ string, _ int) ([]qdrant.RunbookChunk, error) {
	return f.chunks, f.err
}

func TestRAGContextBuilder_BuildReturnsChunks(t *testing.T) {
	t.Parallel()
	r := &fakeRetriever{chunks: []qdrant.RunbookChunk{
		{Content: "restart the service", Source: "runbook-1"},
	}}
	b := agent.NewRAGContextBuilder(r, zap.NewNop())
	env := &alert.AlertEnvelope{Title: "cpu high", TenantID: "t1"}

	ctx := b.Build(context.Background(), env)
	assert.Contains(t, ctx, "restart the service")
	assert.Contains(t, ctx, "runbook-1")
	assert.Contains(t, ctx, "Relevant Runbooks")
}

func TestRAGContextBuilder_BuildReturnsEmptyOnError(t *testing.T) {
	t.Parallel()
	r := &fakeRetriever{err: errors.New("qdrant down")}
	b := agent.NewRAGContextBuilder(r, zap.NewNop())
	env := &alert.AlertEnvelope{Title: "cpu high", TenantID: "t1"}

	ctx := b.Build(context.Background(), env)
	assert.Empty(t, ctx)
}

func TestRAGContextBuilder_BuildReturnsEmptyOnNoResults(t *testing.T) {
	t.Parallel()
	// nil log should be replaced with zap.NewNop() internally.
	r := &fakeRetriever{chunks: nil}
	b := agent.NewRAGContextBuilder(r, nil)
	env := &alert.AlertEnvelope{Title: "cpu high", TenantID: "t1"}

	ctx := b.Build(context.Background(), env)
	assert.Empty(t, ctx)
}
