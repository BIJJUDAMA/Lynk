package project

import (
	"context"
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
