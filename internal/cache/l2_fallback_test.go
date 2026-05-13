package cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValkeyL2_lookupByScan_ReturnsNearestTenantEntry(t *testing.T) {
	_, rdb := newMiniredis(t)
	l2 := NewValkeyL2(rdb, NewMemEmbedder(4))

	ctx := context.Background()
	queryVec := []float32{1, 0, 0, 0}
	wantKey := "llm:l1:match"
	require.NoError(t, rdb.HSet(ctx, l2EntryKey("tenant-a", wantKey),
		"embedding", float32SliceToBytes(queryVec),
		"l1_key", wantKey,
	).Err())
	require.NoError(t, rdb.HSet(ctx, l2EntryKey("tenant-b", "llm:l1:other"),
		"embedding", float32SliceToBytes(queryVec),
		"l1_key", "llm:l1:other",
	).Err())

	got, err := l2.lookupByScan(ctx, "tenant-a", queryVec)
	require.NoError(t, err)
	require.Equal(t, wantKey, got)
}

func TestBytesToFloat32Slice_RejectsInvalidLength(t *testing.T) {
	_, err := bytesToFloat32Slice([]byte{1, 2, 3})
	require.Error(t, err)
}
