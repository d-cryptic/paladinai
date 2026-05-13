// Package db provides PostgreSQL connection management for PaladinAI services.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Connect opens a pgxpool connection to the database at dsn and pings it.
// The pool automatically injects the tenant ID into each connection so that
// PostgreSQL RLS policies (keyed on app.tenant_id) activate correctly.
//
// Tenant injection:
//   - BeforeAcquire: SET app.tenant_id to the value in ctx (session-level, is_local=false)
//   - AfterRelease: RESET app.tenant_id so the next borrower starts clean
//
// Note: is_local=false is required here because BeforeAcquire runs outside an
// explicit transaction. is_local=true (transaction-local) would be discarded
// immediately after the implicit single-statement transaction ends.
func Connect(ctx context.Context, dsn string, log *zap.Logger) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}

	// Production-safe pool sizing and timeouts.
	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second

	// Statement and lock timeouts prevent runaway queries from starving the pool.
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "5000" // 5 s
	cfg.ConnConfig.RuntimeParams["lock_timeout"] = "2000"      // 2 s

	cfg.BeforeAcquire = func(ctx context.Context, conn *pgx.Conn) bool { //nolint:staticcheck
		return beforeAcquire(ctx, conn, log)
	}

	cfg.AfterRelease = func(conn *pgx.Conn) bool {
		return afterRelease(conn, log)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return pool, nil
}

// WithTenantID stores the tenant ID in ctx so BeforeAcquire injects it.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey{}, tenantID)
}

type tenantIDKey struct{}

// connExecer is the narrow subset of *pgx.Conn used by the tenant-injection
// hooks. Keeping the surface minimal makes the hook bodies unit-testable
// without spinning up a real Postgres connection.
type connExecer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// beforeAcquire injects the tenant ID into the session before the connection
// is handed to the caller. A failed SET CONFIG aborts acquisition so the
// caller never sees a connection with a stale or missing tenant.
func beforeAcquire(ctx context.Context, conn connExecer, log *zap.Logger) bool {
	tenantID, _ := ctx.Value(tenantIDKey{}).(string)
	if _, err := conn.Exec(ctx,
		"SELECT set_config('app.tenant_id', $1, false)", tenantID); err != nil {
		log.Warn("db: BeforeAcquire set_config failed", zap.Error(err))
		return false
	}
	return true
}

// afterRelease clears the tenant ID after the caller returns the connection
// to the pool so the next borrower starts from a clean session.
func afterRelease(conn connExecer, log *zap.Logger) bool {
	if _, err := conn.Exec(context.Background(),
		"SELECT set_config('app.tenant_id', '', false)"); err != nil {
		log.Warn("db: AfterRelease reset failed", zap.Error(err))
		return false
	}
	return true
}
