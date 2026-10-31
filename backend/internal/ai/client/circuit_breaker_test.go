package client

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestCircuitBreaker_TripsOnFailures(t *testing.T) {
	cb := NewCircuitBreaker(3, 50*time.Millisecond)

	// 3 consecutive failures
	for i := 0; i < 3; i++ {
		_ = cb.Execute(func() error {
			return errors.New("ai error")
		})
	}

	// Should be open now
	err := cb.Execute(func() error {
		return nil
	})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}

	// Wait for cooldown
	time.Sleep(60 * time.Millisecond)

	// Should be half-open; success resets it
	err = cb.Execute(func() error {
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error on half-open recovery, got %v", err)
	}

	if cb.State() != StateClosed {
		t.Fatalf("expected StateClosed, got %v", cb.State())
	}
	if cb.Failures() != 0 {
		t.Fatalf("expected 0 failures, got %d", cb.Failures())
	}
}

func TestCircuitBreaker_SuccessResetsFailures(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)

	// 2 failures (not yet 3)
	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error {
			return errors.New("ai error")
		})
	}
	if cb.Failures() != 2 {
		t.Fatalf("expected 2 failures, got %d", cb.Failures())
	}
	if cb.State() != StateClosed {
		t.Fatalf("expected StateClosed, got %v", cb.State())
	}

	// 1 success resets counter
	err := cb.Execute(func() error {
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cb.Failures() != 0 {
		t.Fatalf("expected 0 failures, got %d", cb.Failures())
	}
}

func TestCircuitBreaker_HalfOpenFailureReopens(t *testing.T) {
	cb := NewCircuitBreaker(2, 40*time.Millisecond)

	// 2 failures trip to Open
	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error {
			return errors.New("ai error")
		})
	}
	if cb.State() != StateOpen {
		t.Fatalf("expected StateOpen, got %v", cb.State())
	}

	// Wait for cooldown
	time.Sleep(50 * time.Millisecond)

	// Half-open attempt fails -> immediately trips back to Open
	err := cb.Execute(func() error {
		return errors.New("still failing")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if cb.State() != StateOpen {
		t.Fatalf("expected StateOpen after half-open failure, got %v", cb.State())
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(2, 10*time.Second)

	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error {
			return errors.New("ai error")
		})
	}
	if cb.State() != StateOpen {
		t.Fatalf("expected StateOpen, got %v", cb.State())
	}

	cb.Reset()
	if cb.State() != StateClosed {
		t.Fatalf("expected StateClosed after Reset, got %v", cb.State())
	}
	if cb.Failures() != 0 {
		t.Fatalf("expected 0 failures after Reset, got %d", cb.Failures())
	}
}

func TestCircuitBreaker_Concurrency(t *testing.T) {
	cb := NewCircuitBreaker(10, 50*time.Millisecond)
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = cb.Execute(func() error {
				if idx%2 == 0 {
					return errors.New("intermittent error")
				}
				return nil
			})
			_ = cb.State()
			_ = cb.Failures()
		}(i)
	}

	wg.Wait()
}
