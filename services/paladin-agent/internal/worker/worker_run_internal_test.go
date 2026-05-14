package worker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAcquireSlotSucceedsWhenCapacityAvailable(t *testing.T) {
	sem := make(chan struct{}, 1)

	require.True(t, acquireSlot(context.Background(), sem))
	require.Len(t, sem, 1)
}

func TestAcquireSlotReturnsOnCanceledContextWhenFull(t *testing.T) {
	sem := make(chan struct{}, 1)
	sem <- struct{}{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	started := time.Now()
	require.False(t, acquireSlot(ctx, sem))
	require.Less(t, time.Since(started), 100*time.Millisecond)
	require.Len(t, sem, 1)
}
