// Package handler implements the MemoryService gRPC server.
package handler

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	memoryv1 "github.com/paladinai/paladinai/gen/go/memory/v1"
	"github.com/paladinai/paladinai/internal/qdrant"
	"github.com/paladinai/paladinai/services/paladin-memory/internal/store"
)

// ChunkSearcher is the common interface for Qdrant-backed memory tiers.
// Both procedural (runbooks) and semantic (fact chunks) implement this.
type ChunkSearcher interface {
	Search(ctx context.Context, query, tenantID string, topK int) ([]qdrant.RunbookChunk, error)
}

// ProceduralSearcher is an alias kept for backward compatibility.
type ProceduralSearcher = ChunkSearcher

// MemoryHandler implements memoryv1.MemoryServiceServer.
type MemoryHandler struct {
	memoryv1.UnimplementedMemoryServiceServer

	working    store.WorkingStore
	episodic   store.EpisodicStore
	procedural ChunkSearcher
	semantic   ChunkSearcher // optional; nil = tier not configured
	workingTTL time.Duration
	log        *zap.Logger
}

// New constructs a MemoryHandler. The logger must not be nil; callers can pass
// zap.NewNop() in tests.
func New(working store.WorkingStore, episodic store.EpisodicStore, workingTTL time.Duration, log *zap.Logger) *MemoryHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &MemoryHandler{
		working:    working,
		episodic:   episodic,
		workingTTL: workingTTL,
		log:        log,
	}
}

// WithProcedural attaches a procedural-memory searcher (Qdrant runbooks).
func (h *MemoryHandler) WithProcedural(p ChunkSearcher) *MemoryHandler {
	h.procedural = p
	return h
}

// WithSemantic attaches a semantic-memory searcher (Qdrant fact chunks).
func (h *MemoryHandler) WithSemantic(s ChunkSearcher) *MemoryHandler {
	h.semantic = s
	return h
}

// SearchMemory fans out to each requested memory tier and merges results.
func (h *MemoryHandler) SearchMemory(ctx context.Context, req *memoryv1.SearchMemoryRequest) (*memoryv1.SearchMemoryResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("memory: search: nil request")
	}
	if req.TenantID == "" {
		return nil, fmt.Errorf("memory: search: tenant_id is required")
	}
	topK := req.TopK
	if topK <= 0 {
		topK = 10
	}
	types := req.MemoryTypes
	if len(types) == 0 {
		types = []memoryv1.MemoryType{memoryv1.MemoryTypeEpisodic}
	}

	resp := &memoryv1.SearchMemoryResponse{}
	for _, mt := range types {
		switch mt {
		case memoryv1.MemoryTypeWorking:
			// SearchMemoryRequest carries no session_id; for working memory we
			// scan under a "*" session placeholder. Callers that need a
			// specific session should use GetWorkingMemory.
			values, err := h.working.Scan(ctx, req.TenantID, "*", req.Query, topK)
			if err != nil {
				return nil, fmt.Errorf("memory: search working: %w", err)
			}
			for _, v := range values {
				resp.Results = append(resp.Results, &memoryv1.MemoryResult{
					Type:    memoryv1.MemoryTypeWorking,
					Content: v,
					Score:   1.0,
					Source:  "working",
				})
			}
		case memoryv1.MemoryTypeEpisodic:
			episodes, err := h.episodic.Search(ctx, req.TenantID, req.Query, topK)
			if err != nil {
				return nil, fmt.Errorf("memory: search episodic: %w", err)
			}
			for _, ep := range episodes {
				resp.Results = append(resp.Results, &memoryv1.MemoryResult{
					Type:    memoryv1.MemoryTypeEpisodic,
					Content: ep.Summary,
					Score:   1.0,
					Source:  "episodic",
				})
			}
		case memoryv1.MemoryTypeProcedural:
			if h.procedural == nil {
				h.log.Debug("search: procedural memory not configured")
				continue
			}
			chunks, err := h.procedural.Search(ctx, req.Query, req.TenantID, topK)
			if err != nil {
				return nil, fmt.Errorf("memory: search procedural: %w", err)
			}
			for _, ch := range chunks {
				resp.Results = append(resp.Results, &memoryv1.MemoryResult{
					Type:    memoryv1.MemoryTypeProcedural,
					Content: ch.Content,
					Score:   1.0,
					Source:  ch.Source,
				})
			}
		case memoryv1.MemoryTypeSemantic:
			if h.semantic == nil {
				h.log.Debug("search: semantic memory not configured")
				continue
			}
			chunks, err := h.semantic.Search(ctx, req.Query, req.TenantID, topK)
			if err != nil {
				return nil, fmt.Errorf("memory: search semantic: %w", err)
			}
			for _, ch := range chunks {
				resp.Results = append(resp.Results, &memoryv1.MemoryResult{
					Type:    memoryv1.MemoryTypeSemantic,
					Content: ch.Content,
					Score:   1.0,
					Source:  ch.Source,
				})
			}
		default:
			h.log.Debug("search: unknown memory type", zap.Int("type", int(mt)))
		}
	}
	return resp, nil
}

