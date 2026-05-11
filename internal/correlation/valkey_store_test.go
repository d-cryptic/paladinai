package correlation_test

// Unit tests for ValkeyStore using a fake RedisClient.
// No real Valkey/Redis server is needed — the RedisClient interface is
// satisfied by fakeRedis below.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paladinai/paladinai/internal/correlation"
)

// ─── fake RedisClient ─────────────────────────────────────────────────────────

type fakeRedis struct {
	store   map[string]fakeRedisEntry
	setNXFn func(ctx context.Context, key string, value any, exp time.Duration) *redis.BoolCmd
	getFn   func(ctx context.Context, key string) *redis.StringCmd
}

type fakeRedisEntry struct {
	value  string
	expiry time.Time
}

func newFakeRedis() *fakeRedis {
	return &fakeRedis{store: make(map[string]fakeRedisEntry)}
}

func (r *fakeRedis) SetNX(ctx context.Context, key string, value any, exp time.Duration) *redis.BoolCmd {
	if r.setNXFn != nil {
		return r.setNXFn(ctx, key, value, exp)
	}
	cmd := redis.NewBoolCmd(ctx)
	if e, ok := r.store[key]; ok && time.Now().Before(e.expiry) {
		cmd.SetVal(false)
		return cmd
	}
	r.store[key] = fakeRedisEntry{value: value.(string), expiry: time.Now().Add(exp)}
	cmd.SetVal(true)
	return cmd
}

func (r *fakeRedis) Get(ctx context.Context, key string) *redis.StringCmd {
	if r.getFn != nil {
		return r.getFn(ctx, key)
	}
	cmd := redis.NewStringCmd(ctx)
	e, ok := r.store[key]
	if !ok || time.Now().After(e.expiry) {
		cmd.SetErr(redis.Nil)
		return cmd
	}
	cmd.SetVal(e.value)
	return cmd
}

// ─── tests ────────────────────────────────────────────────────────────────────

func TestValkeyStore_GetOrSet_NewKey(t *testing.T) {
	rdb := newFakeRedis()
	s := correlation.NewValkeyStoreFromClient(rdb)

	val, wasNew, err := s.GetOrSet(context.Background(), "key1", "corr-abc", time.Minute)
	require.NoError(t, err)
	assert.True(t, wasNew, "first call should create a new entry")
	assert.Equal(t, "corr-abc", val)
}

func TestValkeyStore_GetOrSet_ExistingKey(t *testing.T) {
	rdb := newFakeRedis()
	s := correlation.NewValkeyStoreFromClient(rdb)
	ctx := context.Background()

	_, _, err := s.GetOrSet(ctx, "key1", "corr-first", time.Minute)
	require.NoError(t, err)

	val, wasNew, err := s.GetOrSet(ctx, "key1", "corr-second", time.Minute)
	require.NoError(t, err)
	assert.False(t, wasNew, "second call should not create a new entry")
	assert.Equal(t, "corr-first", val, "should return the first value stored")
}

func TestValkeyStore_GetOrSet_SetNXError(t *testing.T) {
	wantErr := errors.New("setnx: connection refused")
	rdb := newFakeRedis()
	rdb.setNXFn = func(ctx context.Context, key string, value any, exp time.Duration) *redis.BoolCmd {
		cmd := redis.NewBoolCmd(ctx)
		cmd.SetErr(wantErr)
		return cmd
	}
	s := correlation.NewValkeyStoreFromClient(rdb)

	_, _, err := s.GetOrSet(context.Background(), "key1", "corr-x", time.Minute)
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}

func TestValkeyStore_GetOrSet_GetError(t *testing.T) {
	wantErr := errors.New("get: timeout")
	rdb := newFakeRedis()
	rdb.store["key1"] = fakeRedisEntry{value: "corr-existing", expiry: time.Now().Add(time.Minute)}
	rdb.getFn = func(ctx context.Context, key string) *redis.StringCmd {
		cmd := redis.NewStringCmd(ctx)
		cmd.SetErr(wantErr)
		return cmd
	}
	s := correlation.NewValkeyStoreFromClient(rdb)

	_, _, err := s.GetOrSet(context.Background(), "key1", "corr-x", time.Minute)
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}

