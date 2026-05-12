package dedup_test

import (
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"

	"github.com/paladinai/paladinai/services/paladin-ingest/internal/dedup"
)

func TestNewValkeyStore_ReturnsNonNil(t *testing.T) {
	// redis.NewClient creates a struct without dialing — safe in sandbox.
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer rdb.Close() //nolint:errcheck

	vs := dedup.NewValkeyStore(rdb)
	assert.NotNil(t, vs)
}
