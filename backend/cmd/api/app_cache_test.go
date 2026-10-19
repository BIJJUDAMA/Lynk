package main

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/lynk/backend/internal/cache"
	"github.com/redis/go-redis/v9"
)

func TestBuildAppCache_WithRedisUsesShardedL1(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	c, closeFn := buildAppCache(client)
	defer closeFn()

	ttc, ok := c.(*cache.TwoTierCache)
	if !ok {
		t.Fatalf("expected *cache.TwoTierCache, got %T", c)
	}
	if _, ok := ttc.L1().(*cache.ShardedL1Cacher); !ok {
		t.Fatalf("expected *cache.ShardedL1Cacher L1, got %T", ttc.L1())
	}
}

func TestBuildAppCache_WithoutRedisFallsBackToMemory(t *testing.T) {
	c, closeFn := buildAppCache(nil)
	defer closeFn()
	if _, ok := c.(*cache.MemoryCache); !ok {
		t.Fatalf("expected *cache.MemoryCache, got %T", c)
	}
}
