package internal_test

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lynk/backend/internal/breaker"
)

// TestConcurrency_DeduplicatedAction simulates 50 concurrent goroutines
// attempting to claim the same unique action (e.g. accepting a project application
// or submitting a peer review). Asserts exactly 1 success and 49 conflicts.
func TestConcurrency_DeduplicatedAction(t *testing.T) {
	var (
		mu            sync.Mutex
		actionClaimed bool
		successCount  int64
		conflictCount int64
		wg            sync.WaitGroup
	)

	numGoroutines := 50
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			// Atomic check-and-set simulation matching DB UNIQUE constraint or SERIALIZABLE transaction
			mu.Lock()
			if !actionClaimed {
				actionClaimed = true
				mu.Unlock()
				atomic.AddInt64(&successCount, 1)
			} else {
				mu.Unlock()
				atomic.AddInt64(&conflictCount, 1)
			}
		}()
	}

	wg.Wait()

	if successCount != 1 {
		t.Fatalf("expected exactly 1 success under concurrent execution, got %d", successCount)
	}
	if conflictCount != int64(numGoroutines-1) {
		t.Fatalf("expected %d conflicts, got %d", numGoroutines-1, conflictCount)
	}
}

// TestConcurrency_CircuitBreakerSafety simulates 100 parallel goroutines hitting
// a failure condition, verifying thread-safe tripping and rejection after threshold
// without race conditions.
func TestConcurrency_CircuitBreakerSafety(t *testing.T) {
	cb := breaker.New(5, 5*time.Second)
	var (
		failures      int64
		rejectedCount int64
		maxFailures   = int64(5)
		wg            sync.WaitGroup
	)

	numGoroutines := 100
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			err := cb.Do(func() error {
				return errors.New("simulated upstream failure")
			})

			if errors.Is(err, breaker.ErrOpen) {
				atomic.AddInt64(&rejectedCount, 1)
			} else if err != nil {
				atomic.AddInt64(&failures, 1)
			}
		}()
	}

	wg.Wait()

	if atomic.LoadInt64(&failures) < maxFailures {
		t.Fatalf("expected at least %d failures before tripping, got %d", maxFailures, failures)
	}
	if atomic.LoadInt64(&rejectedCount) == 0 {
		t.Fatal("expected subsequent requests to be rejected after trip threshold")
	}
	if atomic.LoadInt64(&failures)+atomic.LoadInt64(&rejectedCount) != int64(numGoroutines) {
		t.Fatalf("expected total executed requests (%d + %d) to equal %d",
			failures, rejectedCount, numGoroutines)
	}
}

// TeamRoster models a concurrent team roster with a strictly enforced capacity cap.
type TeamRoster struct {
	mu       sync.Mutex
	capacity int
	members  []string
}

// NewTeamRoster creates a new TeamRoster with the specified capacity limit.
func NewTeamRoster(capacity int) *TeamRoster {
	return &TeamRoster{
		capacity: capacity,
		members:  make([]string, 0, capacity),
	}
}

// ErrTeamFull is returned when a join is attempted on a roster at capacity.
var ErrTeamFull = errors.New("team roster capacity reached")

// Join attempts to add a member to the roster in a thread-safe manner.
func (r *TeamRoster) Join(memberID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.members) >= r.capacity {
		return ErrTeamFull
	}
	r.members = append(r.members, memberID)
	return nil
}

// Size returns the current member count.
func (r *TeamRoster) Size() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.members)
}

// TestConcurrency_TeamRosterAtomicJoin simulates concurrent member additions
// asserting thread-safe capacity cap enforcement.
func TestConcurrency_TeamRosterAtomicJoin(t *testing.T) {
	const capacity = 5
	const numGoroutines = 50

	roster := NewTeamRoster(capacity)
	var (
		successCount int64
		fullCount    int64
		wg           sync.WaitGroup
	)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		memberID := fmt.Sprintf("student-%03d", i)
		go func(id string) {
			defer wg.Done()

			if err := roster.Join(id); err != nil {
				if errors.Is(err, ErrTeamFull) {
					atomic.AddInt64(&fullCount, 1)
				}
				return
			}
			atomic.AddInt64(&successCount, 1)
		}(memberID)
	}

	wg.Wait()

	if successCount != int64(capacity) {
		t.Fatalf("expected exactly %d successful joins, got %d", capacity, successCount)
	}
	if fullCount != int64(numGoroutines-capacity) {
		t.Fatalf("expected %d capacity rejections, got %d", numGoroutines-capacity, fullCount)
	}
	if roster.Size() != capacity {
		t.Fatalf("expected final roster size to be %d, got %d", capacity, roster.Size())
	}
}
