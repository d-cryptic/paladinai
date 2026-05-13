// Command paladin-agent is the PaladinAI agent runtime.
// It consumes correlated alerts from NATS and dispatches them through an
// Eino-based agent graph (Triage → RCA → Runbook → Comms).
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/paladinai/paladinai/internal/anthropic"
	"github.com/paladinai/paladinai/internal/cache"
	"github.com/paladinai/paladinai/internal/logger"
	internalnats "github.com/paladinai/paladinai/internal/nats"
	"github.com/paladinai/paladinai/internal/promptstore"
	"github.com/paladinai/paladinai/internal/qdrant"
	agentcfg "github.com/paladinai/paladinai/services/paladin-agent/config"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/agent"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/incident"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/llm"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/worker"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// pgxQuerier adapts *pgxpool.Pool to promptstore.Querier.
// pgx.Rows satisfies promptstore.Rows structurally (Next/Scan/Err/Close).
type pgxQuerier struct{ pool *pgxpool.Pool }

func (q pgxQuerier) Query(ctx context.Context, sql string, args ...any) (promptstore.Rows, error) {
	return q.pool.Query(ctx, sql, args...)
}

// natsPublisherAdapter bridges the internal/nats.Client to worker.ResultPublisher.
type natsPublisherAdapter struct{ client *internalnats.Client }

func (a natsPublisherAdapter) Publish(ctx context.Context, subject string, data []byte) (worker.PublishResult, error) {
	ack, err := a.client.Publish(ctx, subject, data)
	if err != nil {
		return worker.PublishResult{}, err
	}
	if ack == nil {
		return worker.PublishResult{}, nil
	}
	return worker.PublishResult{Sequence: ack.Sequence}, nil
}

// natsReplayAdapter bridges internal/nats.Client to incident.ReplayPublisher,
// discarding the PubAck since replay callers only need error semantics.
type natsReplayAdapter struct{ client *internalnats.Client }

func (a natsReplayAdapter) Publish(ctx context.Context, subject string, data []byte) error {
	_, err := a.client.Publish(ctx, subject, data)
	return err
}

