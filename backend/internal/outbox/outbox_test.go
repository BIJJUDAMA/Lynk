package outbox

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type mockOutboxRepo struct {
	events []Event
}

func (m *mockOutboxRepo) RecordEvent(ctx context.Context, e Event) error {
	m.events = append(m.events, e)
	return nil
}

func TestOutbox_RecordEvent(t *testing.T) {
	repo := &mockOutboxRepo{}
	writer := NewWriter(repo)

	err := writer.Publish(context.Background(), Event{
		EventType:     "application_submitted",
		AggregateType: "application",
		AggregateID:   "app-123",
		Payload: map[string]any{
			"job_id":       "job-456",
			"applicant_id": "user-789",
		},
	})
	if err != nil {
		t.Fatalf("failed to record outbox event: %v", err)
	}

	if len(repo.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(repo.events))
	}
	if repo.events[0].EventType != "application_submitted" {
		t.Errorf("expected application_submitted, got %s", repo.events[0].EventType)
	}
}

func TestOutbox_Publish_Validation(t *testing.T) {
	repo := &mockOutboxRepo{}
	writer := NewWriter(repo)
	ctx := context.Background()

	// Missing event_type
	if err := writer.Publish(ctx, Event{AggregateType: "application", AggregateID: "app-1"}); err == nil {
		t.Error("expected error for empty event_type")
	}

	// Missing aggregate_type
	if err := writer.Publish(ctx, Event{EventType: "evt", AggregateID: "app-1"}); err == nil {
		t.Error("expected error for empty aggregate_type")
	}

	// Missing aggregate_id
	if err := writer.Publish(ctx, Event{EventType: "evt", AggregateType: "application"}); err == nil {
		t.Error("expected error for empty aggregate_id")
	}

	// Nil payload should be normalized to empty map without error
	if err := writer.Publish(ctx, Event{EventType: "evt", AggregateType: "application", AggregateID: "app-1"}); err != nil {
		t.Errorf("expected nil error for nil payload, got: %v", err)
	}
}

func TestOutbox_BackoffSecondsCalculation(t *testing.T) {
	testCases := []struct {
		retry           int
		expectedSeconds int64
	}{
		{retry: -2, expectedSeconds: 5},
		{retry: -1, expectedSeconds: 5},
		{retry: 0, expectedSeconds: 5},
		{retry: 1, expectedSeconds: 10},
		{retry: 2, expectedSeconds: 20},
		{retry: 3, expectedSeconds: 40},
		{retry: 4, expectedSeconds: 80},
		{retry: 5, expectedSeconds: 160},
		{retry: 6, expectedSeconds: 320},
		{retry: 7, expectedSeconds: 320},
		{retry: 10, expectedSeconds: 320},
	}

	for _, tc := range testCases {
		backoff := CalculateBackoff(tc.retry)
		seconds := int64(backoff.Seconds())
		if seconds != tc.expectedSeconds {
			t.Errorf("CalculateBackoff(%d) = %d seconds, expected %d", tc.retry, seconds, tc.expectedSeconds)
		}

		clamped := max(tc.retry, 0)
		inlineDuration := time.Duration(1<<min(clamped, 6)) * 5 * time.Second
		if int64(inlineDuration.Seconds()) != tc.expectedSeconds {
			t.Errorf("inline formula for retry %d = %d seconds, expected %d", tc.retry, int64(inlineDuration.Seconds()), tc.expectedSeconds)
		}
	}
}

func TestWorkerPool_RegisterHandler(t *testing.T) {
	wp := NewWorkerPool(nil)
	calledSpecific := false
	calledWildcard := false

	wp.RegisterHandler("user_registered", func(ctx context.Context, e Event) error {
		calledSpecific = true
		return nil
	})
	wp.RegisterHandler("*", func(ctx context.Context, e Event) error {
		calledWildcard = true
		return nil
	})

	if wp.handlers["user_registered"] == nil {
		t.Fatal("expected handler to be registered for user_registered")
	}
	if wp.handlers["*"] == nil {
		t.Fatal("expected handler to be registered for wildcard *")
	}

	_ = wp.handlers["user_registered"](context.Background(), Event{EventType: "user_registered"})
	if !calledSpecific {
		t.Errorf("expected specific handler to be called")
	}

	_ = wp.handlers["*"](context.Background(), Event{EventType: "other_event"})
	if !calledWildcard {
		t.Errorf("expected wildcard handler to be called")
	}
}

func TestOutboxWorker_HandlerExecutionDoesNotHoldTxConnection(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	defer pool.Close()

	wp := NewWorkerPool(pool)
	handlerStarted := make(chan struct{})
	handlerFinish := make(chan struct{})

	eventID := uuid.New()
	wp.RegisterHandler("test_batch_event", func(ctx context.Context, e Event) error {
		close(handlerStarted)
		<-handlerFinish
		return nil
	})

	// Insert test event
	_, err = pool.Exec(ctx, `
		INSERT INTO outbox_events (id, event_type, aggregate_type, aggregate_id, payload, status)
		VALUES ($1, 'test_batch_event', 'test', '123', '{"foo":"bar"}', 'pending')
	`, eventID)
	if err != nil {
		t.Fatalf("failed to insert test event: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM outbox_events WHERE id = $1", eventID)
	}()

	workerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go wp.processBatch(workerCtx)

	// Wait for handler to start
	select {
	case <-handlerStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not start in time")
	}

	// Verify we can acquire connections from the pool freely without being blocked by an open transaction
	conn, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("failed to acquire connection while handler was executing: %v", err)
	}
	conn.Release()

	close(handlerFinish)

	// Wait for event to transition to processed
	deadline := time.Now().Add(3 * time.Second)
	var status string
	for time.Now().Before(deadline) {
		err = pool.QueryRow(ctx, "SELECT status FROM outbox_events WHERE id = $1", eventID).Scan(&status)
		if err == nil && status == "processed" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if status != "processed" {
		t.Fatalf("expected event status to be processed, got %s", status)
	}
}

