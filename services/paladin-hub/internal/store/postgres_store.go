package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/paladinai/paladinai/services/paladin-hub/internal/registry"
)

const createTableSQL = `
CREATE TABLE IF NOT EXISTS mcp_servers (
    id            TEXT        NOT NULL,
    tenant_id     TEXT        NOT NULL,
    name          TEXT        NOT NULL,
    description   TEXT        NOT NULL DEFAULT '',
    endpoint      TEXT        NOT NULL,
    capabilities  TEXT[]      NOT NULL DEFAULT '{}',
    healthy       BOOLEAN     NOT NULL DEFAULT TRUE,
    registered_at TIMESTAMPTZ NOT NULL,
    last_seen_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, id)
)`

// PostgresStore is a production Store backed by PostgreSQL via pgx/v5.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore wraps an existing connection pool.
// Call MigrateUp before first use to ensure the table exists.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// MigrateUp creates the mcp_servers table if it does not already exist.
// Idempotent; safe to call at startup.
func (p *PostgresStore) MigrateUp(ctx context.Context) error {
	_, err := p.pool.Exec(ctx, createTableSQL)
	if err != nil {
		return fmt.Errorf("postgres store migrate: %w", err)
	}
	return nil
}

// Upsert inserts or replaces a server record.
func (p *PostgresStore) Upsert(ctx context.Context, s *registry.MCPServer) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO mcp_servers
			(id, tenant_id, name, description, endpoint, capabilities, healthy, registered_at, last_seen_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (tenant_id, id) DO UPDATE SET
			name          = EXCLUDED.name,
			description   = EXCLUDED.description,
			endpoint      = EXCLUDED.endpoint,
			capabilities  = EXCLUDED.capabilities,
			healthy       = EXCLUDED.healthy,
			last_seen_at  = EXCLUDED.last_seen_at`,
		s.ID, s.TenantID, s.Name, s.Description, s.Endpoint,
		s.Capabilities, s.Healthy, s.RegisteredAt, s.LastSeenAt,
	)
	if err != nil {
		return fmt.Errorf("postgres store upsert: %w", err)
	}
	return nil
}

// Get returns a server by tenantID + serverID; returns ErrNotFound if absent.
func (p *PostgresStore) Get(ctx context.Context, tenantID, serverID string) (*registry.MCPServer, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, description, endpoint, capabilities, healthy, registered_at, last_seen_at
		FROM   mcp_servers
		WHERE  tenant_id = $1 AND id = $2`,
		tenantID, serverID,
	)
	s, err := scanServer(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("postgres store get: %w", err)
	}
	return s, nil
}

// List returns all servers for a tenant; returns an empty slice if none found.
func (p *PostgresStore) List(ctx context.Context, tenantID string) ([]*registry.MCPServer, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, tenant_id, name, description, endpoint, capabilities, healthy, registered_at, last_seen_at
		FROM   mcp_servers
		WHERE  tenant_id = $1
		ORDER BY registered_at ASC`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres store list: %w", err)
	}
	defer rows.Close()

	var result []*registry.MCPServer
	for rows.Next() {
		s, err := scanServer(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres store list scan: %w", err)
		}
		result = append(result, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres store list rows: %w", err)
	}
	if result == nil {
		result = []*registry.MCPServer{}
	}
	return result, nil
}

// Delete removes a server; returns ErrNotFound if absent.
func (p *PostgresStore) Delete(ctx context.Context, tenantID, serverID string) error {
	tag, err := p.pool.Exec(ctx,
		`DELETE FROM mcp_servers WHERE tenant_id = $1 AND id = $2`,
		tenantID, serverID,
	)
	if err != nil {
		return fmt.Errorf("postgres store delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Heartbeat updates LastSeenAt and sets Healthy=true for a server.
func (p *PostgresStore) Heartbeat(ctx context.Context, tenantID, serverID string, at time.Time) error {
	tag, err := p.pool.Exec(ctx,
		`UPDATE mcp_servers SET last_seen_at = $1, healthy = TRUE WHERE tenant_id = $2 AND id = $3`,
		at, tenantID, serverID,
	)
	if err != nil {
		return fmt.Errorf("postgres store heartbeat: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// scanner is the common scan interface satisfied by pgx.Row and pgx.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanServer(row scanner) (*registry.MCPServer, error) {
	var s registry.MCPServer
	err := row.Scan(
		&s.ID, &s.TenantID, &s.Name, &s.Description, &s.Endpoint,
		&s.Capabilities, &s.Healthy, &s.RegisteredAt, &s.LastSeenAt,
	)
	if err != nil {
		return nil, err
	}
	if s.Capabilities == nil {
		s.Capabilities = []string{}
	}
	return &s, nil
}
