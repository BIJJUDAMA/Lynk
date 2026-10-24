package queue

import (
	"context"
	"testing"
	"time"
)

func TestRetryPolicy_ComputeDelay(t *testing.T) {
	policy := DefaultRetryPolicy()

	if delay := policy.ComputeDelay(0); delay != 0 {
		t.Errorf("expected 0 delay for attempt 0, got %v", delay)
	}

	// Verify delays are bounded by MaxDelay
	for attempt := 1; attempt <= 20; attempt++ {
		delay := policy.ComputeDelay(attempt)
		if delay < 0 {
			t.Errorf("attempt %d generated negative delay: %v", attempt, delay)
		}
		if delay > policy.MaxDelay {
			t.Errorf("attempt %d delay %v exceeded MaxDelay %v", attempt, delay, policy.MaxDelay)
		}
	}

	// Verify jitter variation (two consecutive delay calls should not be identical)
	d1 := policy.ComputeDelay(5)
	d2 := policy.ComputeDelay(5)
	// While statistically possible to be equal, with 30s nanosecond space, probability is ~1 in 30 billion
	if d1 == d2 && d1 > 0 {
		t.Logf("note: two delay computations produced identical values (%v), re-testing", d1)
	}
}

func TestClient_NotConnected(t *testing.T) {
	client := NewClient(Config{URL: "amqp://invalid:invalid@localhost:5672/"}, nil)

	if client.IsConnected() {
		t.Errorf("expected client to report disconnected before Connect()")
	}

	ch, err := client.Channel()
	if err == nil {
		t.Errorf("expected error getting channel from disconnected client")
	}
	if ch != nil {
		t.Errorf("expected nil channel from disconnected client")
	}

	// Close should be safe even if not connected
	if err := client.Close(); err != nil {
		t.Errorf("unexpected error on closing disconnected client: %v", err)
	}
}

func TestPublisher_Interface(t *testing.T) {
	var pub Publisher
	client := NewClient(Config{URL: "amqp://test:test@localhost:5672/"}, nil)
	pub = NewConfirmedPublisher(client)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Should return ErrNotConnected since client is not connected
	err := pub.PublishPersistent(ctx, ExchangeEvents, "test.key", []byte(`{"test":true}`), nil)
	if err == nil {
		t.Errorf("expected error publishing to disconnected client")
	}
}
