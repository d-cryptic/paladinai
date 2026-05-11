package store

// White-box unit tests for PostgresStore using in-memory fakes.
// No real Postgres is needed — all pgx interfaces are satisfied by fakes below.
// These tests run in the standard `go test` pass (no build tags required).

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/paladinai/paladinai/services/paladin-hub/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── fake Row ────────────────────────────────────────────────────────────────

type fakeRow struct {
	scanFn func(dest ...any) error
}

func (r *fakeRow) Scan(dest ...any) error { return r.scanFn(dest...) }

// ─── fake Rows ───────────────────────────────────────────────────────────────

type fakeRows struct {
	data    []func(dest ...any) error
	idx     int
	rowsErr error
}

func (r *fakeRows) Close()                                       {}
func (r *fakeRows) Err() error                                   { return r.rowsErr }
func (r *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeRows) RawValues() [][]byte                          { return nil }
func (r *fakeRows) Conn() *pgx.Conn                              { return nil }
func (r *fakeRows) Next() bool {
	if r.idx >= len(r.data) {
		return false
	}
	r.idx++
	return true
}
func (r *fakeRows) Scan(dest ...any) error { return r.data[r.idx-1](dest...) }

// ─── fake Tx ─────────────────────────────────────────────────────────────────

type fakeTx struct {
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	commitFn   func(ctx context.Context) error
	rollbackFn func(ctx context.Context) error
}

