// White-box tests: package db gives access to tenantIDKey.
package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestWithTenantID_ValueIsAccessible(t *testing.T) {
	t.Parallel()
	ctx := WithTenantID(context.Background(), "acme-corp")
	got, ok := ctx.Value(tenantIDKey{}).(string)
	assert.True(t, ok, "value should be a string")
	assert.Equal(t, "acme-corp", got)
}

func TestWithTenantID_EmptyID_IsStored(t *testing.T) {
	t.Parallel()
	// An empty tenant ID is a valid (if unusual) value; the caller controls validity.
	ctx := WithTenantID(context.Background(), "")
	got, ok := ctx.Value(tenantIDKey{}).(string)
	require.True(t, ok, "value must be present even when empty")
	assert.Equal(t, "", got)
}

func TestWithTenantID_OverridesParentValue(t *testing.T) {
	t.Parallel()
	parent := WithTenantID(context.Background(), "tenant-a")
	child := WithTenantID(parent, "tenant-b")
	got, _ := child.Value(tenantIDKey{}).(string)
	assert.Equal(t, "tenant-b", got, "child context should shadow the parent value")
}

func TestWithTenantID_ParentUnmodified(t *testing.T) {
	t.Parallel()
	parent := WithTenantID(context.Background(), "tenant-a")
	_ = WithTenantID(parent, "tenant-b")
	got, _ := parent.Value(tenantIDKey{}).(string)
	assert.Equal(t, "tenant-a", got, "creating a child context must not modify the parent")
}

func TestWithTenantID_BaseContextLacksValue(t *testing.T) {
	t.Parallel()
	// Verify that Background() has no tenant ID — the test above's parent check depends on this.
	_, ok := context.Background().Value(tenantIDKey{}).(string)
	assert.False(t, ok, "base context must not carry a tenant ID before WithTenantID is called")
}

func TestWithTenantID_IsolatedAcrossContexts(t *testing.T) {
	t.Parallel()
	// Independent contexts must carry independent tenant IDs.
	ctxA := WithTenantID(context.Background(), "tenant-alpha")
	ctxB := WithTenantID(context.Background(), "tenant-beta")

	gotA, _ := ctxA.Value(tenantIDKey{}).(string)
	gotB, _ := ctxB.Value(tenantIDKey{}).(string)

	assert.Equal(t, "tenant-alpha", gotA)
	assert.Equal(t, "tenant-beta", gotB)
}

// ─── Connect tests ───────────────────────────────────────────────────────────

func TestConnect_InvalidDSN_ReturnsParseError(t *testing.T) {
	t.Parallel()
	// A malformed DSN must fail at pgxpool.ParseConfig before any network I/O.
	pool, err := Connect(context.Background(), "://not-a-valid-dsn", zap.NewNop())

	require.Error(t, err)
	assert.Nil(t, pool, "no pool should be returned on parse failure")
	assert.Contains(t, err.Error(), "db: parse config", "error must be wrapped with parse-config context")
}

func TestConnect_UnreachableHost_FailsPing(t *testing.T) {
	t.Parallel()
	// DSN parses cleanly but the host is unroutable; pool.Ping must fail with the
	// "db: ping" wrapper. This exercises the entire Connect happy path up to (but
	// not including) a successful connection: ParseConfig, pool-tuning assignments,
	// hook registration, and NewWithConfig.
	//
	// 127.0.0.1:1 (tcpmux) typically refuses connections quickly; we also bound the
	// dial via a short ctx timeout so the test is fast even on platforms that drop.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	dsn := "postgres://user:pass@127.0.0.1:1/postgres?sslmode=disable&connect_timeout=1"
	pool, err := Connect(ctx, dsn, zap.NewNop())

	require.Error(t, err, "ping against unreachable host must fail")
	assert.Nil(t, pool, "pool must be closed and nil when Connect fails")
	// Error originates either from NewWithConfig or Ping — both are wrapped paths
	// inside Connect, so coverage hits the same body either way.
	msg := err.Error()
	assert.True(t,
		contains(msg, "db: ping") || contains(msg, "db: connect"),
		"error must be wrapped by Connect (got %q)", msg,
	)
}

// ─── beforeAcquire / afterRelease hook tests ─────────────────────────────────

type fakeExecer struct {
	gotSQL  string
	gotArgs []any
	calls   int
	err     error
}

func (f *fakeExecer) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.calls++
	f.gotSQL = sql
	f.gotArgs = args
	return pgconn.CommandTag{}, f.err
}

func TestBeforeAcquire_InjectsTenantID(t *testing.T) {
	t.Parallel()
	exec := &fakeExecer{}
	ctx := WithTenantID(context.Background(), "tenant-xyz")

	ok := beforeAcquire(ctx, exec, zap.NewNop())

	assert.True(t, ok, "successful set_config must allow acquisition")
	assert.Equal(t, 1, exec.calls)
	assert.Contains(t, exec.gotSQL, "set_config")
	assert.Contains(t, exec.gotSQL, "app.tenant_id")
	require.Len(t, exec.gotArgs, 1, "tenant ID must be passed as a parameterized arg")
	assert.Equal(t, "tenant-xyz", exec.gotArgs[0])
}

func TestBeforeAcquire_NoTenantInContext_DeniesConnection(t *testing.T) {
	t.Parallel()
	exec := &fakeExecer{}

	ok := beforeAcquire(context.Background(), exec, zap.NewNop())

	assert.False(t, ok, "missing tenant ID must deny connection to fail closed")
	assert.Empty(t, exec.gotArgs, "no Exec must be called when tenant is missing")
}

func TestBeforeAcquire_ExecError_RejectsConnection(t *testing.T) {
	t.Parallel()
	exec := &fakeExecer{err: errors.New("server gone")}
	ctx := WithTenantID(context.Background(), "tenant-xyz")

	ok := beforeAcquire(ctx, exec, zap.NewNop())

	assert.False(t, ok, "Exec failure must reject the connection so it is not reused")
}

func TestAfterRelease_ClearsTenantID(t *testing.T) {
	t.Parallel()
	exec := &fakeExecer{}

	ok := afterRelease(exec, zap.NewNop())

	assert.True(t, ok)
	assert.Equal(t, 1, exec.calls)
	assert.Contains(t, exec.gotSQL, "set_config")
	assert.Contains(t, exec.gotSQL, "app.tenant_id")
	// Reset uses a literal empty string, not a parameter.
	assert.Empty(t, exec.gotArgs, "afterRelease must not parameterize the reset value")
}

func TestAfterRelease_ExecError_RejectsConnection(t *testing.T) {
	t.Parallel()
	exec := &fakeExecer{err: errors.New("connection lost")}

	ok := afterRelease(exec, zap.NewNop())

	assert.False(t, ok, "failure to reset session must drop the connection from the pool")
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
