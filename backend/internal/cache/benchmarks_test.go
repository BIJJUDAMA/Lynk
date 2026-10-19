package cache

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func BenchmarkShardedL1Cache_Get(b *testing.B) {
	ctx := context.Background()
	c := NewShardedL1Cache(64, time.Minute)
	key := "benchmark:project"
	val := []byte("serialized_project_card_json_payload_under_test")
	_ = c.Set(ctx, key, val, time.Minute)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _, _ = c.Get(ctx, key)
		}
	})
}

func BenchmarkShardedL1Cache_SetGetMixed(b *testing.B) {
	ctx := context.Background()
	c := NewShardedL1Cache(64, time.Minute)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			k := fmt.Sprintf("key:%d", i%100)
			if i%10 == 0 {
				_ = c.Set(ctx, k, []byte("data"), time.Minute)
			} else {
				_, _, _ = c.Get(ctx, k)
			}
			i++
		}
	})
}
