package dedup

import (
	"github.com/redis/go-redis/v9"

	shared "github.com/paladinai/paladinai/internal/dedup"
)

// ValkeyStore adapts go-redis Client to the Store interface.
type ValkeyStore = shared.ValkeyStore

// NewValkeyStore wraps a go-redis Client.
func NewValkeyStore(rdb *redis.Client) *ValkeyStore { return shared.NewValkeyStore(rdb) }
