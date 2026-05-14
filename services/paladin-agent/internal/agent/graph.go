// Package agent: graph.go builds the Stage 3 Eino compose.Graph for the
// supervisor pipeline. The graph enforces an explicit, auditable execution
// path: classify → route (conditional branch) → specialist agent → end.
//
// Node names are exported constants so tests and trace dashboards can
// reference them without string literals.
package agent

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"
	"github.com/paladinai/paladinai/internal/alert"
	"go.uber.org/zap"
)

// Graph node keys — used in AddEdge / AddBranch calls and in trace events.
const (
	NodeClassify    = "classify"
	NodeRouteBranch = "route"
	NodeTriage      = "triage"
	NodeRCA         = "rca"
	NodeRunbook     = "runbook"
	NodeIntegration = "integration"
	NodeMemory      = "memory"
	NodeCollect     = "collect"
	NodeEnd         = compose.END
)

// CompiledGraph is the compiled, runnable Eino graph for the supervisor pipeline.
// Thread-safe after compilation; call Invoke from multiple goroutines.
type CompiledGraph struct {
	runnable compose.Runnable[*IncidentState, *IncidentState]
	log      *zap.Logger
}

// Run executes the full supervisor pipeline for one alert and returns the
// populated IncidentState. It is safe to call concurrently.
func (cg *CompiledGraph) Run(ctx context.Context, state *IncidentState) (*IncidentState, error) {
	return cg.runnable.Invoke(ctx, state)
}

// Classifier is the interface satisfied by ClassifierAgent.
// Extracted here so tests can inject fakes without starting an LLM.
type Classifier interface {
	Classify(ctx context.Context, env *alert.AlertEnvelope) (*ClassificationResult, error)
}

// GraphConfig holds the dependencies wired into each graph node.
type GraphConfig struct {
	Classifier Classifier
	Triager    Triager
	RCA        RCAAnalyzer // may be nil; P1 alerts fall back to triage when nil
	Log        *zap.Logger
}

