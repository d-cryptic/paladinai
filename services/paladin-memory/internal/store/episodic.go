package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	memoryv1 "github.com/paladinai/paladinai/gen/go/memory/v1"
)

// Episode is a row in episodic_memory.
type Episode struct {
	ID          string
	TenantID    string
	IncidentID  string
	Fingerprint string
	Summary     string
	RootCause   string
	Resolution  string
	Severity    string
	Labels      map[string]string
	ValidAt     time.Time
	RecordedAt  time.Time
}

// EpisodicStore is the contract for durable incident memory.
type EpisodicStore interface {
	Write(ctx context.Context, req *memoryv1.WriteEpisodeRequest) (string, error)
	Search(ctx context.Context, tenantID, query string, topK int) ([]*Episode, error)
	Delete(ctx context.Context, tenantID string, incidentIDs []string) (int64, error)
}

// PostgresEpisodicStore is a pgx/v5-backed EpisodicStore.
type PostgresEpisodicStore struct {
	pool *pgxpool.Pool
}

func NewPostgresEpisodicStore(pool *pgxpool.Pool) *PostgresEpisodicStore {
	return &PostgresEpisodicStore{pool: pool}
}

// Write inserts a new episode and returns the generated episode ID.
func (p *PostgresEpisodicStore) Write(ctx context.Context, req *memoryv1.WriteEpisodeRequest) (string, error) {
	if req == nil {
		return "", fmt.Errorf("memory: episodic write: nil request")
	}
	if req.TenantID == "" || req.IncidentID == "" {
		return "", fmt.Errorf("memory: episodic write: tenant_id and incident_id are required")
	}
	labels := req.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	labelsJSON, err := json.Marshal(labels)
	if err != nil {
		return "", fmt.Errorf("memory: episodic write marshal labels: %w", err)
	}

	var id string
	err = p.pool.QueryRow(ctx, `
		INSERT INTO episodic_memory
			(tenant_id, incident_id, fingerprint, summary, root_cause, resolution, severity, labels)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id::text`,
		req.TenantID, req.IncidentID, req.Fingerprint, req.Summary,
		nullableString(req.RootCause), nullableString(req.Resolution),
		req.Severity, labelsJSON,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("memory: episodic write: %w", err)
	}
	return id, nil
}

// Search returns the top-K most recent episodes whose fingerprint, severity, or
// summary contains the given query string. Simple text match — no vector
// search at this stage.
func (p *PostgresEpisodicStore) Search(ctx context.Context, tenantID, query string, topK int) ([]*Episode, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("memory: episodic search: tenant_id is required")
	}
	if topK <= 0 {
		topK = 10
	}
	like := "%" + strings.ToLower(query) + "%"
	rows, err := p.pool.Query(ctx, `
		SELECT id::text, tenant_id::text, incident_id::text, fingerprint,
			   summary, COALESCE(root_cause, ''), COALESCE(resolution, ''),
			   severity, labels, valid_at, recorded_at
		FROM   episodic_memory
		WHERE  tenant_id = $1
		  AND ($2 = '' OR LOWER(summary)     LIKE $3
		               OR LOWER(fingerprint) LIKE $3
		               OR LOWER(severity)    LIKE $3)
		ORDER BY valid_at DESC
		LIMIT $4`,
		tenantID, query, like, topK,
	)
	if err != nil {
		return nil, fmt.Errorf("memory: episodic search: %w", err)
	}
	defer rows.Close()

	var out []*Episode
	for rows.Next() {
		var (
			ep         Episode
			labelsJSON []byte
		)
		if err := rows.Scan(&ep.ID, &ep.TenantID, &ep.IncidentID, &ep.Fingerprint,
			&ep.Summary, &ep.RootCause, &ep.Resolution, &ep.Severity,
			&labelsJSON, &ep.ValidAt, &ep.RecordedAt); err != nil {
			return nil, fmt.Errorf("memory: episodic search scan: %w", err)
		}
		labels := map[string]string{}
		if len(labelsJSON) > 0 {
			if err := json.Unmarshal(labelsJSON, &labels); err != nil {
				return nil, fmt.Errorf("memory: episodic search unmarshal labels: %w", err)
			}
		}
		ep.Labels = labels
		out = append(out, &ep)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("memory: episodic search rows: %w", err)
	}
	return out, nil
}

// Delete removes episodes matching the (tenantID, incidentIDs) tuple.
// Returns the number of rows deleted.
func (p *PostgresEpisodicStore) Delete(ctx context.Context, tenantID string, incidentIDs []string) (int64, error) {
	if tenantID == "" {
		return 0, fmt.Errorf("memory: episodic delete: tenant_id is required")
	}
	if len(incidentIDs) == 0 {
		return 0, nil
	}
	// Validate UUIDs so we can pass them as a uuid[] to the query.
	parsed := make([]uuid.UUID, 0, len(incidentIDs))
	for _, id := range incidentIDs {
		u, err := uuid.Parse(id)
		if err != nil {
			return 0, fmt.Errorf("memory: episodic delete: invalid incident_id %q: %w", id, err)
		}
		parsed = append(parsed, u)
	}
	tag, err := p.pool.Exec(ctx,
		`DELETE FROM episodic_memory WHERE tenant_id = $1 AND incident_id = ANY($2)`,
		tenantID, parsed,
	)
	if err != nil {
		return 0, fmt.Errorf("memory: episodic delete: %w", err)
	}
	return tag.RowsAffected(), nil
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
