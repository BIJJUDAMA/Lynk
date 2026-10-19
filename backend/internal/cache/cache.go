package cache

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cacher defines standard cache operations for key-value storage.
type Cacher interface {
	Get(ctx context.Context, key string, dest any) (bool, error)
	Set(ctx context.Context, key string, val any, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	DeletePattern(ctx context.Context, pattern string) error
}

type memoryEntry struct {
	data      []byte
	expiresAt time.Time
}

// MemoryCache provides a thread-safe in-memory cache implementation.
type MemoryCache struct {
	mu    sync.RWMutex
	items map[string]memoryEntry
}

// NewMemoryCache creates a new in-memory cache instance.
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		items: make(map[string]memoryEntry),
	}
}

// Get retrieves a cached value by key and unmarshals it into dest.
// Returns (false, nil) if the key does not exist or has expired.
func (m *MemoryCache) Get(ctx context.Context, key string, dest any) (bool, error) {
	m.mu.RLock()
	entry, exists := m.items[key]
	m.mu.RUnlock()

	if !exists {
		return false, nil
	}

	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		m.mu.Lock()
		if e, ok := m.items[key]; ok && !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
			delete(m.items, key)
		}
		m.mu.Unlock()
		return false, nil
	}

	if err := json.Unmarshal(entry.data, dest); err != nil {
		return false, err
	}
	return true, nil
}

// Set marshals val into JSON and stores it with the provided TTL.
// A TTL <= 0 means no expiration.
func (m *MemoryCache) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	m.mu.Lock()
	m.items[key] = memoryEntry{
		data:      data,
		expiresAt: expiresAt,
	}
	m.mu.Unlock()
	return nil
}

// Delete removes one or more keys from the cache.
func (m *MemoryCache) Delete(ctx context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range keys {
		delete(m.items, k)
	}
	return nil
}

// DeletePattern removes all keys matching the given glob-style pattern (e.g. "jobs:search:*").
func (m *MemoryCache) DeletePattern(ctx context.Context, pattern string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	prefixMatch := strings.HasSuffix(pattern, "*")
	prefix := strings.TrimSuffix(pattern, "*")

	for k := range m.items {
		if pattern == "*" || (prefixMatch && strings.HasPrefix(k, prefix)) {
			delete(m.items, k)
			continue
		}
		if matched, _ := path.Match(pattern, k); matched {
			delete(m.items, k)
			continue
		}
		if pattern == k {
			delete(m.items, k)
		}
	}
	return nil
}

// RedisCache provides a Redis-backed implementation of Cacher.
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new RedisCache wrapping the given redis.Client.
func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

// Get retrieves a key from Redis and unmarshals its JSON into dest.
func (r *RedisCache) Get(ctx context.Context, key string, dest any) (bool, error) {
	if r.client == nil {
		return false, errors.New("redis client not configured")
	}

	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, err
	}

	if err := json.Unmarshal(data, dest); err != nil {
		return false, err
	}
	return true, nil
}

// Set marshals val into JSON and saves it in Redis with the given TTL.
func (r *RedisCache) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	if r.client == nil {
		return errors.New("redis client not configured")
	}

	data, err := json.Marshal(val)
	if err != nil {
		return err
	}

	return r.client.Set(ctx, key, data, ttl).Err()
}

// Delete deletes one or more keys from Redis.
func (r *RedisCache) Delete(ctx context.Context, keys ...string) error {
	if r.client == nil {
		return errors.New("redis client not configured")
	}
	if len(keys) == 0 {
		return nil
	}
	return r.client.Del(ctx, keys...).Err()
}

// DeletePattern scans Redis for keys matching pattern and deletes them in batches.
func (r *RedisCache) DeletePattern(ctx context.Context, pattern string) error {
	if r.client == nil {
		return errors.New("redis client not configured")
	}

	iter := r.client.Scan(ctx, 0, pattern, 100).Iterator()
	var toDelete []string
	for iter.Next(ctx) {
		toDelete = append(toDelete, iter.Val())
		if len(toDelete) >= 500 {
			if err := r.client.Del(ctx, toDelete...).Err(); err != nil {
				return err
			}
			toDelete = toDelete[:0]
		}
	}
	if err := iter.Err(); err != nil {
		return err
	}
	if len(toDelete) > 0 {
		return r.client.Del(ctx, toDelete...).Err()
	}
	return nil
}