func TestValkeyStore_GetOrSet_RaceRetry_SetNXWins(t *testing.T) {
	ctx := context.Background()
	calls := 0
	rdb := newFakeRedis()
	rdb.setNXFn = func(ctx context.Context, key string, value any, exp time.Duration) *redis.BoolCmd {
		calls++
		cmd := redis.NewBoolCmd(ctx)
		if calls == 1 {
			cmd.SetVal(false)
		} else {
			cmd.SetVal(true)
		}
		return cmd
	}
	rdb.getFn = func(ctx context.Context, key string) *redis.StringCmd {
		cmd := redis.NewStringCmd(ctx)
		cmd.SetErr(redis.Nil)
		return cmd
	}
	s := correlation.NewValkeyStoreFromClient(rdb)

	val, wasNew, err := s.GetOrSet(ctx, "key1", "corr-retry", time.Minute)
	require.NoError(t, err)
	assert.True(t, wasNew)
	assert.Equal(t, "corr-retry", val)
	assert.Equal(t, 2, calls, "should have called SetNX twice")
}

func TestValkeyStore_GetOrSet_RaceRetry_SetNXLoses(t *testing.T) {
	ctx := context.Background()
	getCalls := 0
	rdb := newFakeRedis()
	rdb.setNXFn = func(ctx context.Context, key string, value any, exp time.Duration) *redis.BoolCmd {
		cmd := redis.NewBoolCmd(ctx)
		cmd.SetVal(false)
		return cmd
	}
	rdb.getFn = func(ctx context.Context, key string) *redis.StringCmd {
		getCalls++
		cmd := redis.NewStringCmd(ctx)
		if getCalls == 1 {
			cmd.SetErr(redis.Nil)
		} else {
			cmd.SetVal("corr-winner")
		}
		return cmd
	}
	s := correlation.NewValkeyStoreFromClient(rdb)

	val, wasNew, err := s.GetOrSet(ctx, "key1", "corr-loser", time.Minute)
	require.NoError(t, err)
	assert.False(t, wasNew)
	assert.Equal(t, "corr-winner", val)
}

func TestValkeyStore_GetOrSet_RaceRetry_SecondSetNXError(t *testing.T) {
	wantErr := errors.New("second setnx failed")
	ctx := context.Background()
	setNXCalls := 0
	rdb := newFakeRedis()
	rdb.setNXFn = func(ctx context.Context, key string, value any, exp time.Duration) *redis.BoolCmd {
		setNXCalls++
		cmd := redis.NewBoolCmd(ctx)
		if setNXCalls == 1 {
			cmd.SetVal(false)
		} else {
			cmd.SetErr(wantErr)
		}
		return cmd
	}
	rdb.getFn = func(ctx context.Context, key string) *redis.StringCmd {
		cmd := redis.NewStringCmd(ctx)
		cmd.SetErr(redis.Nil)
		return cmd
	}
	s := correlation.NewValkeyStoreFromClient(rdb)

	_, _, err := s.GetOrSet(ctx, "key1", "corr-x", time.Minute)
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}

func TestValkeyStore_GetOrSet_RaceRetry_SecondGetError(t *testing.T) {
	wantErr := errors.New("second get failed")
	ctx := context.Background()
	getCalls := 0
	rdb := newFakeRedis()
	rdb.setNXFn = func(ctx context.Context, key string, value any, exp time.Duration) *redis.BoolCmd {
		cmd := redis.NewBoolCmd(ctx)
		cmd.SetVal(false)
		return cmd
	}
	rdb.getFn = func(ctx context.Context, key string) *redis.StringCmd {
		getCalls++
		cmd := redis.NewStringCmd(ctx)
		if getCalls == 1 {
			cmd.SetErr(redis.Nil)
		} else {
			cmd.SetErr(wantErr)
		}
		return cmd
	}
	s := correlation.NewValkeyStoreFromClient(rdb)

	_, _, err := s.GetOrSet(ctx, "key1", "corr-x", time.Minute)
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}