func (t *fakeTx) Begin(ctx context.Context) (pgx.Tx, error) { return t, nil }
func (t *fakeTx) Commit(ctx context.Context) error {
	if t.commitFn != nil {
		return t.commitFn(ctx)
	}
	return nil
}
func (t *fakeTx) Rollback(ctx context.Context) error {
	if t.rollbackFn != nil {
		return t.rollbackFn(ctx)
	}
	return nil
}
func (t *fakeTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if t.execFn != nil {
		return t.execFn(ctx, sql, args...)
	}
	return pgconn.NewCommandTag("OK"), nil
}
func (t *fakeTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return &fakeRows{}, nil
}
func (t *fakeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return &fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
}
func (t *fakeTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *fakeTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults { return nil }
func (t *fakeTx) LargeObjects() pgx.LargeObjects                               { return pgx.LargeObjects{} }
func (t *fakeTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *fakeTx) Conn() *pgx.Conn { return nil }

// ─── fake Pool ───────────────────────────────────────────────────────────────

type fakePool struct {
	beginFn    func(ctx context.Context) (pgx.Tx, error)
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (p *fakePool) Begin(ctx context.Context) (pgx.Tx, error) {
	if p.beginFn != nil {
		return p.beginFn(ctx)
	}
	return &fakeTx{}, nil
}

func (p *fakePool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if p.execFn != nil {
		return p.execFn(ctx, sql, args...)
	}
	return pgconn.NewCommandTag("1"), nil
}

func (p *fakePool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if p.queryRowFn != nil {
		return p.queryRowFn(ctx, sql, args...)
	}
	return &fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
}

func (p *fakePool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if p.queryFn != nil {
		return p.queryFn(ctx, sql, args...)
	}
	return &fakeRows{}, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func newFakeStore(pool Pool) *PostgresStore {
	return newPostgresStoreFromPool(pool)
}

func fakeServerRow(s *registry.MCPServer) func(dest ...any) error {
	return func(dest ...any) error {
		if len(dest) != 9 {
			return errors.New("expected 9 scan destinations")
		}
		*(dest[0].(*string)) = s.ID
		*(dest[1].(*string)) = s.TenantID
		*(dest[2].(*string)) = s.Name
		*(dest[3].(*string)) = s.Description
		*(dest[4].(*string)) = s.Endpoint
		*(dest[5].(*[]string)) = s.Capabilities
		*(dest[6].(*bool)) = s.Healthy
		*(dest[7].(*time.Time)) = s.RegisteredAt
		*(dest[8].(*time.Time)) = s.LastSeenAt
		return nil
	}
}

// ─── MigrateUp tests ─────────────────────────────────────────────────────────

func TestPostgresStoreUnit_MigrateUp_Success(t *testing.T) {
	pool := &fakePool{
		beginFn: func(ctx context.Context) (pgx.Tx, error) {
			return &fakeTx{}, nil
		},
	}
	s := newFakeStore(pool)
	require.NoError(t, s.MigrateUp(context.Background()))
}

func TestPostgresStoreUnit_MigrateUp_BeginError(t *testing.T) {
	wantErr := errors.New("begin failed")
	pool := &fakePool{
		beginFn: func(ctx context.Context) (pgx.Tx, error) {
			return nil, wantErr
		},
	}
	s := newFakeStore(pool)
	err := s.MigrateUp(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}

func TestPostgresStoreUnit_MigrateUp_AdvisoryLockError(t *testing.T) {
	wantErr := errors.New("advisory lock failed")
	callCount := 0
	tx := &fakeTx{
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			callCount++
			if callCount == 1 {
				return pgconn.CommandTag{}, wantErr
			}
			return pgconn.NewCommandTag("OK"), nil
		},
	}
	pool := &fakePool{
		beginFn: func(ctx context.Context) (pgx.Tx, error) { return tx, nil },
	}
	s := newFakeStore(pool)
	err := s.MigrateUp(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}

func TestPostgresStoreUnit_MigrateUp_CreateTableError(t *testing.T) {
	wantErr := errors.New("create table failed")
	callCount := 0
	tx := &fakeTx{
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			callCount++
			if callCount == 2 {
				return pgconn.CommandTag{}, wantErr
			}
			return pgconn.NewCommandTag("OK"), nil
		},
	}
	pool := &fakePool{
		beginFn: func(ctx context.Context) (pgx.Tx, error) { return tx, nil },
	}
	s := newFakeStore(pool)
	err := s.MigrateUp(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}

func TestPostgresStoreUnit_MigrateUp_CommitError(t *testing.T) {
	wantErr := errors.New("commit failed")
	tx := &fakeTx{
		commitFn: func(ctx context.Context) error { return wantErr },
	}
	pool := &fakePool{
		beginFn: func(ctx context.Context) (pgx.Tx, error) { return tx, nil },
	}
	s := newFakeStore(pool)
	err := s.MigrateUp(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}

// ─── Upsert tests ────────────────────────────────────────────────────────────

func TestPostgresStoreUnit_Upsert_Success(t *testing.T) {
	pool := &fakePool{
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	s := newFakeStore(pool)
	srv := &registry.MCPServer{
		ID: "srv1", TenantID: "t1", Name: "Test", Endpoint: "http://x",
		Capabilities: []string{"a"}, Healthy: true,
		RegisteredAt: time.Now(), LastSeenAt: time.Now(),
	}
	require.NoError(t, s.Upsert(context.Background(), srv))
}

func TestPostgresStoreUnit_Upsert_Error(t *testing.T) {
	wantErr := errors.New("exec failed")
	pool := &fakePool{
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, wantErr
		},
	}
	s := newFakeStore(pool)
	err := s.Upsert(context.Background(), &registry.MCPServer{})
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}

// ─── Get tests ───────────────────────────────────────────────────────────────

func TestPostgresStoreUnit_Get_Found(t *testing.T) {
	srv := &registry.MCPServer{
		ID: "srv1", TenantID: "t1", Name: "Found", Endpoint: "http://x",
		Capabilities: []string{"tool"}, Healthy: true,
		RegisteredAt: time.Now().UTC().Truncate(time.Second),
		LastSeenAt:   time.Now().UTC().Truncate(time.Second),
	}
	pool := &fakePool{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{scanFn: fakeServerRow(srv)}
		},
	}
	s := newFakeStore(pool)
	got, err := s.Get(context.Background(), "t1", "srv1")
	require.NoError(t, err)
	assert.Equal(t, srv.Name, got.Name)
	assert.Equal(t, srv.Capabilities, got.Capabilities)
}

func TestPostgresStoreUnit_Get_NotFound(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
		},
	}
	s := newFakeStore(pool)
	_, err := s.Get(context.Background(), "t1", "missing")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPostgresStoreUnit_Get_ScanError(t *testing.T) {
	wantErr := errors.New("scan blew up")
	pool := &fakePool{
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &fakeRow{scanFn: func(dest ...any) error { return wantErr }}
		},
	}
	s := newFakeStore(pool)
	_, err := s.Get(context.Background(), "t1", "srv1")
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}

// ─── List tests ──────────────────────────────────────────────────────────────

func TestPostgresStoreUnit_List_Empty(t *testing.T) {
	pool := &fakePool{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &fakeRows{}, nil
		},
	}
	s := newFakeStore(pool)
	list, err := s.List(context.Background(), "t1")
	require.NoError(t, err)
	assert.NotNil(t, list, "nil slice would break JSON")
	assert.Len(t, list, 0)
}

func TestPostgresStoreUnit_List_MultipleRows(t *testing.T) {
	srv1 := &registry.MCPServer{
		ID: "s1", TenantID: "t1", Name: "A", Endpoint: "http://a",
		Capabilities: []string{}, Healthy: true,
		RegisteredAt: time.Now().UTC(), LastSeenAt: time.Now().UTC(),
	}
	srv2 := &registry.MCPServer{
		ID: "s2", TenantID: "t1", Name: "B", Endpoint: "http://b",
		Capabilities: []string{"x"}, Healthy: false,
		RegisteredAt: time.Now().UTC(), LastSeenAt: time.Now().UTC(),
	}
	pool := &fakePool{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &fakeRows{data: []func(dest ...any) error{
				fakeServerRow(srv1),
				fakeServerRow(srv2),
			}}, nil
		},
	}
	s := newFakeStore(pool)
	list, err := s.List(context.Background(), "t1")
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestPostgresStoreUnit_List_QueryError(t *testing.T) {
	wantErr := errors.New("query failed")
	pool := &fakePool{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, wantErr
		},
	}
	s := newFakeStore(pool)
	_, err := s.List(context.Background(), "t1")
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}

