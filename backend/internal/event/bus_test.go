package event

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEventBus_PublishAndSubscribe(t *testing.T) {
	bus := NewBus()
	ctx := context.Background()

	var received1 []Event
	var received2 []Event
	var mu sync.Mutex

	bus.Subscribe(TopicContractCreated, func(ctx context.Context, e Event) error {
		mu.Lock()
		defer mu.Unlock()
		received1 = append(received1, e)
		return nil
	})

	bus.Subscribe(TopicContractCreated, func(ctx context.Context, e Event) error {
		mu.Lock()
		defer mu.Unlock()
		received2 = append(received2, e)
		return nil
	})

	testEvent := Event{
		Type:      TopicContractCreated,
		EntityID:  "contract-123",
		ActorID:   "user-456",
		Timestamp: time.Now().UTC(),
		Payload:   map[string]any{"job_id": "job-789"},
	}

	if err := bus.Publish(ctx, testEvent); err != nil {
		t.Fatalf("failed to publish event: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received1) != 1 {
		t.Fatalf("expected 1 event for handler 1, got %d", len(received1))
	}
	if received1[0].EntityID != "contract-123" {
		t.Errorf("expected entity ID 'contract-123', got %s", received1[0].EntityID)
	}
	if received1[0].ActorID != "user-456" {
		t.Errorf("expected actor ID 'user-456', got %s", received1[0].ActorID)
	}
	if len(received2) != 1 {
		t.Fatalf("expected 1 event for handler 2, got %d", len(received2))
	}
	if received2[0].EntityID != "contract-123" {
		t.Errorf("expected entity ID 'contract-123', got %s", received2[0].EntityID)
	}
}

func TestEventBus_DifferentTopicNotTriggered(t *testing.T) {
	bus := NewBus()
	ctx := context.Background()

	triggered := false
	bus.Subscribe(TopicReviewSubmitted, func(ctx context.Context, e Event) error {
		triggered = true
		return nil
	})

	err := bus.Publish(ctx, Event{
		Type:     TopicContractCreated,
		EntityID: "c-1",
	})
	if err != nil {
		t.Fatalf("unexpected publish error: %v", err)
	}

	if triggered {
		t.Errorf("expected review listener NOT to trigger for contract event")
	}
}

func TestEventBus_HandlerErrorPropagates(t *testing.T) {
	bus := NewBus()
	ctx := context.Background()
	expectedErr := errors.New("handler failed")

	bus.Subscribe(TopicJobCreated, func(ctx context.Context, e Event) error {
		return expectedErr
	})

	err := bus.Publish(ctx, Event{
		Type:     TopicJobCreated,
		EntityID: "job-1",
	})
	if err == nil {
		t.Fatalf("expected error from Publish, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected wrapped error to contain %v, got %v", expectedErr, err)
	}
}

func TestEventBus_ConcurrencySafe(t *testing.T) {
	bus := NewBus()
	ctx := context.Background()

	const numSubscribers = 10
	const numPublishers = 10
	const eventsPerPublisher = 50

	var totalHandled int64
	var wg sync.WaitGroup

	for i := 0; i < numSubscribers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			bus.Subscribe(TopicProfileUpdated, func(ctx context.Context, e Event) error {
				atomic.AddInt64(&totalHandled, 1)
				return nil
			})
		}(i)
	}

	for i := 0; i < numPublishers; i++ {
		wg.Add(1)
		go func(pubID int) {
			defer wg.Done()
			for j := 0; j < eventsPerPublisher; j++ {
				_ = bus.Publish(ctx, Event{
					Type:     TopicProfileUpdated,
					EntityID: fmt.Sprintf("user-%d-%d", pubID, j),
				})
			}
		}(i)
	}

	wg.Wait()

	var _ EventBus = bus
}
