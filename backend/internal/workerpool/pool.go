// Package workerpool provides bounded goroutine worker pool execution.
// 100% pure 7-bit ASCII
package workerpool

import (
	"context"
	"errors"
	"sync"
)

var (
	// ErrPoolStopped is returned when submitting tasks to a stopped pool.
	ErrPoolStopped = errors.New("workerpool: pool is stopped")
	// ErrQueueFull is returned when task queue capacity is exhausted or context is done.
	ErrQueueFull = errors.New("workerpool: task queue is full")
)

// Pool executes submitted tasks using a bounded number of worker goroutines.
type Pool struct {
	tasks  chan func()
	wg     sync.WaitGroup
	quit   chan struct{}
	closed sync.Once
}

// New creates and starts a bounded worker pool with the specified number of workers and queue size.
func New(workers, queueSize int) *Pool {
	if workers <= 0 {
		workers = 1
	}
	if queueSize <= 0 {
		queueSize = 1
	}

	p := &Pool{
		tasks: make(chan func(), queueSize),
		quit:  make(chan struct{}),
	}

	p.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go p.worker()
	}

	return p
}

func (p *Pool) worker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.quit:
			// Drain remaining buffered tasks
			for {
				select {
				case task, ok := <-p.tasks:
					if ok && task != nil {
						task()
					}
				default:
					return
				}
			}
		case task, ok := <-p.tasks:
			if !ok {
				return
			}
			if task != nil {
				task()
			}
		}
	}
}

// Submit enqueues a task for execution by the pool.
// Returns ErrPoolStopped if pool is stopped, or ErrQueueFull if context is cancelled before queue accepts the task.
func (p *Pool) Submit(ctx context.Context, task func()) error {
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-p.quit:
		return ErrPoolStopped
	default:
	}

	select {
	case <-p.quit:
		return ErrPoolStopped
	case <-ctx.Done():
		return ErrQueueFull
	case p.tasks <- task:
		return nil
	}
}

// Stop gracefully shuts down the worker pool, waiting for running and buffered tasks to complete.
func (p *Pool) Stop() {
	p.closed.Do(func() {
		close(p.quit)
		p.wg.Wait()
	})
}