func main() {
	cfg, err := agentcfg.Load()
	if err != nil {
		log, _ := zap.NewProduction()
		log.Fatal("config load failed", zap.Error(err))
	}

	log, err := logger.New("paladin-agent")
	if err != nil {
		boot, _ := zap.NewProduction()
		boot.Fatal("logger init failed", zap.Error(err))
	}
	defer log.Sync() //nolint:errcheck

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── Prompt store (versioned system prompts, 60s hot-reload) ──────────────
	var psQuerier promptstore.Querier
	if cfg.DatabaseURL != "" {
		pool, poolErr := pgxpool.New(ctx, cfg.DatabaseURL)
		if poolErr != nil {
			log.Warn("prompt store: postgres connect failed, using built-in defaults", zap.Error(poolErr))
		} else {
			psQuerier = pgxQuerier{pool: pool}
			defer pool.Close()
		}
	}
	ps, err := promptstore.New(ctx, psQuerier, log)
	if err != nil {
		log.Warn("prompt store init failed, using built-in defaults", zap.Error(err))
	}
	if ps != nil {
		ps.RefreshInterval = 60 * time.Second
		ps.StartRefresh(ctx)
	}

	// ── LLM client ───────────────────────────────────────────────────────────
	llmClient, err := llm.New(ctx, llm.Config{
		BaseURL:              cfg.LLM.GatewayURL,
		APIKey:               llm.Secret(cfg.LLM.OpenRouterKey),
		AllowInsecureBaseURL: cfg.LLM.AllowInsecureGateway,
		ModelTierA:           cfg.LLM.TierA,
		ModelTierB:           cfg.LLM.TierB,
		ModelTierC:           cfg.LLM.TierC,
	})
	if err != nil {
		log.Fatal("llm client init failed", zap.Error(err))
	}

	// ── Tier A model (classifier — fast, cheap) ──────────────────────────────
	tierAModel, err := llmClient.Model(llm.TierA)
	if err != nil {
		log.Fatal("llm tier A not found", zap.Error(err))
	}

	tierBModel, err := llmClient.Model(llm.TierB)
	if err != nil {
		log.Fatal("llm tier B not found", zap.Error(err))
	}

	// ── Classifier (Stage 3 §3: always Tier A — fast routing decision) ───────
	classifierAgent := agent.NewClassifierAgent(tierAModel, log)

	// ── Triage agent ─────────────────────────────────────────────────────────
	// When ANTHROPIC_API_KEY is set, use the native Anthropic API with automatic
	// prompt cache injection (Stage 9). Otherwise fall back to OpenRouter/Eino path.
	var triageAgent agent.Triager
	if cfg.AnthropicAPIKey != "" {
		ac, acErr := anthropic.New(anthropic.Config{
			APIKey: cfg.AnthropicAPIKey,
			Model:  cfg.AnthropicModel,
		}, log)
		if acErr != nil {
			log.Fatal("anthropic client init failed", zap.Error(acErr))
		}
		at := agent.NewAnthropicTriager(ac, log)
		if ps != nil {
			at.WithPromptStore(ps)
		}
		triageAgent = at
		log.Info("triage: using Anthropic native API with prompt cache injection",
			zap.String("model", cfg.AnthropicModel),
		)
	} else {
		ta, taErr := agent.NewTriageAgent(ctx, tierBModel, log)
		if taErr != nil {
			log.Fatal("triage agent init failed", zap.Error(taErr))
		}
		triageAgent = ta
		log.Info("triage: using OpenRouter/Eino path")
	}

	// ── L1 Exact Cache (Stage 9) ─────────────────────────────────────────────
	// Wrap triageAgent with write-through L1 exact cache if Valkey is reachable.
	if cfg.ValkeyURL != "" {
		rdbOpts, rdbErr := redis.ParseURL(cfg.ValkeyURL)
		if rdbErr != nil {
			log.Warn("l1 cache: invalid VALKEY_URL, skipping", zap.Error(rdbErr))
		} else {
			rdb := redis.NewClient(rdbOpts)
			if pingErr := rdb.Ping(ctx).Err(); pingErr != nil {
				log.Warn("l1 cache: valkey ping failed, skipping", zap.Error(pingErr))
				_ = rdb.Close()
			} else {
				l1 := cache.NewValkeyL1(rdb)
				modelID := cfg.AnthropicModel
				if cfg.AnthropicAPIKey == "" {
					modelID = cfg.LLM.TierB
				}
				triageAgent = agent.NewCachedTriager(triageAgent, l1, modelID, log)
				log.Info("l1 cache enabled (valkey)", zap.String("url", cfg.ValkeyURL))
				// Close rdb when main exits — defer runs on return
				defer rdb.Close() //nolint:errcheck
			}
		}
	}

	// ── RAG context injection (Stage 6) ──────────────────────────────────────
	// Attach runbook retriever to triage agent when Qdrant is reachable.
	if qdrantURL := os.Getenv("QDRANT_URL"); qdrantURL != "" {
		qc := qdrant.New(qdrantURL, os.Getenv("QDRANT_API_KEY"), log)
		var embedder qdrant.Embedder = &qdrant.StubEmbedder{}
		if orKey := cfg.LLM.OpenRouterKey; orKey != "" {
			gatewayURL := cfg.LLM.GatewayURL
			if gatewayURL == "" {
				gatewayURL = "https://openrouter.ai/api/v1"
			}
			embedder = qdrant.NewHTTPEmbedder(gatewayURL, string(orKey), "BAAI/bge-m3", qdrant.EmbeddingDim)
		}
		if err := qc.EnsureCollection(ctx, qdrant.RunbookCollection, qdrant.EmbeddingDim); err != nil {
			log.Warn("rag: qdrant collection unavailable, skipping", zap.Error(err))
		} else {
			ragRetriever := qdrant.NewIndexer(qc, embedder)
			ragBuilder := agent.NewRAGContextBuilder(ragRetriever, log)
			if at, ok := triageAgent.(*agent.AnthropicTriager); ok {
				at.WithRAG(ragBuilder)
				log.Info("rag: runbook context injection enabled (qdrant)")
			}
		}
	}

	// ── RCA agent (Tier C for deeper reasoning) ───────────────────────────────
	tierCModel, err := llmClient.Model(llm.TierC)
	if err != nil {
		log.Fatal("llm tier C not found", zap.Error(err))
	}
	rcaAgent, err := agent.NewRCAAgent(ctx, tierCModel, log)
	if err != nil {
		log.Fatal("rca agent init failed", zap.Error(err))
	}

	// ── Stage 3 SupervisorPipeline (classify → route → specialist) ───────────
	// This is the production path. The worker dispatches through the supervisor
	// instead of calling triager/rca directly, giving us the classifier gate.
	supervisor := agent.NewSupervisorPipeline(classifierAgent, triageAgent, rcaAgent, log)

	// ── NATS ─────────────────────────────────────────────────────────────────
	natsClient, err := internalnats.Connect(cfg.NatsURL, log)
	if err != nil {
		log.Fatal("nats connect failed", zap.Error(err))
	}
	defer natsClient.Close()

	// ── Incident store + HTTP API ─────────────────────────────────────────────
	incStore := incident.NewStore()
	incHandler := incident.NewHandler(incStore, log).
		WithReplayPublisher(natsReplayAdapter{client: natsClient})

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Recoverer)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/metrics", promhttp.Handler().ServeHTTP)
	r.Route("/api/v1", func(r chi.Router) {
		incHandler.Routes(r)
	})

	agentPort := os.Getenv("PALADIN_AGENT_PORT")
	if agentPort == "" {
		agentPort = "9004"
	}
	httpSrv := &http.Server{
		Addr:         ":" + agentPort,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		log.Info("paladin-agent HTTP listening", zap.String("port", agentPort))
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server error", zap.Error(err))
		}
	}()

	// ── Worker ───────────────────────────────────────────────────────────────
	_ = incStore // incident store available for future worker integration

	pub := natsPublisherAdapter{client: natsClient}
	w := worker.New(triageAgent, pub, cfg.TriageTimeout, cfg.AgentWorkers, log).
		WithRCA(rcaAgent).
		WithRCATimeout(cfg.RCATimeout).
		WithSupervisor(supervisor)

	log.Info("paladin-agent starting",
		zap.String("consumer", cfg.NATSConsumerName),
		zap.Int("workers", cfg.AgentWorkers),
	)

	if err := w.Run(ctx, natsClient.JS(), cfg.NATSConsumerName); err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Error("worker exited with error", zap.Error(err))
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP shutdown error", zap.Error(err))
	}

	log.Info("paladin-agent stopped")
}
