package outbox

import (
	"context"
	"testing"
	"time"
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

		inlineDuration := time.Duration(1<<min(tc.retry, 6)) * 5 * time.Second
		if int64(inlineDuration.Seconds()) != tc.expectedSeconds {
			t.Errorf("inline formula for retry %d = %d seconds, expected %d", tc.retry, int64(inlineDuration.Seconds()), tc.expectedSeconds)
		}
	}
}
