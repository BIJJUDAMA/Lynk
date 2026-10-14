package breaker

import (
	"errors"
	"testing"
	"time"
)

func TestBreaker_OpensAfterThreshold(t *testing.T) {
	b := New(3, 100*time.Millisecond)
	ef := errors.New("service down")

	for i := 0; i < 3; i++ {
		_ = b.Do(func() error { return ef })
	}

	if err := b.Allow(); !errors.Is(err, ErrOpen) {
		t.Fatalf("expected ErrOpen after threshold, got %v", err)
	}
}

func TestBreaker_RecoveryAfterTimeout(t *testing.T) {
	b := New(1, 50*time.Millisecond)
	ef := errors.New("fail")
	_ = b.Do(func() error { return ef })

	time.Sleep(60 * time.Millisecond)

	if err := b.Allow(); err != nil {
		t.Fatalf("expected circuit to allow after recovery, got %v", err)
	}
}

func TestBreaker_SuccessResets(t *testing.T) {
	b := New(2, time.Second)
	ef := errors.New("err")
	_ = b.Do(func() error { return ef })
	_ = b.Do(func() error { return nil }) // success resets
	_ = b.Do(func() error { return ef })  // now only 1 failure (below threshold)

	if err := b.Allow(); err != nil {
		t.Fatalf("expected circuit to be closed, got %v", err)
	}
}

func TestBreaker_HalfOpenSingleFailureReopens(t *testing.T) {
	// Breaker with threshold=3, recoveryTime=10ms
	b := New(3, 10*time.Millisecond)

	// Cause 3 failures to open circuit
	b.RecordFailure()
	b.RecordFailure()
	b.RecordFailure()
	if err := b.Allow(); !errors.Is(err, ErrOpen) {
		t.Fatalf("expected ErrOpen, got %v", err)
	}

	// Wait for recovery time to elapse
	time.Sleep(15 * time.Millisecond)

	// Probe request transitions to half-open
	if err := b.Allow(); err != nil {
		t.Fatalf("expected nil in half-open, got %v", err)
	}

	// A single failure in half-open state must immediately trip back to open
	b.RecordFailure()

	if err := b.Allow(); !errors.Is(err, ErrOpen) {
		t.Fatalf("expected ErrOpen immediately after 1 failure in half-open, got %v", err)
	}
}

func TestBreaker_HalfOpenZeroFailuresReopensOnSingleFailure(t *testing.T) {
	b := New(3, 10*time.Millisecond)
	b.RecordFailure()
	b.RecordFailure()
	b.RecordFailure()

	time.Sleep(15 * time.Millisecond)
	if err := b.Allow(); err != nil {
		t.Fatalf("expected nil in half-open, got %v", err)
	}

	// Explicitly clear failures while in half-open
	b.failures = 0

	// A single failure must immediately trip back to open
	b.RecordFailure()

	if err := b.Allow(); !errors.Is(err, ErrOpen) {
		t.Fatalf("expected ErrOpen immediately after 1 failure in half-open, got %v", err)
	}
	if b.failures != b.threshold {
		t.Fatalf("expected failures to be set to threshold %d, got %d", b.threshold, b.failures)
	}
}



