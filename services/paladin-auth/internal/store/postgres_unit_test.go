package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePgErr implements the pgx-style SQLState interface so we can exercise
// the unique-violation branch of containsCode without a real database.
type fakePgErr struct {
	state string
	msg   string
}

func (e *fakePgErr) Error() string    { return e.msg }
func (e *fakePgErr) SQLState() string { return e.state }

func TestIsUniqueViolation_NilReturnsFalse(t *testing.T) {
	assert.False(t, isUniqueViolation(nil))
}

func TestIsUniqueViolation_CodeMatch(t *testing.T) {
	err := &fakePgErr{state: "23505", msg: "duplicate key"}
	assert.True(t, isUniqueViolation(err))
}

func TestIsUniqueViolation_WrongCode(t *testing.T) {
	err := &fakePgErr{state: "23502", msg: "not-null violation"}
	assert.False(t, isUniqueViolation(err))
}

func TestIsUniqueViolation_WrappedPgError(t *testing.T) {
	inner := &fakePgErr{state: "23505"}
	wrapped := errors.Join(errors.New("outer"), inner)
	assert.True(t, isUniqueViolation(wrapped))
}

func TestIsUniqueViolation_PlainErrorNoMatch(t *testing.T) {
	// Without a pgErr SQLState, plain errors are not treated as unique violations.
	err := errors.New("duplicate key violates unique constraint")
	assert.False(t, isUniqueViolation(err))
}

func TestIsUniqueViolation_UnrelatedError(t *testing.T) {
	err := errors.New("unrelated error")
	assert.False(t, isUniqueViolation(err))
}

// stubRow implements pgx.Row to exercise scanTenant directly without a pool.
type stubRow struct {
	err    error
	values []any
}

func (r *stubRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return errors.New("stubRow: dest length mismatch")
	}
	for i, d := range dest {
		switch p := d.(type) {
		case *string:
			*p = r.values[i].(string)
		case *TenantState:
			*p = TenantState(r.values[i].(string))
		case *time.Time:
			*p = r.values[i].(time.Time)
		default:
			return errors.New("stubRow: unsupported dest type")
		}
	}
	return nil
}

func TestScanTenant_Success(t *testing.T) {
	now := time.Now().UTC()
	row := &stubRow{values: []any{"id-1", "slug-1", "Name", "active", now, now}}
	tn, err := scanTenant(row)
	require.NoError(t, err)
	assert.Equal(t, "id-1", tn.ID)
	assert.Equal(t, "slug-1", tn.Slug)
	assert.Equal(t, TenantStateActive, tn.State)
}

func TestScanTenant_Error(t *testing.T) {
	row := &stubRow{err: pgx.ErrNoRows}
	tn, err := scanTenant(row)
	assert.Nil(t, tn)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}

// ─── PostgresStore method-level tests with an unreachable pool ──────────────

// newUnreachablePool creates a pool pointing at a port nothing listens on so
// every Query/Exec returns a connection error. Useful to exercise error paths
// without a real database.
func newUnreachablePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig("postgres://nobody:nobody@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")
	require.NoError(t, err)
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func TestNewPostgresStore_Wraps(t *testing.T) {
	pool := newUnreachablePool(t)
	s := NewPostgresStore(pool)
	assert.NotNil(t, s)
}

func TestPostgresStore_Create_EmptySlug(t *testing.T) {
	s := NewPostgresStore(nil) // safe — validation runs before pool access
	_, err := s.Create(context.Background(), "", "Name")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slug is required")
}

func TestPostgresStore_Create_ConnectionFailure(t *testing.T) {
	pool := newUnreachablePool(t)
	s := NewPostgresStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := s.Create(ctx, "slug", "Name")
	require.Error(t, err)
	// Not a unique violation — should be wrapped as create error.
	assert.Contains(t, err.Error(), "auth store: create tenant")
}

func TestPostgresStore_Get_ConnectionFailure(t *testing.T) {
	pool := newUnreachablePool(t)
	s := NewPostgresStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := s.Get(ctx, "id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth store: get tenant")
}

func TestPostgresStore_GetBySlug_ConnectionFailure(t *testing.T) {
	pool := newUnreachablePool(t)
	s := NewPostgresStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := s.GetBySlug(ctx, "slug")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth store: get tenant by slug")
}

func TestPostgresStore_List_ConnectionFailure(t *testing.T) {
	pool := newUnreachablePool(t)
	s := NewPostgresStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := s.List(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth store: list tenants")
}

func TestPostgresStore_SetState_ConnectionFailure(t *testing.T) {
	pool := newUnreachablePool(t)
	s := NewPostgresStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := s.SetState(ctx, "id", TenantStateSuspended)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth store: set state")
}
