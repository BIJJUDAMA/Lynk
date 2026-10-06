package breaker

import (
	"errors"
	"sync"
	"time"
)

// ErrOpen is returned when the circuit breaker is in the Open state.
var ErrOpen = errors.New("circuit breaker is open")

// State represents the circuit breaker state.
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

// Breaker is a lightweight circuit breaker.
type Breaker struct {
	mu           sync.Mutex
	state        State
	failures     int
	threshold    int
	openUntil    time.Time
	recoveryTime time.Duration
}

// New creates a Breaker that opens after threshold consecutive failures
// and attempts recovery after recoveryTime.
func New(threshold int, recoveryTime time.Duration) *Breaker {
	return &Breaker{
		threshold:    threshold,
		recoveryTime: recoveryTime,
	}
}

// Allow returns true if the request is permitted. Returns ErrOpen when the
// circuit is open and recovery window has not elapsed.
func (b *Breaker) Allow() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateOpen:
		if time.Now().After(b.openUntil) {
			b.state = StateHalfOpen
			return nil
		}
		return ErrOpen
	default:
		return nil
	}
}

// RecordSuccess resets failure count and closes the circuit.
func (b *Breaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.state = StateClosed
}

// RecordFailure increments failure count; opens the circuit at threshold.
func (b *Breaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	if b.failures >= b.threshold {
		b.state = StateOpen
		b.openUntil = time.Now().Add(b.recoveryTime)
	}
}

// Do executes fn guarded by the circuit breaker.
func (b *Breaker) Do(fn func() error) error {
	if err := b.Allow(); err != nil {
		return err
	}
	if err := fn(); err != nil {
		b.RecordFailure()
		return err
	}
	b.RecordSuccess()
	return nil
}