// BuildGraph constructs and compiles the Eino compose.Graph for the supervisor pipeline.
//
// Graph topology:
//
//	START → [classify] → [route branch] ─── "triage" → [triage] ─┐
//	                                     └── "rca"    → [rca]     ┘
//	                                     └── specialists fallback to [triage]
//	                                                               ↓
//	                                                           [collect] → END
//
// All nodes are compose.Lambda wrapping existing agent methods.
// The branch condition reads AgentType from the state returned by [classify].
func BuildGraph(ctx context.Context, cfg GraphConfig) (*CompiledGraph, error) {
	if cfg.Log == nil {
		cfg.Log = zap.NewNop()
	}

	g := compose.NewGraph[*IncidentState, *IncidentState]()

	// ── [classify] node ──────────────────────────────────────────────────────
	classifyLambda := compose.InvokableLambda[*IncidentState, *IncidentState](
		func(ctx context.Context, state *IncidentState) (*IncidentState, error) {
			cls, err := cfg.Classifier.Classify(ctx, &state.Alert)
			if err != nil {
				return state, fmt.Errorf("graph[classify]: %w", err)
			}
			state.Intent = cls.Intent
			state.AgentType = cls.AgentType
			state.Severity = cls.Severity
			state.RoutingConfidence = cls.Confidence
			cfg.Log.Info("graph: classify complete",
				zap.String("tenant", state.TenantID),
				zap.String("intent", state.Intent),
				zap.String("agent_type", state.AgentType),
				zap.String("severity", state.Severity),
			)
			return state, nil
		},
	)
	if err := g.AddLambdaNode(NodeClassify, classifyLambda); err != nil {
		return nil, fmt.Errorf("graph: add classify node: %w", err)
	}

	// ── [triage] node ────────────────────────────────────────────────────────
	triageLambda := compose.InvokableLambda[*IncidentState, *IncidentState](
		func(ctx context.Context, state *IncidentState) (*IncidentState, error) {
			result, err := cfg.Triager.Triage(ctx, &state.Alert)
			if err != nil {
				return state, fmt.Errorf("graph[triage]: %w", err)
			}
			state.TriageResult = result
			state.NeedsHuman = result.NeedsHuman
			cfg.Log.Info("graph: triage complete",
				zap.String("tenant", state.TenantID),
				zap.String("severity", result.ConfirmedSeverity),
				zap.Bool("needs_human", result.NeedsHuman),
			)
			return state, nil
		},
	)
	if err := g.AddLambdaNode(NodeTriage, triageLambda); err != nil {
		return nil, fmt.Errorf("graph: add triage node: %w", err)
	}

	// ── [rca] node ───────────────────────────────────────────────────────────
	rcaLambda := compose.InvokableLambda[*IncidentState, *IncidentState](
		func(ctx context.Context, state *IncidentState) (*IncidentState, error) {
			if cfg.RCA == nil {
				// Graceful degradation: no RCA agent wired → fall back to triage.
				cfg.Log.Warn("graph[rca]: RCAAnalyzer not configured, falling back to triage",
					zap.String("tenant", state.TenantID),
				)
				result, err := cfg.Triager.Triage(ctx, &state.Alert)
				if err != nil {
					return state, fmt.Errorf("graph[rca→triage fallback]: %w", err)
				}
				state.TriageResult = result
				state.NeedsHuman = result.NeedsHuman
				return state, nil
			}

			// RCA requires the prior triage result as context. Run triage first if missing.
			triageResult := state.TriageResult
			if triageResult == nil {
				tr, terr := cfg.Triager.Triage(ctx, &state.Alert)
				if terr != nil {
					return state, fmt.Errorf("graph[rca→pre-triage]: %w", terr)
				}
				state.TriageResult = tr
				triageResult = tr
			}

			result, err := cfg.RCA.Analyze(ctx, &state.Alert, triageResult)
			if err != nil {
				return state, fmt.Errorf("graph[rca]: %w", err)
			}
			state.RCAResult = result
			state.NeedsHuman = true // P1 always requires human review
			cfg.Log.Info("graph: rca complete",
				zap.String("tenant", state.TenantID),
				zap.String("root_cause", result.RootCauseHypothesis),
				zap.String("confidence", result.Confidence),
			)
			return state, nil
		},
	)
	if err := g.AddLambdaNode(NodeRCA, rcaLambda); err != nil {
		return nil, fmt.Errorf("graph: add rca node: %w", err)
	}

	// ── [collect] node ───────────────────────────────────────────────────────
	// Passthrough: merges outputs from triage/rca paths into a single node
	// before END. Allows future post-processing (comms, PG write) to be added here.
	collectLambda := compose.InvokableLambda[*IncidentState, *IncidentState](
		func(_ context.Context, state *IncidentState) (*IncidentState, error) {
			return state, nil
		},
	)
	if err := g.AddLambdaNode(NodeCollect, collectLambda); err != nil {
		return nil, fmt.Errorf("graph: add collect node: %w", err)
	}

	// ── Edges ────────────────────────────────────────────────────────────────
	if err := g.AddEdge(compose.START, NodeClassify); err != nil {
		return nil, fmt.Errorf("graph: edge START→classify: %w", err)
	}

	// Conditional branch after classify: AgentType determines the specialist.
	branch := compose.NewGraphBranch(
		func(_ context.Context, state *IncidentState) (string, error) {
			switch state.AgentType {
			case "rca":
				return NodeRCA, nil
			case "runbook", "integration", "memory":
				return NodeTriage, nil
			default: // "triage" and anything unrecognised
				return NodeTriage, nil
			}
		},
		map[string]bool{NodeTriage: true, NodeRCA: true},
	)
	if err := g.AddBranch(NodeClassify, branch); err != nil {
		return nil, fmt.Errorf("graph: branch classify→[triage|rca]: %w", err)
	}

	if err := g.AddEdge(NodeTriage, NodeCollect); err != nil {
		return nil, fmt.Errorf("graph: edge triage→collect: %w", err)
	}
	if err := g.AddEdge(NodeRCA, NodeCollect); err != nil {
		return nil, fmt.Errorf("graph: edge rca→collect: %w", err)
	}
	if err := g.AddEdge(NodeCollect, compose.END); err != nil {
		return nil, fmt.Errorf("graph: edge collect→END: %w", err)
	}

	// ── Compile ──────────────────────────────────────────────────────────────
	runnable, err := g.Compile(ctx, compose.WithGraphName("paladin-supervisor"))
	if err != nil {
		return nil, fmt.Errorf("graph: compile: %w", err)
	}

	return &CompiledGraph{runnable: runnable, log: cfg.Log}, nil
}
