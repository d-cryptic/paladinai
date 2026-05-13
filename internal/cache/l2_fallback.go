package cache

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
)

const l2FallbackScanLimit = 1000

func (c *ValkeyL2) lookupByScan(ctx context.Context, tenantID string, queryVec []float32) (string, error) {
	var (
		cursor uint64
		seen   int
		best   string
		score  float64
	)

	for {
		keys, next, err := c.rdb.Scan(ctx, cursor, l2EntryPrefix(tenantID)+"*", 100).Result()
		if err != nil {
			return "", fmt.Errorf("l2 fallback scan: %w", err)
		}
		cursor = next

		for _, key := range keys {
			if seen >= l2FallbackScanLimit {
				return best, nil
			}
			seen++

			values, err := c.rdb.HMGet(ctx, key, "embedding", "l1_key").Result()
			if err != nil {
				return "", fmt.Errorf("l2 fallback hget %s: %w", key, err)
			}
			if len(values) != 2 || values[0] == nil || values[1] == nil {
				continue
			}

			vecBytes, ok := redisBytes(values[0])
			if !ok {
				continue
			}
			vec, err := bytesToFloat32Slice(vecBytes)
			if err != nil {
				continue
			}

			l1Key, ok := redisString(values[1])
			if !ok || l1Key == "" {
				continue
			}

			similarity := cosineSimilarity(queryVec, vec)
			if similarity > score {
				score = similarity
				best = l1Key
			}
		}

		if cursor == 0 {
			break
		}
	}

	if best == "" || 1.0-score > l2MaxDistance {
		return "", nil
	}
	return best, nil
}

func bytesToFloat32Slice(b []byte) ([]float32, error) {
	if len(b)%4 != 0 {
		return nil, fmt.Errorf("invalid float32 vector byte length %d", len(b))
	}
	vec := make([]float32, len(b)/4)
	for i := range vec {
		vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return vec, nil
}

func redisBytes(value any) ([]byte, bool) {
	switch v := value.(type) {
	case []byte:
		return v, true
	case string:
		return []byte(v), true
	default:
		return nil, false
	}
}

func redisString(value any) (string, bool) {
	switch v := value.(type) {
	case []byte:
		return string(v), true
	case string:
		return v, true
	default:
		return "", false
	}
}
