package cache

import (
	"context"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
)

type l1Item struct {
	value     []byte
	expiresAt time.Time
	hits      uint64
}

type l1Shard struct {
	mu    sync.RWMutex
	items map[string]l1Item
}

// ShardedL1Cache splits cache keys across N shards to minimize mutex contention under 1M RPS.
type ShardedL1Cache struct {
	shards    []*l1Shard
	numShards uint64
	ttl       time.Duration
}

// NewShardedL1Cache allocates a sharded in-memory cache with specified shard count.
func NewShardedL1Cache(numShards int, defaultTTL time.Duration) *ShardedL1Cache {
	if numShards <= 0 {
		numShards = 32
	}
	shards := make([]*l1Shard, numShards)
	for i := 0; i < numShards; i++ {
		shards[i] = &l1Shard{
			items: make(map[string]l1Item),
		}
	}
	return &ShardedL1Cache{
		shards:    shards,
		numShards: uint64(numShards),
		ttl:       defaultTTL,
	}
}

func (c *ShardedL1Cache) getShard(key string) *l1Shard {
	h := xxhash.Sum64String(key)
	return c.shards[h%c.numShards]
}

// Get retrieves cached item and increments access frequency.
func (c *ShardedL1Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	shard := c.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	item, exists := shard.items[key]
	if !exists {
		return nil, false, nil
	}

	if time.Now().After(item.expiresAt) {
		delete(shard.items, key)
		return nil, false, nil
	}

	item.hits++
	shard.items[key] = item

	cpy := make([]byte, len(item.value))
	copy(cpy, item.value)
	return cpy, true, nil
}

// Set saves item into appropriate shard.
func (c *ShardedL1Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = c.ttl
	}
	shard := c.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	valCopy := make([]byte, len(value))
	copy(valCopy, value)

	shard.items[key] = l1Item{
		value:     valCopy,
		expiresAt: time.Now().Add(ttl),
		hits:      1,
	}
	return nil
}

// Delete removes item from cache.
func (c *ShardedL1Cache) Delete(ctx context.Context, key string) error {
	shard := c.getShard(key)
	shard.mu.Lock()
	delete(shard.items, key)
	shard.mu.Unlock()
	return nil
}

// Frequency returns number of cache hits on key for hot-key detection.
func (c *ShardedL1Cache) Frequency(key string) uint64 {
	shard := c.getShard(key)
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	if item, ok := shard.items[key]; ok {
		return item.hits
	}
	return 0
}
