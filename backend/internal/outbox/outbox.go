package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lynk/backend/internal/queue"
)

// Event represents a row in the outbox_events table.
type Event struct {
	ID            uuid.UUID      `db:"id"`
	EventType     string         `db:"event_type"`
	AggregateType string         `db:"aggregate_type"`
	AggregateID   string         `db:"aggregate_id"`
	Payload       map[string]any `db:"payload"`
	Headers       map[string]any `db:"headers"`
	Status        string         `db:"status"`
	RetryCount    int            `db:"retry_count"`
	MaxRetries    int            `db:"max_retries"`
	NextRetryAt   time.Time      `db:"next_retry_at"`
	CreatedAt     time.Time      `db:"created_at"`
	ProcessedAt   *time.Time     `db:"processed_at"`
}

// Recorder persists raw outbox events (low-level storage interface).
type Recorder interface {
	RecordEvent(ctx context.Context, e Event) error
}

// TxRecorder allows recording outbox events atomically within a PostgreSQL transaction.
type TxRecorder interface {
	RecordEventTx(ctx context.Context, tx pgx.Tx, e Event) error
}

// Publisher is the high-level interface services use to emit domain events.
// It accepts a pre-built Event struct for ergonomics.
type Publisher interface {
	Publish(ctx context.Context, e Event) error
}

// Writer implements Publisher on top of a Recorder (e.g. DBRecorder).
// It validates fields and fills in generated values (ID, Status, timestamps).
type Writer struct {
	repo Recorder
}

// NewWriter creates a new Writer backed by the given Recorder.
// To build one backed by PostgreSQL directly, pass a *DBRecorder.
func NewWriter(repo Recorder) *Writer {
	return &Writer{repo: repo}
}

// Publish validates and persists an outbox event.
func (w *Writer) Publish(ctx context.Context, e Event) error {
	if e.EventType == "" {
		return fmt.Errorf("outbox: event_type is required")
	}
	if e.AggregateType == "" {
		return fmt.Errorf("outbox: aggregate_type is required")
	}
	if e.AggregateID == "" {
		return fmt.Errorf("outbox: aggregate_id is required")
	}

	if e.Payload == nil {
		e.Payload = map[string]any{}
	}

	// Validate that payload is JSON-serializable
	if _, err := json.Marshal(e.Payload); err != nil {
		return fmt.Errorf("outbox: payload is not JSON-serializable: %w", err)
	}

	e.ID = uuid.New()
	e.Status = "pending"
	e.RetryCount = 0
	if e.MaxRetries == 0 {
		e.MaxRetries = 5
	}
	e.NextRetryAt = time.Now().UTC()
	e.CreatedAt = time.Now().UTC()

	return w.repo.RecordEvent(ctx, e)
}

// DBRecorder implements Recorder using a PostgreSQL connection pool.
// It is the production Recorder backing Writer.
type DBRecorder struct {
	pool *pgxpool.Pool
}

// NewDBRecorder creates a Recorder that writes directly to the outbox_events table.
func NewDBRecorder(pool *pgxpool.Pool) *DBRecorder {
	return &DBRecorder{pool: pool}
}

// RecordEvent inserts one row into outbox_events.
func (r *DBRecorder) RecordEvent(ctx context.Context, e Event) error {
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		return fmt.Errorf("outbox: marshal payload: %w", err)
	}

	headers, err := json.Marshal(e.Headers)
	if err != nil {
		return fmt.Errorf("outbox: marshal headers: %w", err)
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO outbox_events
			(id, event_type, aggregate_type, aggregate_id, payload, headers, status, retry_count, max_retries, next_retry_at, created_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`,
		e.ID, e.EventType, e.AggregateType, e.AggregateID,
		payload, headers, e.Status, e.RetryCount, e.MaxRetries, e.NextRetryAt, e.CreatedAt,
	)
	return err
}

// RecordEventTx inserts one row into outbox_events within an ongoing database transaction.
func (r *DBRecorder) RecordEventTx(ctx context.Context, tx pgx.Tx, e Event) error {
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		return fmt.Errorf("outbox: marshal payload: %w", err)
	}

	headers, err := json.Marshal(e.Headers)
	if err != nil {
		return fmt.Errorf("outbox: marshal headers: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events
			(id, event_type, aggregate_type, aggregate_id, payload, headers, status, retry_count, max_retries, next_retry_at, created_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`,
		e.ID, e.EventType, e.AggregateType, e.AggregateID,
		payload, headers, e.Status, e.RetryCount, e.MaxRetries, e.NextRetryAt, e.CreatedAt,
	)
	return err
}

// PublishTx validates and persists an outbox event within a transaction.
func (w *Writer) PublishTx(ctx context.Context, tx pgx.Tx, e Event) error {
	if e.EventType == "" {
		return fmt.Errorf("outbox: event_type is required")
	}
	if e.AggregateType == "" {
		return fmt.Errorf("outbox: aggregate_type is required")
	}
	if e.AggregateID == "" {
		return fmt.Errorf("outbox: aggregate_id is required")
	}

	if e.Payload == nil {
		e.Payload = map[string]any{}
	}
	if e.Headers == nil {
		e.Headers = map[string]any{}
	}

	if _, err := json.Marshal(e.Payload); err != nil {
		return fmt.Errorf("outbox: payload is not JSON-serializable: %w", err)
	}

	e.ID = uuid.New()
	e.Status = "pending"
	e.RetryCount = 0
	if e.MaxRetries == 0 {
		e.MaxRetries = 5
	}
	e.NextRetryAt = time.Now().UTC()
	e.CreatedAt = time.Now().UTC()

	txRec, ok := w.repo.(TxRecorder)
	if !ok {
		return fmt.Errorf("outbox: recorder does not support transactions")
	}
	return txRec.RecordEventTx(ctx, tx, e)
}

// RabbitMQDispatcher returns an outbox HandlerFunc that relays events to RabbitMQ with publisher confirms.
func RabbitMQDispatcher(pub queue.Publisher) HandlerFunc {
	return func(ctx context.Context, e Event) error {
		routingKey := fmt.Sprintf("%s.%s", e.AggregateType, e.EventType)
		payloadBytes, err := json.Marshal(e.Payload)
		if err != nil {
			return fmt.Errorf("failed to marshal event payload: %w", err)
		}

		headers := make(map[string]interface{})
		for k, v := range e.Headers {
			headers[k] = v
		}

		return pub.PublishPersistent(ctx, queue.ExchangeEvents, routingKey, payloadBytes, headers)
	}
}
