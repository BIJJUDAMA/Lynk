package cache

import (
	"context"
	"encoding/json"
	"time"
)

// ShardedL1Cacher adapts the raw byte-oriented ShardedL1Cache to the Cacher
// interface so the sharded in-memory cache can be used as an L1 tier.
type ShardedL1Cacher struct {
	inner *ShardedL1Cache
}

var _ Cacher = (*ShardedL1Cacher)(nil)

// NewShardedL1Cacher wraps a ShardedL1Cache so it satisfies Cacher.
func NewShardedL1Cacher(inner *ShardedL1Cache) *ShardedL1Cacher {
	return &ShardedL1Cacher{inner: inner}
}

// Get retrieves key and JSON-unmarshals the stored bytes into dest.
func (c *ShardedL1Cacher) Get(ctx context.Context, key string, dest any) (bool, error) {
	raw, ok, err := c.inner.Get(ctx, key)
	if err != nil || !ok {
		return ok, err
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return false, err
	}
	return true, nil
}

// Set JSON-marshals val and stores it in the sharded cache with the given TTL.
func (c *ShardedL1Cacher) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	raw, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return c.inner.Set(ctx, key, raw, ttl)
}

// Delete removes every provided key from the sharded cache.
func (c *ShardedL1Cacher) Delete(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		if err := c.inner.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

// DeletePattern delegates to the inner sharded cache's pattern deletion.
func (c *ShardedL1Cacher) DeletePattern(ctx context.Context, pattern string) error {
	return c.inner.DeletePattern(ctx, pattern)
}
