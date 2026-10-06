package outbox

import (
	"context"
	"encoding/json"
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

func (wp *WorkerPool) processBatch(ctx context.Context) {
	conn, err := wp.pool.Acquire(ctx)
	if err != nil {
		slog.Warn("outbox: failed to acquire connection", "error", err)
		return
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		slog.Warn("outbox: failed to begin transaction", "error", err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		SELECT id, event_type, aggregate_type, aggregate_id, payload, retry_count, max_retries
		FROM outbox_events
		WHERE status IN ('pending', 'failed') AND next_retry_at <= NOW()
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`, wp.batchSize)
	if err != nil {
		slog.Warn("outbox: query failed", "error", err)
		return
	}
	defer rows.Close()

	type rawRow struct {
		ID            string
		EventType     string
		AggregateType string
		AggregateID   string
		PayloadJSON   []byte
		RetryCount    int
		MaxRetries    int
	}

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
		slog.Warn("outbox: rows error", "error", err)
		return
	}

	for _, r := range batch {
		var payload map[string]any
		if err := json.Unmarshal(r.PayloadJSON, &payload); err != nil {
			slog.Warn("outbox: payload unmarshal failed", "id", r.ID, "error", err)
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
			backoff := time.Duration(1<<min(newRetry, 6)) * 5 * time.Second
			newStatus := "failed"
			if newRetry >= r.MaxRetries {
				newStatus = "dead"
			}
			_, _ = tx.Exec(ctx, `
				UPDATE outbox_events
				SET retry_count = $1, status = $2, next_retry_at = NOW() + ($3 * INTERVAL '1 second'), last_error = $4
				WHERE id = $5
			`, newRetry, newStatus, int64(backoff.Seconds()), dispatchErr.Error(), r.ID)
		} else {
			now := time.Now().UTC()
			_, _ = tx.Exec(ctx, `
				UPDATE outbox_events
				SET status = 'processed', processed_at = $1
				WHERE id = $2
			`, now, r.ID)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		slog.Warn("outbox: commit failed", "error", err)
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
