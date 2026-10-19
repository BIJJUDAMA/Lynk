package project

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// flushTimeout bounds a single flush so a stalled sink cannot block shutdown.
const flushTimeout = 2 * time.Second

// ViewFlushFunc handles persisting batched view counts to PostgreSQL.
type ViewFlushFunc func(ctx context.Context, batch map[uuid.UUID]int64) error

// ViewBatcher buffers high-frequency project page view increments in memory.
type ViewBatcher struct {
	mu        sync.Mutex
	pending   map[uuid.UUID]int64
	interval  time.Duration
	flushFunc ViewFlushFunc
	stopCh    chan struct{}
	stoppedCh chan struct{}
}

// NewViewBatcher creates a view count accumulator.
func NewViewBatcher(interval time.Duration, flushFunc ViewFlushFunc) *ViewBatcher {
	if interval <= 0 {
		interval = 1 * time.Second
	}
	return &ViewBatcher{
		pending:   make(map[uuid.UUID]int64),
		interval:  interval,
		flushFunc: flushFunc,
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}
}

// RecordView adds an atomic view increment into memory.
func (b *ViewBatcher) RecordView(projectID uuid.UUID) {
	b.mu.Lock()
	b.pending[projectID]++
	b.mu.Unlock()
}

// Start begins periodic background flushing.
func (b *ViewBatcher) Start(ctx context.Context) {
	ticker := time.NewTicker(b.interval)
	go func() {
		defer close(b.stoppedCh)
		defer ticker.Stop()
		for {
			select {
			case <-b.stopCh:
				_ = b.Flush(ctx)
				return
			case <-ticker.C:
				_ = b.Flush(ctx)
			}
		}
	}()
}

// Flush writes currently accumulated views and resets the in-memory map.
func (b *ViewBatcher) Flush(ctx context.Context) error {
	b.mu.Lock()
	if len(b.pending) == 0 {
		b.mu.Unlock()
		return nil
	}
	current := b.pending
	b.pending = make(map[uuid.UUID]int64)
	b.mu.Unlock()

	if b.flushFunc != nil {
		fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), flushTimeout)
		defer cancel()
		if err := b.flushFunc(fctx, current); err != nil {
			b.mu.Lock()
			for id, count := range current {
				b.pending[id] += count
			}
			b.mu.Unlock()
			return err
		}
	}
	return nil
}

// Stop terminates the background flush routine.
func (b *ViewBatcher) Stop() {
	close(b.stopCh)
	<-b.stoppedCh
}