func TestPostgresStoreUnit_List_ScanError(t *testing.T) {
	wantErr := errors.New("scan error on row")
	pool := &fakePool{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &fakeRows{data: []func(dest ...any) error{
				func(dest ...any) error { return wantErr },
			}}, nil
		},
	}
	s := newFakeStore(pool)
	_, err := s.List(context.Background(), "t1")
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}

func TestPostgresStoreUnit_List_RowsErr(t *testing.T) {
	wantErr := errors.New("rows.Err")
	pool := &fakePool{
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &fakeRows{rowsErr: wantErr}, nil
		},
	}
	s := newFakeStore(pool)
	_, err := s.List(context.Background(), "t1")
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}

// ─── Delete tests ────────────────────────────────────────────────────────────

func TestPostgresStoreUnit_Delete_Success(t *testing.T) {
	pool := &fakePool{
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	s := newFakeStore(pool)
	require.NoError(t, s.Delete(context.Background(), "t1", "srv1"))
}

func TestPostgresStoreUnit_Delete_NotFound(t *testing.T) {
	pool := &fakePool{
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 0"), nil
		},
	}
	s := newFakeStore(pool)
	err := s.Delete(context.Background(), "t1", "missing")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPostgresStoreUnit_Delete_ExecError(t *testing.T) {
	wantErr := errors.New("exec failed")
	pool := &fakePool{
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, wantErr
		},
	}
	s := newFakeStore(pool)
	err := s.Delete(context.Background(), "t1", "srv1")
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}

// ─── Heartbeat tests ─────────────────────────────────────────────────────────

func TestPostgresStoreUnit_Heartbeat_Success(t *testing.T) {
	pool := &fakePool{
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	s := newFakeStore(pool)
	require.NoError(t, s.Heartbeat(context.Background(), "t1", "srv1", time.Now()))
}

func TestPostgresStoreUnit_Heartbeat_NotFound(t *testing.T) {
	pool := &fakePool{
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	s := newFakeStore(pool)
	err := s.Heartbeat(context.Background(), "t1", "missing", time.Now())
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPostgresStoreUnit_Heartbeat_ExecError(t *testing.T) {
	wantErr := errors.New("heartbeat exec failed")
	pool := &fakePool{
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, wantErr
		},
	}
	s := newFakeStore(pool)
	err := s.Heartbeat(context.Background(), "t1", "srv1", time.Now())
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}
