package main

import (
	"log/slog"
	"time"

	"github.com/lynk/backend/internal/cache"
	"github.com/redis/go-redis/v9"
)

// buildAppCache returns the application cache and a cleanup function.
// With Redis it returns a two-tier cache whose L1 is the sharded L1 cache;
// without Redis it returns the existing MemoryCache fallback.
func buildAppCache(redisClient *redis.Client) (cache.Cacher, func()) {
	if redisClient == nil {
		return cache.NewMemoryCache(), func() {}
	}
	logger := slog.Default()
	l1 := cache.NewShardedL1Cacher(cache.NewShardedL1Cache(64, 10*time.Second))
	l2 := cache.NewRedisCache(redisClient)
	ttc := cache.NewTwoTierCache(l1, l2, 10*time.Second)
	bus := cache.NewRedisInvalidationBus(redisClient, "lynk:cache:invalidation")
	if err := ttc.SetInvalidationBus(bus); err != nil {
		logger.Warn("failed to subscribe to cache invalidation bus", "error", err)
	}
	return ttc, func() {
		if err := bus.Close(); err != nil {
			logger.Warn("failed to close cache invalidation bus", "error", err)
		}
	}
}
