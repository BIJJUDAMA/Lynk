package cache

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestShardedL1Cache_BasicOps(t *testing.T) {
	ctx := context.Background()
	c := NewShardedL1Cache(16, 100*time.Millisecond)

	// Set and Get
	err := c.Set(ctx, "project:1", []byte("project_payload"), 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	data, found, err := c.Get(ctx, "project:1")
	if err != nil || !found {
		t.Fatalf("Get failed: found=%v, err=%v", found, err)
	}
	if string(data) != "project_payload" {
		t.Fatalf("unexpected data: %s", string(data))
	}

	// Delete
	err = c.Delete(ctx, "project:1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, found, _ = c.Get(ctx, "project:1")
	if found {
		t.Fatal("expected item to be deleted")
	}
}

func TestShardedL1Cache_ConcurrentContention(t *testing.T) {
	ctx := context.Background()
	c := NewShardedL1Cache(32, time.Second)

	var wg sync.WaitGroup
	workers := 50
	opsPerWorker := 500

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < opsPerWorker; i++ {
				key := fmt.Sprintf("key:%d", (workerID+i)%20)
				val := []byte(fmt.Sprintf("val:%d", i))
				_ = c.Set(ctx, key, val, time.Second)
				_, _, _ = c.Get(ctx, key)
			}
		}(w)
	}

	wg.Wait()
}

func TestShardedL1Cache_HotKeyCounter(t *testing.T) {
	ctx := context.Background()
	c := NewShardedL1Cache(8, time.Second)

	hotKey := "project:viral-drop"
	_ = c.Set(ctx, hotKey, []byte("viral"), time.Second)

	for i := 0; i < 50; i++ {
		_, _, _ = c.Get(ctx, hotKey)
	}

	freq := c.Frequency(hotKey)
	if freq < 50 {
		t.Fatalf("expected frequency >= 50, got %d", freq)
	}
}