func TestOutboxWorker_HandlerFailureCalculatesBackoff_Integration(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	defer pool.Close()

	wp := NewWorkerPool(pool)
	eventID := uuid.New()
	expectedErr := errors.New("simulated handler failure")

	wp.RegisterHandler("test_fail_event", func(ctx context.Context, e Event) error {
		return expectedErr
	})

	// Insert test event with retry_count = 0, max_retries = 3
	_, err = pool.Exec(ctx, `
		INSERT INTO outbox_events (id, event_type, aggregate_type, aggregate_id, payload, status, retry_count, max_retries)
		VALUES ($1, 'test_fail_event', 'test', '456', '{"retry":"test"}', 'pending', 0, 3)
	`, eventID)
	if err != nil {
		t.Fatalf("failed to insert test event: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM outbox_events WHERE id = $1", eventID)
	}()

	wp.processBatch(ctx)

	var status string
	var retryCount int
	var lastError *string
	err = pool.QueryRow(ctx, "SELECT status, retry_count, last_error FROM outbox_events WHERE id = $1", eventID).Scan(&status, &retryCount, &lastError)
	if err != nil {
		t.Fatalf("failed to query event: %v", err)
	}
	if status != "failed" {
		t.Errorf("expected status 'failed', got %q", status)
	}
	if retryCount != 1 {
		t.Errorf("expected retry_count 1, got %d", retryCount)
	}
	if lastError == nil || *lastError != expectedErr.Error() {
		t.Errorf("expected last_error %q, got %v", expectedErr.Error(), lastError)
	}
}

func TestOutboxWorker_HandlerMaxRetriesDead_Integration(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	defer pool.Close()

	wp := NewWorkerPool(pool)
	eventID := uuid.New()
	expectedErr := errors.New("terminal failure")

	wp.RegisterHandler("test_dead_event", func(ctx context.Context, e Event) error {
		return expectedErr
	})

	// Insert test event with retry_count = 2, max_retries = 3
	_, err = pool.Exec(ctx, `
		INSERT INTO outbox_events (id, event_type, aggregate_type, aggregate_id, payload, status, retry_count, max_retries)
		VALUES ($1, 'test_dead_event', 'test', '789', '{"dead":"test"}', 'pending', 2, 3)
	`, eventID)
	if err != nil {
		t.Fatalf("failed to insert test event: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM outbox_events WHERE id = $1", eventID)
	}()

	wp.processBatch(ctx)

	var status string
	var retryCount int
	err = pool.QueryRow(ctx, "SELECT status, retry_count FROM outbox_events WHERE id = $1", eventID).Scan(&status, &retryCount)
	if err != nil {
		t.Fatalf("failed to query event: %v", err)
	}
	if status != "dead" {
		t.Errorf("expected status 'dead', got %q", status)
	}
	if retryCount != 3 {
		t.Errorf("expected retry_count 3, got %d", retryCount)
	}
}

type mockTxRecorder struct {
	events []Event
}

func (m *mockTxRecorder) RecordEvent(ctx context.Context, e Event) error {
	m.events = append(m.events, e)
	return nil
}

func (m *mockTxRecorder) RecordEventTx(ctx context.Context, tx pgx.Tx, e Event) error {
	m.events = append(m.events, e)
	return nil
}

type mockPublisher struct {
	publishedRoutes []string
}

func (p *mockPublisher) PublishPersistent(ctx context.Context, exchange, routingKey string, body []byte, headers map[string]interface{}) error {
	p.publishedRoutes = append(p.publishedRoutes, routingKey)
	return nil
}

func TestOutbox_PublishTx(t *testing.T) {
	rec := &mockTxRecorder{}
	writer := NewWriter(rec)

	err := writer.PublishTx(context.Background(), nil, Event{
		EventType:     "member_invited",
		AggregateType: "team",
		AggregateID:   "team-1",
		Payload:       map[string]any{"email": "invited@campus.edu"},
	})
	if err != nil {
		t.Fatalf("unexpected PublishTx error: %v", err)
	}
	if len(rec.events) != 1 {
		t.Fatalf("expected 1 event recorded, got %d", len(rec.events))
	}
	if rec.events[0].EventType != "member_invited" {
		t.Errorf("expected member_invited event, got %s", rec.events[0].EventType)
	}
}

func TestOutbox_RabbitMQDispatcher(t *testing.T) {
	pub := &mockPublisher{}
	dispatcher := RabbitMQDispatcher(pub)

	evt := Event{
		AggregateType: "audit",
		EventType:     "contract_signed",
		Payload:       map[string]any{"contract_id": "c1"},
	}

	err := dispatcher(context.Background(), evt)
	if err != nil {
		t.Fatalf("unexpected dispatcher error: %v", err)
	}
	if len(pub.publishedRoutes) != 1 || pub.publishedRoutes[0] != "audit.contract_signed" {
		t.Errorf("expected published route 'audit.contract_signed', got %v", pub.publishedRoutes)
	}
}
