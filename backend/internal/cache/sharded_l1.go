package cache

import (
	"context"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cespare/xxhash/v2"
)

const l1SweepInterval = 256

type l1Item struct {
	value     []byte
	expiresAt time.Time
	hits      atomic.Uint64
}

type l1Shard struct {
	mu     sync.RWMutex
	items  map[string]*l1Item
	writes uint64
}

func (s *l1Shard) sweepExpiredLocked(now time.Time) {
	for key, item := range s.items {
		if now.After(item.expiresAt) {
			delete(s.items, key)
		}
	}
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
	// Bound the shard count so the int->uint64 conversion below is provably safe
	// (gosec G115) and the shard slice cannot be made unboundedly large.
	const maxShards = 1 << 16
	if numShards > maxShards {
		numShards = maxShards
	}
	shards := make([]*l1Shard, numShards)
	for i := 0; i < numShards; i++ {
		shards[i] = &l1Shard{
			items: make(map[string]*l1Item),
		}
	}
	return &ShardedL1Cache{
		shards:    shards,
		numShards: uint64(numShards), //nolint:gosec // G115: numShards is bounded to [1, 65536] above
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
	shard.mu.RLock()

	item, exists := shard.items[key]
	if !exists {
		shard.mu.RUnlock()
		return nil, false, nil
	}

	if time.Now().After(item.expiresAt) {
		shard.mu.RUnlock()
		shard.mu.Lock()
		if current, ok := shard.items[key]; ok && time.Now().After(current.expiresAt) {
			delete(shard.items, key)
		}
		shard.mu.Unlock()
		return nil, false, nil
	}

	item.hits.Add(1)
	cpy := make([]byte, len(item.value))
	copy(cpy, item.value)
	shard.mu.RUnlock()
	return cpy, true, nil
}

// Set saves item into appropriate shard.
func (c *ShardedL1Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = c.ttl
	}

	valCopy := make([]byte, len(value))
	copy(valCopy, value)

	item := &l1Item{
		value:     valCopy,
		expiresAt: time.Now().Add(ttl),
	}
	item.hits.Store(1)

	shard := c.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	shard.items[key] = item
	shard.writes++
	if shard.writes%l1SweepInterval == 0 {
		shard.sweepExpiredLocked(time.Now())
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

// DeletePattern removes every key matching the glob-style pattern,
// mirroring MemoryCache.DeletePattern semantics: "*" deletes all, a trailing
// "*" is a prefix match, otherwise path.Match is used, and exact equality
// also matches. Each shard is scanned under its own lock.
func (c *ShardedL1Cache) DeletePattern(ctx context.Context, pattern string) error {
	prefixMatch := strings.HasSuffix(pattern, "*")
	prefix := strings.TrimSuffix(pattern, "*")

	for _, shard := range c.shards {
		shard.mu.Lock()
		for k := range shard.items {
			if pattern == "*" || (prefixMatch && strings.HasPrefix(k, prefix)) {
				delete(shard.items, k)
				continue
			}
			if matched, _ := path.Match(pattern, k); matched {
				delete(shard.items, k)
				continue
			}
			if pattern == k {
				delete(shard.items, k)
			}
		}
		shard.mu.Unlock()
	}
	return nil
}

// Frequency returns number of cache hits on key for hot-key detection.
func (c *ShardedL1Cache) Frequency(key string) uint64 {
	shard := c.getShard(key)
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	if item, ok := shard.items[key]; ok {
		return item.hits.Load()
	}
	return 0
}
