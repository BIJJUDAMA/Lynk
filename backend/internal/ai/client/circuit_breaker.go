package client

import (
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen is returned when the circuit breaker is open and refusing calls.
var ErrCircuitOpen = errors.New("ai client: circuit breaker is open")

// CircuitState represents the current state of the circuit breaker.
type CircuitState int

const (
	StateClosed CircuitState = iota
	StateHalfOpen
	StateOpen
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateHalfOpen:
		return "half-open"
	case StateOpen:
		return "open"
	default:
		return "unknown"
	}
}

// CircuitBreaker guards outbound calls against cascading failures.
type CircuitBreaker struct {
	mu          sync.RWMutex
	state       CircuitState
	failures    int
	maxFailures int
	cooldown    time.Duration
	lastFailure time.Time
}

// NewCircuitBreaker creates a new thread-safe CircuitBreaker.
func NewCircuitBreaker(maxFailures int, cooldown time.Duration) *CircuitBreaker {
	if maxFailures <= 0 {
		maxFailures = 5
	}
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}
	return &CircuitBreaker{
		state:       StateClosed,
		maxFailures: maxFailures,
		cooldown:    cooldown,
	}
}

// Execute runs the provided action guarded by the circuit breaker.
func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()
	now := time.Now()
	switch cb.state {
	case StateOpen:
		if now.Sub(cb.lastFailure) >= cb.cooldown {
			cb.state = StateHalfOpen
		} else {
			cb.mu.Unlock()
			return ErrCircuitOpen
		}
	case StateHalfOpen:
		// Allow probe request to evaluate upstream health
	case StateClosed:
		// Normal operation
	}
	cb.mu.Unlock()

	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()
		if cb.state == StateHalfOpen || cb.failures >= cb.maxFailures {
			cb.state = StateOpen
		}
		return err
	}

	// On successful execution
	cb.failures = 0
	cb.state = StateClosed
	return nil
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	if cb.state == StateOpen && time.Since(cb.lastFailure) >= cb.cooldown {
		return StateHalfOpen
	}
	return cb.state
}

// Failures returns the number of consecutive recorded failures.
func (cb *CircuitBreaker) Failures() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}

// Reset restores the circuit breaker to closed state with zero failures.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.failures = 0
	cb.lastFailure = time.Time{}
}