// New initializes a Cacher. If redisURL is valid and connects within a short timeout,
// it returns a RedisCache. Otherwise, it logs a warning and falls back to MemoryCache.
func New(redisURL string) Cacher {
	trimmed := strings.TrimSpace(redisURL)
	if trimmed == "" {
		return NewMemoryCache()
	}

	opt, err := redis.ParseURL(trimmed)
	if err != nil {
		slog.Warn("invalid redis url, falling back to memory cache", "error", err)
		return NewMemoryCache()
	}

	opt.MaxRetries = 0
	opt.DialTimeout = 200 * time.Millisecond
	opt.ReadTimeout = 200 * time.Millisecond
	opt.WriteTimeout = 200 * time.Millisecond

	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		slog.Warn("cannot ping redis, falling back to memory cache", "error", err)
		_ = client.Close()
		return NewMemoryCache()
	}

	slog.Info("connected to redis cache successfully")
	return NewRedisCache(client)
}

// TwoTierCache provides a transparent two-tier cache architecture
// combining an in-memory L1 cache with a remote L2 cache (such as Redis).
type TwoTierCache struct {
	l1    Cacher
	l2    Cacher
	l1TTL time.Duration
}

var _ Cacher = (*TwoTierCache)(nil)

// NewTwoTierCache constructs a new TwoTierCache.
// If l1TTL <= 0, it defaults to 10 seconds.
func NewTwoTierCache(l1 Cacher, l2 Cacher, l1TTL time.Duration) *TwoTierCache {
	if l1TTL <= 0 {
		l1TTL = 10 * time.Second
	}
	return &TwoTierCache{
		l1:    l1,
		l2:    l2,
		l1TTL: l1TTL,
	}
}

// L1 returns the underlying L1 cache.
func (c *TwoTierCache) L1() Cacher {
	return c.l1
}

// L2 returns the underlying L2 cache.
func (c *TwoTierCache) L2() Cacher {
	return c.l2
}

// Get checks L1 first. If hit, returns (true, nil).
// If miss, checks L2. If L2 hits, backfills L1 with l1TTL and returns (true, nil).
// If miss in both, returns (false, nil).
func (c *TwoTierCache) Get(ctx context.Context, key string, dest any) (bool, error) {
	if c.l1 != nil {
		hit, err := c.l1.Get(ctx, key, dest)
		if err == nil && hit {
			return true, nil
		}
	}

	if c.l2 != nil {
		hit, err := c.l2.Get(ctx, key, dest)
		if err != nil {
			return false, err
		}
		if !hit {
			return false, nil
		}
		if c.l1 != nil {
			_ = c.l1.Set(ctx, key, dest, c.l1TTL)
		}
		return true, nil
	}

	return false, nil
}

// Set writes to L1 with min(ttl, l1TTL) and to L2 with full ttl.
func (c *TwoTierCache) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	effectiveL1TTL := c.l1TTL
	if ttl > 0 && ttl < effectiveL1TTL {
		effectiveL1TTL = ttl
	}

	var errs []error
	if c.l1 != nil {
		if err := c.l1.Set(ctx, key, val, effectiveL1TTL); err != nil {
			errs = append(errs, err)
		}
	}
	if c.l2 != nil {
		if err := c.l2.Set(ctx, key, val, ttl); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Delete removes one or more keys from both L1 and L2.
func (c *TwoTierCache) Delete(ctx context.Context, keys ...string) error {
	var errs []error
	if c.l1 != nil {
		if err := c.l1.Delete(ctx, keys...); err != nil {
			errs = append(errs, err)
		}
	}
	if c.l2 != nil {
		if err := c.l2.Delete(ctx, keys...); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// DeletePattern purges keys matching pattern from both L1 and L2.
func (c *TwoTierCache) DeletePattern(ctx context.Context, pattern string) error {
	var errs []error
	if c.l1 != nil {
		if err := c.l1.DeletePattern(ctx, pattern); err != nil {
			errs = append(errs, err)
		}
	}
	if c.l2 != nil {
		if err := c.l2.DeletePattern(ctx, pattern); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