// WriteEpisode persists a closed incident into episodic memory.
func (h *MemoryHandler) WriteEpisode(ctx context.Context, req *memoryv1.WriteEpisodeRequest) (*memoryv1.WriteEpisodeResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("memory: write episode: nil request")
	}
	id, err := h.episodic.Write(ctx, req)
	if err != nil {
		return nil, err
	}
	h.log.Info("episode written",
		zap.String("tenant_id", req.TenantID),
		zap.String("incident_id", req.IncidentID),
		zap.String("episode_id", id),
	)
	return &memoryv1.WriteEpisodeResponse{EpisodeID: id}, nil
}

// GetWorkingMemory returns the value at (tenant, session, key).
func (h *MemoryHandler) GetWorkingMemory(ctx context.Context, req *memoryv1.GetWorkingMemoryRequest) (*memoryv1.GetWorkingMemoryResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("memory: get working: nil request")
	}
	if req.TenantID == "" || req.SessionID == "" || req.Key == "" {
		return nil, fmt.Errorf("memory: get working: tenant_id, session_id, key are required")
	}
	v, found, err := h.working.Get(ctx, req.TenantID, req.SessionID, req.Key)
	if err != nil {
		return nil, err
	}
	return &memoryv1.GetWorkingMemoryResponse{Value: v, Found: found}, nil
}

// SetWorkingMemory writes a value at (tenant, session, key) with optional TTL.
// When TTLSeconds is <= 0 the handler default (workingTTL) is used.
func (h *MemoryHandler) SetWorkingMemory(ctx context.Context, req *memoryv1.SetWorkingMemoryRequest) (*memoryv1.SetWorkingMemoryResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("memory: set working: nil request")
	}
	if req.TenantID == "" || req.SessionID == "" || req.Key == "" {
		return nil, fmt.Errorf("memory: set working: tenant_id, session_id, key are required")
	}
	ttl := h.workingTTL
	if req.TTLSeconds > 0 {
		ttl = time.Duration(req.TTLSeconds) * time.Second
	}
	if err := h.working.Set(ctx, req.TenantID, req.SessionID, req.Key, req.Value, ttl); err != nil {
		return nil, err
	}
	return &memoryv1.SetWorkingMemoryResponse{}, nil
}

// DeleteEpisodes removes episodes by incident ID for the tenant.
func (h *MemoryHandler) DeleteEpisodes(ctx context.Context, req *memoryv1.DeleteEpisodesRequest) (*memoryv1.DeleteEpisodesResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("memory: delete: nil request")
	}
	n, err := h.episodic.Delete(ctx, req.TenantID, req.IncidentIDs)
	if err != nil {
		return nil, err
	}
	return &memoryv1.DeleteEpisodesResponse{Deleted: n}, nil
}
