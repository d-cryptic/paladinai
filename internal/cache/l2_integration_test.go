//go:build integration

// Run with: go test -tags integration -run TestValkeyL2 ./internal/cache/...
// Requires Valkey with the Search module running at VALKEY_URL (default: localhost:6379).
package cache

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func newIntegrationRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("VALKEY_URL")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr, Protocol: 3, UnstableResp3: true})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("skipping integration test: Valkey not reachable at %s: %v", addr, err)
	}
	return rdb
}

func TestValkeyL2_StoreAndLookup(t *testing.T) {
	rdb := newIntegrationRedis(t)
	ctx := context.Background()

	emb := NewMemEmbedder(4)
	l2 := NewValkeyL2(rdb, emb)

	// Use a unique tenant to avoid collisions with other test runs.
	tenant := fmt.Sprintf("integ-test-%d", time.Now().UnixNano())

	const query = "OOMKilled pod redis-cache-0"
	const l1Key = "llm:l1:integtest0001"

	// Store.
	if err := l2.Store(ctx, tenant, query, l1Key); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// Exact same query should hit.
	waitForL2Hit(t, l2, tenant, query, l1Key)

	// Cleanup.
	rdb.Del(ctx, l2EntryKey(tenant, l1Key))          //nolint:errcheck
	rdb.Do(ctx, "FT.DROPINDEX", l2IndexName(tenant)) //nolint:errcheck
}

func waitForL2Hit(t *testing.T, l2 *ValkeyL2, tenantID, queryText, wantL1Key string) {
	t.Helper()

	ctx := context.Background()
	deadline := time.Now().Add(3 * time.Second)
	for {
		got, err := l2.Lookup(ctx, tenantID, queryText)
		if err != nil {
			t.Fatalf("Lookup: %v", err)
		}
		if got == wantL1Key {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected %q, got %q", wantL1Key, got)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestValkeyL2_MissOnEmptyIndex(t *testing.T) {
	rdb := newIntegrationRedis(t)
	ctx := context.Background()

	emb := NewMemEmbedder(4)
	l2 := NewValkeyL2(rdb, emb)

	tenant := fmt.Sprintf("integ-empty-%d", time.Now().UnixNano())

	got, err := l2.Lookup(ctx, tenant, "some query with no stored entries")
	if err != nil {
		t.Fatalf("Lookup on empty index: %v", err)
	}
	if got != "" {
		t.Errorf("empty index should return miss, got %q", got)
	}

	rdb.Do(ctx, "FT.DROPINDEX", l2IndexName(tenant)) //nolint:errcheck
}

func TestValkeyL2_TenantIsolation(t *testing.T) {
	rdb := newIntegrationRedis(t)
	ctx := context.Background()

	emb := NewMemEmbedder(4)
	l2 := NewValkeyL2(rdb, emb)

	ts := time.Now().UnixNano()
	tenantA := fmt.Sprintf("integ-tenant-a-%d", ts)
	tenantB := fmt.Sprintf("integ-tenant-b-%d", ts)

	const query = "high disk I/O on storage-node-1"
	const l1Key = "llm:l1:disk-io-key"

	if err := l2.Store(ctx, tenantA, query, l1Key); err != nil {
		t.Fatalf("Store: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	got, err := l2.Lookup(ctx, tenantB, query)
	if err != nil {
		t.Fatalf("Lookup tenant B: %v", err)
	}
	if got != "" {
		t.Errorf("tenant B should not see tenant A entry, got %q", got)
	}

	rdb.Del(ctx, l2EntryKey(tenantA, l1Key))          //nolint:errcheck
	rdb.Do(ctx, "FT.DROPINDEX", l2IndexName(tenantA)) //nolint:errcheck
}
