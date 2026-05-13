// graph_helpers_test.go provides exported test constructors for graph_test.go
// that allow injecting arbitrary agentType decisions without a real LLM classifier.
package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/compose"
	"github.com/paladinai/paladinai/internal/alert"
)

// stubClassifier is a Classifier that always returns a fixed agentType.
type stubClassifier struct {
	agentType string
	err       error
}

func (s *stubClassifier) Classify(_ context.Context, env *alert.AlertEnvelope) (*ClassificationResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	sev := "P2"
	if s.agentType == "rca" {
		sev = "P1"
	}
	return &ClassificationResult{
		Intent:     "metric_spike",
		AgentType:  s.agentType,
		Severity:   sev,
		Confidence: 0.95,
	}, nil
}

// BuildTestGraph constructs a CompiledGraph with a stubbed classifier that always
// routes to agentType. Used by graph_test.go to avoid real LLM calls.
func BuildTestGraph(ctx context.Context, agentType string, triager Triager, rca RCAAnalyzer) (*CompiledGraph, error) {
	return BuildGraph(ctx, GraphConfig{
		Classifier: &stubClassifier{agentType: agentType},
		Triager:    triager,
		RCA:        rca,
	})
}

// BuildTestGraphWithClassifyError constructs a CompiledGraph whose classify node
// always returns classifyErr. Used to test error propagation.
func BuildTestGraphWithClassifyError(ctx context.Context, classifyErr error, triager Triager, rca RCAAnalyzer) (*CompiledGraph, error) {
	if classifyErr == nil {
		return nil, errors.New("classifyErr must not be nil")
	}
	return BuildGraph(ctx, GraphConfig{
		Classifier: &stubClassifier{err: classifyErr},
		Triager:    triager,
		RCA:        rca,
	})
}

// Ensure ClassifierAgent satisfies Classifier interface at compile time.
var _ Classifier = (*ClassifierAgent)(nil)

// verifyGraphNodeKeys is a compile-time check that exported node key constants are strings.
var _ = fmt.Sprint(NodeClassify, NodeRouteBranch, NodeTriage, NodeRCA, NodeCollect, compose.END)
