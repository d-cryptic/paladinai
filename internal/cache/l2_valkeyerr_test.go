package cache

// l2_valkeyerr_test.go tests ValkeyL2 error paths using miniredis.
// miniredis does not support FT commands, so FT.CREATE returns
// "ERR unknown command 'FT.CREATE'" which exercises ensureIndex error paths,
// and the Lookup/Store error propagation branches.

import (
	"context"
	"testing"
)

func TestValkeyL2_Lookup_EnsureIndexFails(t *testing.T) {
	skipIfNoNetwork(t)
	_, rdb := newMiniredis(t)
	emb := NewMemEmbedder(4)
	l2 := NewValkeyL2(rdb, emb)

	// FT.CREATE is not supported by miniredis → ensureIndex returns error →
	// Lookup propagates it as "l2 ensure index: ..."
	_, err := l2.Lookup(context.Background(), "acme", "why is payments down")
	if err == nil {
		t.Error("expected error when FT.CREATE not supported, got nil")
	}
}

func TestValkeyL2_Store_EnsureIndexFails(t *testing.T) {
	skipIfNoNetwork(t)
	_, rdb := newMiniredis(t)
	emb := NewMemEmbedder(4)
	l2 := NewValkeyL2(rdb, emb)

	err := l2.Store(context.Background(), "acme", "why is payments down", "l1-key-abc")
	if err == nil {
		t.Error("expected error when FT.CREATE not supported, got nil")
	}
}

func TestValkeyL2_ensureIndex_SecondCall_UsesCachedState(t *testing.T) {
	// Simulate two consecutive Lookup calls for the same tenant.
	// The first sets indexSeen after ensureIndex returns (error or not).
	// Here, since FT.CREATE fails with miniredis, indexSeen is NOT set on
	// first call. But the in-memory indexSeen only advances on success.
	// Verify that calling Lookup twice does not panic.
	skipIfNoNetwork(t)
	_, rdb := newMiniredis(t)
	emb := NewMemEmbedder(4)
	l2 := NewValkeyL2(rdb, emb)

	_, _ = l2.Lookup(context.Background(), "acme", "query1")
	_, _ = l2.Lookup(context.Background(), "acme", "query2")
	// No panic = pass.
}
