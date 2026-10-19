package project

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestViewBatcher_AccumulationAndFlush(t *testing.T) {
	var flushedBatches []map[uuid.UUID]int64
	var mu sync.Mutex

	batcher := NewViewBatcher(20*time.Millisecond, func(ctx context.Context, batch map[uuid.UUID]int64) error {
		mu.Lock()
		defer mu.Unlock()
		cpy := make(map[uuid.UUID]int64, len(batch))
		for k, v := range batch {
			cpy[k] = v
		}
		flushedBatches = append(flushedBatches, cpy)
		return nil
	})

	id1 := uuid.New()
	id2 := uuid.New()

	batcher.Start(context.Background())
	defer batcher.Stop()

	// Accumulate 50 views
	for i := 0; i < 30; i++ {
		batcher.RecordView(id1)
	}
	for i := 0; i < 20; i++ {
		batcher.RecordView(id2)
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(flushedBatches) == 0 {
		t.Fatal("expected at least one flush batch")
	}

	var total1, total2 int64
	for _, b := range flushedBatches {
		total1 += b[id1]
		total2 += b[id2]
	}

	if total1 != 30 {
		t.Fatalf("expected 30 views for id1, got %d", total1)
	}
	if total2 != 20 {
		t.Fatalf("expected 20 views for id2, got %d", total2)
	}
}

func TestViewBatcher_FlushFailureRetainsBatch(t *testing.T) {
	const views = 25
	id := uuid.New()

	var mu sync.Mutex
	var calls int
	var successful map[uuid.UUID]int64

	batcher := NewViewBatcher(time.Second, func(ctx context.Context, batch map[uuid.UUID]int64) error {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls == 1 {
			return context.DeadlineExceeded
		}
		successful = make(map[uuid.UUID]int64, len(batch))
		for k, v := range batch {
			successful[k] = v
		}
		return nil
	})

	for i := 0; i < views; i++ {
		batcher.RecordView(id)
	}

	if err := batcher.Flush(context.Background()); err == nil {
		t.Fatal("expected first Flush to return the flushFunc error")
	}

	if err := batcher.Flush(context.Background()); err != nil {
		t.Fatalf("expected second Flush to succeed, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if calls != 2 {
		t.Fatalf("expected 2 flushFunc calls, got %d", calls)
	}
	if got := successful[id]; got != views {
		t.Fatalf("expected %d views persisted on retry, got %d", views, got)
	}
	if len(successful) != 1 {
		t.Fatalf("expected exactly 1 project in retried batch, got %d", len(successful))
	}
}

func TestViewBatcher_FlushReturnsWhenSinkStalls(t *testing.T) {
	batcher := NewViewBatcher(time.Second, func(ctx context.Context, batch map[uuid.UUID]int64) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(30 * time.Second):
			return nil
		}
	})
	batcher.RecordView(uuid.New())

	done := make(chan error, 1)
	go func() {
		done <- batcher.Flush(context.Background())
	}()

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected DeadlineExceeded from bounded flush, got %v", err)
		}
	case <-time.After(flushTimeout + 3*time.Second):
		t.Fatal("Flush hung past the bounded timeout")
	}
}
