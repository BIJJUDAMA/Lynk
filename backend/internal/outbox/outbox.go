package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Event represents a row in the outbox_events table.
type Event struct {
	ID            uuid.UUID      `db:"id"`
	EventType     string         `db:"event_type"`
	AggregateType string         `db:"aggregate_type"`
	AggregateID   string         `db:"aggregate_id"`
	Payload       map[string]any `db:"payload"`
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

	_, err = r.pool.Exec(ctx, `
		INSERT INTO outbox_events
			(id, event_type, aggregate_type, aggregate_id, payload, status, retry_count, max_retries, next_retry_at, created_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`,
		e.ID, e.EventType, e.AggregateType, e.AggregateID,
		payload, e.Status, e.RetryCount, e.MaxRetries, e.NextRetryAt, e.CreatedAt,
	)
	return err
}
