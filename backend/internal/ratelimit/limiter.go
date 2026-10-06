package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// Store defines the interface for rate limiting storage.
type Store interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Duration, error)
}

// MemoryStore provides a thread-safe in-memory sliding window rate limiter.
type MemoryStore struct {
	mu      sync.Mutex
	entries map[string][]time.Time
}

// NewMemoryStore initializes a new in-memory rate limiter store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		entries: make(map[string][]time.Time),
	}
}

// Allow evaluates whether a key has exceeded the limit in the given window.
func (m *MemoryStore) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-window)

	timestamps := m.entries[key]
	valid := make([]time.Time, 0, len(timestamps))
	for _, t := range timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) < limit {
		valid = append(valid, now)
		m.entries[key] = valid
		remaining := limit - len(valid)
		resetAfter := window
		if len(valid) > 0 {
			oldest := valid[0]
			resetAfter = window - now.Sub(oldest)
			if resetAfter < 0 {
				resetAfter = 0
			}
		}
		return true, remaining, resetAfter, nil
	}

	if len(valid) == 0 {
		delete(m.entries, key)
	} else {
		m.entries[key] = valid
	}
	resetAfter := window
	if len(valid) > 0 {
		oldest := valid[0]
		resetAfter = window - now.Sub(oldest)
		if resetAfter < 0 {
			resetAfter = 0
		}
	}
	return false, 0, resetAfter, nil
}

// Redis Lua script implementing sliding window rate limiter with ZSET.
var slidingWindowScript = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local windowMs = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]

local clearBefore = now - windowMs
redis.call('ZREMRANGEBYSCORE', key, '-inf', clearBefore)

local count = redis.call('ZCARD', key)
if count < limit then
    redis.call('ZADD', key, now, member)
    redis.call('PEXPIRE', key, windowMs)
    local remaining = limit - count - 1
    return {1, remaining, windowMs}
else
    local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
    local resetAfterMs = windowMs
    if oldest and #oldest >= 2 then
        local oldestScore = tonumber(oldest[2])
        resetAfterMs = (oldestScore + windowMs) - now
    end
    if resetAfterMs < 0 then
        resetAfterMs = 0
    end
    return {0, 0, resetAfterMs}
end
`)

// RedisStore implements Store backed by Redis with automatic fallback to MemoryStore.
type RedisStore struct {
	client    *redis.Client
	fallback  *MemoryStore
	isDown    atomic.Bool
	lastCheck atomic.Int64
}

// NewRedisStore creates a new RedisStore.
func NewRedisStore(client *redis.Client) *RedisStore {
	store := &RedisStore{
		client:   client,
		fallback: NewMemoryStore(),
	}
	if client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		if err := client.Ping(ctx).Err(); err != nil {
			store.isDown.Store(true)
			store.lastCheck.Store(time.Now().Unix())
		}
	}
	return store
}

// NewRedisStoreFromURL creates a new RedisStore from a redis connection URL.
func NewRedisStoreFromURL(redisURL string) (*RedisStore, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	opt.MaxRetries = 0
	opt.DialTimeout = 200 * time.Millisecond
	opt.ReadTimeout = 200 * time.Millisecond
	opt.WriteTimeout = 200 * time.Millisecond
	client := redis.NewClient(opt)
	return NewRedisStore(client), nil
}

// Allow executes sliding window rate limiting via Redis ZSET, falling back gracefully on error.
func (r *RedisStore) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Duration, error) {
	if r.client == nil {
		if r.fallback != nil {
			return r.fallback.Allow(ctx, key, limit, window)
		}
		return true, limit, window, nil
	}

	if r.isDown.Load() {
		nowSec := time.Now().Unix()
		if nowSec-r.lastCheck.Load() > 10 {
			pingCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			if err := r.client.Ping(pingCtx).Err(); err == nil {
				r.isDown.Store(false)
			} else {
				r.lastCheck.Store(nowSec)
			}
		}
		if r.isDown.Load() {
			if r.fallback != nil {
				return r.fallback.Allow(ctx, key, limit, window)
			}
			return true, limit, window, nil
		}
	}

	now := time.Now()
	nowMs := now.UnixMilli()
	windowMs := window.Milliseconds()
	member := fmt.Sprintf("%d-%d", now.UnixNano(), now.Nanosecond())
	redisKey := fmt.Sprintf("ratelimit:%s", key)

	res, err := slidingWindowScript.Run(ctx, r.client, []string{redisKey}, nowMs, windowMs, limit, member).Slice()
	if err != nil {
		r.isDown.Store(true)
		r.lastCheck.Store(time.Now().Unix())
		slog.Warn("redis rate limiter error, falling back to in-memory store", "error", err, "key", key)
		if r.fallback != nil {
			return r.fallback.Allow(ctx, key, limit, window)
		}
		return true, limit, window, nil
	}

	if len(res) < 3 {
		return true, limit, window, nil
	}

	allowedInt, _ := res[0].(int64)
	remainingInt, _ := res[1].(int64)
	resetAfterMs, _ := res[2].(int64)

	allowed := allowedInt == 1
	remaining := int(remainingInt)
	resetAfter := time.Duration(resetAfterMs) * time.Millisecond

	return allowed, remaining, resetAfter, nil
}

func cleanIP(ipStr string) string {
	ipStr = strings.TrimSpace(ipStr)
	if host, _, err := net.SplitHostPort(ipStr); err == nil {
		ipStr = host
	}
	ipStr = strings.Trim(ipStr, "[]")
	return ipStr
}

// ExtractClientIP extracts the client IP strictly from RemoteAddr to prevent unvalidated X-Forwarded-For spoofing.
func ExtractClientIP(r *http.Request) string {
	remote := cleanIP(r.RemoteAddr)
	if remote == "" {
		return "127.0.0.1"
	}

	return remote
}

// NewSlidingWindowMiddleware constructs HTTP middleware for rate limiting using the given store.
func NewSlidingWindowMiddleware(store Store, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if path == "/health" || path == "/api/v1/health" {
				next.ServeHTTP(w, r)
				return
			}

			ip := ExtractClientIP(r)
			allowed, remaining, resetAfter, err := store.Allow(r.Context(), ip, limit, window)
			if err != nil {
				slog.Error("rate limiter allow check failed", "error", err, "ip", ip)
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			if !allowed {
				retrySeconds := int(math.Ceil(resetAfter.Seconds()))
				if retrySeconds < 1 {
					retrySeconds = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(retrySeconds))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error":   "too_many_requests",
					"message": "Rate limit exceeded. Please wait before retrying.",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// NewMiddlewareWithStore is an alias for NewSlidingWindowMiddleware.
func NewMiddlewareWithStore(store Store, limit int, window time.Duration) func(http.Handler) http.Handler {
	return NewSlidingWindowMiddleware(store, limit, window)
}
