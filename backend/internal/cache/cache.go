package cache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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

// InvalidationMessage represents a cache eviction event broadcast across nodes.
type InvalidationMessage struct {
	SenderID string   `json:"sender_id"`
	Action   string   `json:"action"` // "delete" or "pattern"
	Keys     []string `json:"keys,omitempty"`
	Pattern  string   `json:"pattern,omitempty"`
}

// InvalidationBus defines a pub/sub bus for distributed cache invalidation.
type InvalidationBus interface {
	Publish(ctx context.Context, msg InvalidationMessage) error
	Subscribe(ctx context.Context, handler func(msg InvalidationMessage)) error
	Close() error
}

// RedisInvalidationBus implements InvalidationBus backed by Redis Pub/Sub.
type RedisInvalidationBus struct {
	client  *redis.Client
	channel string
	pubsub  *redis.PubSub
	cancel  context.CancelFunc
	mu      sync.Mutex
	closed  bool
}

var _ InvalidationBus = (*RedisInvalidationBus)(nil)

// NewRedisInvalidationBus creates a new RedisInvalidationBus for the given channel.
func NewRedisInvalidationBus(client *redis.Client, channel string) *RedisInvalidationBus {
	if channel == "" {
		channel = "lynk:cache:invalidation"
	}
	return &RedisInvalidationBus{
		client:  client,
		channel: channel,
	}
}

// Publish serializes and publishes an InvalidationMessage to the Redis channel.
func (b *RedisInvalidationBus) Publish(ctx context.Context, msg InvalidationMessage) error {
	if b.client == nil {
		return errors.New("redis client not configured")
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return b.client.Publish(ctx, b.channel, payload).Err()
}

// Subscribe subscribes to the Redis channel and starts a background goroutine
// that invokes handler for each received InvalidationMessage.
func (b *RedisInvalidationBus) Subscribe(ctx context.Context, handler func(msg InvalidationMessage)) error {
	if b.client == nil {
		return errors.New("redis client not configured")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return errors.New("invalidation bus is closed")
	}
	if b.pubsub != nil {
		return errors.New("already subscribed")
	}

	subCtx, cancel := context.WithCancel(context.Background())
	pubsub := b.client.Subscribe(subCtx, b.channel)

	// Confirm subscription is active
	_, err := pubsub.Receive(ctx)
	if err != nil {
		cancel()
		_ = pubsub.Close()
		return err
	}

	b.pubsub = pubsub
	b.cancel = cancel

	ch := pubsub.Channel()
	go func() {
		for {
			select {
			case <-subCtx.Done():
				return
			case redisMsg, ok := <-ch:
				if !ok {
					return
				}
				var msg InvalidationMessage
				if err := json.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
					continue
				}
				handler(msg)
			}
		}
	}()

	return nil
}

// Close closes the Redis pubsub subscription and stops the receiver goroutine.
func (b *RedisInvalidationBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	if b.cancel != nil {
		b.cancel()
	}
	if b.pubsub != nil {
		return b.pubsub.Close()
	}
	return nil
}

// InMemoryInvalidationBus provides an in-memory pub/sub multiplexer for testing.
type InMemoryInvalidationBus struct {
	mu          sync.RWMutex
	subscribers []func(msg InvalidationMessage)
	closed      bool
}

// MockInvalidationBus is an alias for InMemoryInvalidationBus.
type MockInvalidationBus = InMemoryInvalidationBus

var _ InvalidationBus = (*InMemoryInvalidationBus)(nil)

// NewInMemoryInvalidationBus creates a new InMemoryInvalidationBus instance.
func NewInMemoryInvalidationBus() *InMemoryInvalidationBus {
	return &InMemoryInvalidationBus{
		subscribers: make([]func(msg InvalidationMessage), 0),
	}
}

// NewMockInvalidationBus creates a new MockInvalidationBus instance.
func NewMockInvalidationBus() *MockInvalidationBus {
	return NewInMemoryInvalidationBus()
}

// Publish broadcasts msg to all registered subscribers.
func (b *InMemoryInvalidationBus) Publish(ctx context.Context, msg InvalidationMessage) error {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return errors.New("invalidation bus is closed")
	}
	subs := make([]func(msg InvalidationMessage), len(b.subscribers))
	copy(subs, b.subscribers)
	b.mu.RUnlock()

	for _, handler := range subs {
		handler(msg)
	}
	return nil
}

// Subscribe registers a new subscriber callback.
func (b *InMemoryInvalidationBus) Subscribe(ctx context.Context, handler func(msg InvalidationMessage)) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return errors.New("invalidation bus is closed")
	}
	b.subscribers = append(b.subscribers, handler)
	return nil
}

// Close closes the in-memory bus and releases subscribers.
func (b *InMemoryInvalidationBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	b.subscribers = nil
	return nil
}

func generateNodeID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("node-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// TwoTierCache provides a transparent two-tier cache architecture
// combining an in-memory L1 cache with a remote L2 cache (such as Redis).
type TwoTierCache struct {
	l1     Cacher
	l2     Cacher
	l1TTL  time.Duration
	nodeID string
	bus    InvalidationBus
}

var _ Cacher = (*TwoTierCache)(nil)

// NewTwoTierCache constructs a new TwoTierCache.
// If l1TTL <= 0, it defaults to 10 seconds.
func NewTwoTierCache(l1 Cacher, l2 Cacher, l1TTL time.Duration) *TwoTierCache {
	if l1TTL <= 0 {
		l1TTL = 10 * time.Second
	}
	return &TwoTierCache{
		l1:     l1,
		l2:     l2,
		l1TTL:  l1TTL,
		nodeID: generateNodeID(),
	}
}

// NodeID returns the unique node identifier for this cache instance.
func (c *TwoTierCache) NodeID() string {
	return c.nodeID
}

// Bus returns the configured InvalidationBus, or nil if none is set.
func (c *TwoTierCache) Bus() InvalidationBus {
	return c.bus
}

// SetInvalidationBus registers a distributed invalidation bus and subscribes
// to remote eviction events. An optional nodeID can be passed; if omitted, a
// random hex identifier is generated.
func (c *TwoTierCache) SetInvalidationBus(bus InvalidationBus, nodeID ...string) error {
	c.bus = bus
	if len(nodeID) > 0 && nodeID[0] != "" {
		c.nodeID = nodeID[0]
	} else if c.nodeID == "" {
		c.nodeID = generateNodeID()
	}
	if c.bus == nil {
		return nil
	}
	return c.bus.Subscribe(context.Background(), func(msg InvalidationMessage) {
		if msg.SenderID == c.nodeID {
			return
		}
		if c.l1 == nil {
			return
		}
		switch msg.Action {
		case "delete":
			_ = c.l1.Delete(context.Background(), msg.Keys...)
		case "pattern":
			_ = c.l1.DeletePattern(context.Background(), msg.Pattern)
		}
	})
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

// Delete removes one or more keys from both L1 and L2, then broadcasts an invalidation.
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
	if c.bus != nil && len(keys) > 0 {
		if err := c.bus.Publish(ctx, InvalidationMessage{
			SenderID: c.nodeID,
			Action:   "delete",
			Keys:     keys,
		}); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// DeletePattern purges keys matching pattern from both L1 and L2, then broadcasts an invalidation.
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
	if c.bus != nil {
		if err := c.bus.Publish(ctx, InvalidationMessage{
			SenderID: c.nodeID,
			Action:   "pattern",
			Pattern:  pattern,
		}); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
