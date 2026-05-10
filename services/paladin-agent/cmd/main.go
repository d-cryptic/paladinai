// Command paladin-agent is the PaladinAI agent runtime.
// It consumes correlated alerts from NATS and dispatches them through an
// Eino-based agent graph (Triage → RCA → Runbook → Comms).
package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/paladinai/paladinai/internal/logger"
	internalnats "github.com/paladinai/paladinai/internal/nats"
	agentcfg "github.com/paladinai/paladinai/services/paladin-agent/config"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/agent"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/llm"
	"github.com/paladinai/paladinai/services/paladin-agent/internal/worker"
	"go.uber.org/zap"
)

func main() {
	cfg, err := agentcfg.Load()
	if err != nil {
		log, _ := zap.NewProduction()
		log.Fatal("config load failed", zap.Error(err))
	}

	log, err := logger.New("paladin-agent")
	if err != nil {
		panic(err)
	}
	defer log.Sync() //nolint:errcheck

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── LLM client ───────────────────────────────────────────────────────────
	llmClient, err := llm.New(ctx, llm.Config{
		BaseURL:    cfg.LLM.GatewayURL,
		APIKey:     llm.Secret(cfg.LLM.OpenRouterKey),
		ModelTierA: cfg.LLM.TierA,
		ModelTierB: cfg.LLM.TierB,
		ModelTierC: cfg.LLM.TierC,
	})
	if err != nil {
		log.Fatal("llm client init failed", zap.Error(err))
	}

	tierBModel, err := llmClient.Model(llm.TierB)
	if err != nil {
		log.Fatal("llm tier B not found", zap.Error(err))
	}

	// ── Triage agent ─────────────────────────────────────────────────────────
	triageAgent, err := agent.NewTriageAgent(ctx, tierBModel, log)
	if err != nil {
		log.Fatal("triage agent init failed", zap.Error(err))
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

	// ── NATS ─────────────────────────────────────────────────────────────────
	natsClient, err := internalnats.Connect(cfg.NatsURL, log)
	if err != nil {
		log.Fatal("nats connect failed", zap.Error(err))
	}
	defer natsClient.Close()

	// ── Worker ───────────────────────────────────────────────────────────────
	w := worker.New(triageAgent, natsClient, cfg.TriageTimeout, cfg.AgentWorkers, log).WithRCA(rcaAgent)

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

	log.Info("paladin-agent stopped")
}
