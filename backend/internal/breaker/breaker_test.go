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
	_ = b.Do(func() error { return ef }) // now only 1 failure (below threshold)

	if err := b.Allow(); err != nil {
		t.Fatalf("expected circuit to be closed, got %v", err)
	}
}
