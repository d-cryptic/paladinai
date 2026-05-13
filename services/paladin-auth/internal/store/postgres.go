package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const tenantsLockID = int64(0x706c647461757468) // "pldtauth" in hex

type execer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// PostgresStore is a pgx-backed Store for production use.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore wraps an existing connection pool.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) MigrateUp(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, `SELECT pg_advisory_lock($1)`, tenantsLockID); err != nil {
		return fmt.Errorf("auth store migrate advisory lock: %w", err)
	}
	defer func() {
		_, _ = s.pool.Exec(ctx, `SELECT pg_advisory_unlock($1)`, tenantsLockID)
	}()

	if err := migrateTenantsTable(ctx, s.pool); err != nil {
		return err
	}
	return nil
}

func migrateTenantsTable(ctx context.Context, db execer) error {
	if _, err := db.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pgcrypto`); err != nil {
		return fmt.Errorf("auth store migrate pgcrypto: %w", err)
	}
	if _, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS tenants (
			id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
			slug        TEXT        NOT NULL,
			name        TEXT        NOT NULL,
			state       TEXT        NOT NULL DEFAULT 'active'
			                        CHECK (state IN ('active', 'suspended')),
			created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("auth store migrate create tenants: %w", err)
	}
	if _, err := db.Exec(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS idx_tenants_slug ON tenants (slug)`); err != nil {
		return fmt.Errorf("auth store migrate tenants slug index: %w", err)
	}
	if _, err := db.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_tenants_state ON tenants (state)`); err != nil {
		return fmt.Errorf("auth store migrate tenants state index: %w", err)
	}
	return nil
}

func (s *PostgresStore) Create(ctx context.Context, slug, name string) (*Tenant, error) {
	if slug == "" {
		return nil, fmt.Errorf("auth store: slug is required")
	}
	const q = `
		INSERT INTO tenants (slug, name, state)
		VALUES ($1, $2, $3)
		RETURNING id, slug, name, state, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, slug, name, string(TenantStateActive))
	t, err := scanTenant(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrAlreadyExists
		}
		return nil, fmt.Errorf("auth store: create tenant: %w", err)
	}
	return t, nil
}

func (s *PostgresStore) Get(ctx context.Context, id string) (*Tenant, error) {
	const q = `SELECT id, slug, name, state, created_at, updated_at FROM tenants WHERE id = $1`
	row := s.pool.QueryRow(ctx, q, id)
	t, err := scanTenant(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("auth store: get tenant: %w", err)
	}
	return t, nil
}

func (s *PostgresStore) GetBySlug(ctx context.Context, slug string) (*Tenant, error) {
	const q = `SELECT id, slug, name, state, created_at, updated_at FROM tenants WHERE slug = $1`
	row := s.pool.QueryRow(ctx, q, slug)
	t, err := scanTenant(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("auth store: get tenant by slug: %w", err)
	}
	return t, nil
}

func (s *PostgresStore) List(ctx context.Context) ([]*Tenant, error) {
	const q = `SELECT id, slug, name, state, created_at, updated_at FROM tenants ORDER BY created_at`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("auth store: list tenants: %w", err)
	}
	defer rows.Close()

	var out []*Tenant
	for rows.Next() {
		t := &Tenant{}
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.State, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("auth store: list scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *PostgresStore) SetState(ctx context.Context, id string, state TenantState) (*Tenant, error) {
	const q = `
		UPDATE tenants SET state = $1, updated_at = $2
		WHERE id = $3
		RETURNING id, slug, name, state, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, string(state), time.Now().UTC(), id)
	t, err := scanTenant(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("auth store: set state: %w", err)
	}
	return t, nil
}

func scanTenant(row pgx.Row) (*Tenant, error) {
	t := &Tenant{}
	err := row.Scan(&t.ID, &t.Slug, &t.Name, &t.State, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// isUniqueViolation checks for Postgres unique constraint violation (code 23505).
func isUniqueViolation(err error) bool {
	return err != nil && (containsCode(err, "23505") || containsMsg(err, "unique"))
}

func containsCode(err error, code string) bool {
	type pgErr interface{ SQLState() string }
	var pe pgErr
	if errors.As(err, &pe) {
		return pe.SQLState() == code
	}
	return false
}

func containsMsg(err error, substr string) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for i := 0; i+len(substr) <= len(msg); i++ {
		if msg[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
