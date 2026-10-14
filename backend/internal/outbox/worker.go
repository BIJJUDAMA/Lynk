package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HandlerFunc processes an outbox event. Returns an error to trigger retry.
type HandlerFunc func(ctx context.Context, e Event) error

// WorkerPool polls outbox_events and dispatches them to registered handlers.
type WorkerPool struct {
	pool         *pgxpool.Pool
	handlers     map[string]HandlerFunc
	batchSize    int
	pollInterval time.Duration
}

// NewWorkerPool creates a WorkerPool backed by the given PostgreSQL pool.
func NewWorkerPool(pool *pgxpool.Pool) *WorkerPool {
	return &WorkerPool{
		pool:         pool,
		handlers:     make(map[string]HandlerFunc),
		batchSize:    10,
		pollInterval: 2 * time.Second,
	}
}

// RegisterHandler registers a handler for a specific event type.
func (wp *WorkerPool) RegisterHandler(eventType string, fn HandlerFunc) {
	wp.handlers[eventType] = fn
}

// Start begins polling the outbox_events table until ctx is cancelled.
func (wp *WorkerPool) Start(ctx context.Context) {
	ticker := time.NewTicker(wp.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			wp.processBatch(ctx)
		}
	}
}

type rawRow struct {
	ID            string
	EventType     string
	AggregateType string
	AggregateID   string
	PayloadJSON   []byte
	RetryCount    int
	MaxRetries    int
}

func (wp *WorkerPool) claimBatch(ctx context.Context) ([]rawRow, error) {
	conn, err := wp.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		WITH claimed AS (
			SELECT id
			FROM outbox_events
			WHERE status IN ('pending', 'failed') AND next_retry_at <= NOW()
			ORDER BY created_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE outbox_events e
		SET status = 'processing'
		FROM claimed
		WHERE e.id = claimed.id
		RETURNING e.id, e.event_type, e.aggregate_type, e.aggregate_id, e.payload, e.retry_count, e.max_retries;
	`, wp.batchSize)
	if err != nil {
		return nil, fmt.Errorf("query claimed events: %w", err)
	}
	defer rows.Close()

	var batch []rawRow
	for rows.Next() {
		var r rawRow
		if err := rows.Scan(&r.ID, &r.EventType, &r.AggregateType, &r.AggregateID, &r.PayloadJSON, &r.RetryCount, &r.MaxRetries); err != nil {
			slog.Warn("outbox: scan failed", "error", err)
			continue
		}
		batch = append(batch, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	rows.Close()

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit claim transaction: %w", err)
	}

	return batch, nil
}

func (wp *WorkerPool) processBatch(ctx context.Context) {
	batch, err := wp.claimBatch(ctx)
	if err != nil {
		slog.Warn("outbox: claim batch failed", "error", err)
		return
	}
	if len(batch) == 0 {
		return
	}

	for _, r := range batch {
		var payload map[string]any
		if err := json.Unmarshal(r.PayloadJSON, &payload); err != nil {
			slog.Warn("outbox: payload unmarshal failed", "id", r.ID, "error", err)
			_, _ = wp.pool.Exec(ctx, `
				UPDATE outbox_events
				SET status = 'dead', last_error = $1
				WHERE id = $2
			`, "unmarshal payload: "+err.Error(), r.ID)
			continue
		}

		e := Event{
			EventType:     r.EventType,
			AggregateType: r.AggregateType,
			AggregateID:   r.AggregateID,
			Payload:       payload,
			RetryCount:    r.RetryCount,
			MaxRetries:    r.MaxRetries,
		}

		handle := wp.handlers[r.EventType]
		if handle == nil {
			handle = wp.handlers["*"]
		}

		var dispatchErr error
		if handle != nil {
			dispatchErr = handle(ctx, e)
		}

		if dispatchErr != nil {
			newRetry := r.RetryCount + 1
			backoff := CalculateBackoff(newRetry)
			newStatus := "failed"
			if newRetry >= r.MaxRetries {
				newStatus = "dead"
			}
			_, _ = wp.pool.Exec(ctx, `
				UPDATE outbox_events
				SET retry_count = $1, status = $2, next_retry_at = NOW() + ($3 * INTERVAL '1 second'), last_error = $4
				WHERE id = $5
			`, newRetry, newStatus, int64(backoff.Seconds()), dispatchErr.Error(), r.ID)
		} else {
			_, _ = wp.pool.Exec(ctx, `
				UPDATE outbox_events
				SET status = 'processed', processed_at = NOW()
				WHERE id = $1
			`, r.ID)
		}
	}
}

// CalculateBackoff returns the exponential backoff duration based on retry count.
// Formula: (1 << min(retry, 6)) * 5 seconds.
func CalculateBackoff(retry int) time.Duration {
	if retry < 0 {
		retry = 0
	}
	return time.Duration(1<<min(retry, 6)) * 5 * time.Second
}
