package agent

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/internal/qdrant"
)

// RunbookRetriever retrieves relevant runbook chunks for an alert.
type RunbookRetriever interface {
	Search(ctx context.Context, query, tenantID string, topK int) ([]qdrant.RunbookChunk, error)
}

// RAGContextBuilder builds a runbook context string for injection into prompts.
type RAGContextBuilder struct {
	retriever RunbookRetriever
	topK      int
	log       *zap.Logger
}

// NewRAGContextBuilder creates a context builder with the given retriever.
func NewRAGContextBuilder(r RunbookRetriever, log *zap.Logger) *RAGContextBuilder {
	if log == nil {
		log = zap.NewNop()
	}
	return &RAGContextBuilder{retriever: r, topK: 3, log: log}
}

// Build retrieves relevant runbook chunks and formats them as context.
// Returns an empty string if retrieval fails or returns no results.
func (b *RAGContextBuilder) Build(ctx context.Context, env *alert.AlertEnvelope) string {
	if env == nil {
		return ""
	}
	query := fmt.Sprintf("%s %s", env.Title, env.Description)
	chunks, err := b.retriever.Search(ctx, query, env.TenantID, b.topK)
	if err != nil {
		b.log.Warn("rag: runbook retrieval failed", zap.Error(err))
		return ""
	}
	if len(chunks) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n\n## Relevant Runbooks\n")
	for i, ch := range chunks {
		fmt.Fprintf(&sb, "\n### %d. %s\n%s\n", i+1, ch.Source, ch.Content)
	}
	return sb.String()
}
