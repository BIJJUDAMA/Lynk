package workerpool_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lynk/backend/internal/workerpool"
)

func TestWorkerPool_ExecuteTasks(t *testing.T) {
	pool := workerpool.New(4, 16)

	var counter int64
	var wg sync.WaitGroup
	wg.Add(20)

	for i := 0; i < 20; i++ {
		err := pool.Submit(context.Background(), func() {
			atomic.AddInt64(&counter, 1)
			wg.Done()
		})
		if err != nil {
			t.Fatalf("failed to submit task %d: %v", i, err)
		}
	}

	wg.Wait()
	pool.Stop()

	if got := atomic.LoadInt64(&counter); got != 20 {
		t.Errorf("expected 20 executed tasks, got %d", got)
	}
}

func TestWorkerPool_QueueFullRejection(t *testing.T) {
	pool := workerpool.New(1, 1)
	defer pool.Stop()

	blocker := make(chan struct{})
	started := make(chan struct{})
	var closeOnce sync.Once
	defer closeOnce.Do(func() { close(blocker) })

	// Task 1: blocks the 1 worker
	err := pool.Submit(context.Background(), func() {
		close(started)
		<-blocker
	})
	if err != nil {
		t.Fatalf("failed to submit task 1: %v", err)
	}

	// Wait until worker is actively executing task 1 and blocking on blocker
	<-started

	// Task 2: fills the 1-slot queue buffer
	err = pool.Submit(context.Background(), func() {})
	if err != nil {
		t.Fatalf("failed to submit task 2: %v", err)
	}

	// Task 3: worker is busy, buffer is full -> must be rejected
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err = pool.Submit(ctx, func() {})
	if err != workerpool.ErrQueueFull {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}

	closeOnce.Do(func() { close(blocker) })
}

func TestWorkerPool_StopAndRejectNewTasks(t *testing.T) {
	pool := workerpool.New(2, 4)
	pool.Stop()

	err := pool.Submit(context.Background(), func() {})
	if err != workerpool.ErrPoolStopped {
		t.Fatalf("expected ErrPoolStopped, got %v", err)
	}

	// Double stop should be safe and idempotent
	pool.Stop()
}

func TestWorkerPool_ConcurrentSubmitAndStop(t *testing.T) {
	pool := workerpool.New(4, 16)
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = pool.Submit(context.Background(), func() {
					time.Sleep(time.Microsecond)
				})
			}
		}()
	}

	time.Sleep(5 * time.Millisecond)
	pool.Stop()
	wg.Wait()
}
