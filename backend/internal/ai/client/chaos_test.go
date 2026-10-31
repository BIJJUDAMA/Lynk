package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestChaos_MicroserviceSuddenOutage verifies that when the AI service suddenly
// drops TCP connections, calls gracefully degrade to deterministic fallback
// without blocking or returning an error.
func TestChaos_MicroserviceSuddenOutage(t *testing.T) {
	// Create an httptest server that abruptly hijacks and closes connections
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijack not supported", http.StatusInternalServerError)
			return
		}
		conn, _, _ := hj.Hijack()
		_ = conn.Close()
	}))
	defer server.Close()

	cb := NewCircuitBreaker(5, 50*time.Millisecond)
	cfg := Config{
		BaseURL:        server.URL,
		InternalSecret: "test-secret",
		MaxRetries:     1,
		RetryWaitMin:   5 * time.Millisecond,
		RetryWaitMax:   10 * time.Millisecond,
		CircuitBreaker: cb,
	}
	c := NewClient(cfg)

	ctx := context.Background()
	req := MatchCollaboratorsRequest{
		TargetUserID:  "student-1",
		MissingSkills: []string{"Go", "Kubernetes"},
		Candidates: []CollaboratorCandidate{
			{ID: "c1", UserID: "c1", Name: "Alice", Skills: []string{"Go"}, Rating: 4.8},
		},
		Limit: 5,
	}

	// Should degrade gracefully to deterministic fallback with nil error
	resp, err := c.MatchCollaborators(ctx, req)
	if err != nil {
		t.Fatalf("expected nil error on sudden outage fallback, got %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if len(resp.Matches) == 0 {
		t.Fatalf("expected fallback matches to be populated, got 0")
	}
	if resp.Matches[0].ID != "c1" {
		t.Errorf("expected candidate ID c1, got %s", resp.Matches[0].ID)
	}
}

// TestChaos_CircuitBreakerSelfHealing simulates 5 consecutive failures to trip circuit breaker,
// waits for cooldown, executes a successful request, and verifies circuit transitions
// from Open to Half-Open to Closed.
func TestChaos_CircuitBreakerSelfHealing(t *testing.T) {
	cooldown := 50 * time.Millisecond
	cb := NewCircuitBreaker(5, cooldown)

	// Step 1: Initial state is StateClosed
	if cb.State() != StateClosed {
		t.Fatalf("expected initial state StateClosed, got %v", cb.State())
	}

	// Step 2: 5 consecutive failures trip the circuit breaker
	for i := 1; i <= 5; i++ {
		err := cb.Execute(func() error {
			return errors.New("simulated microservice failure")
		})
		if err == nil {
			t.Fatalf("expected failure at iteration %d", i)
		}
	}

	// Step 3: Verify StateOpen
	if cb.State() != StateOpen {
		t.Fatalf("expected StateOpen after 5 failures, got %v", cb.State())
	}
	if cb.Failures() != 5 {
		t.Fatalf("expected 5 failures, got %d", cb.Failures())
	}

	// Subsequent execution while open must return ErrCircuitOpen
	err := cb.Execute(func() error {
		return nil
	})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen while open, got %v", err)
	}

	// Step 4: Wait for cooldown
	time.Sleep(cooldown + 15*time.Millisecond)

	// Step 5: Verify StateHalfOpen
	if cb.State() != StateHalfOpen {
		t.Fatalf("expected StateHalfOpen after cooldown, got %v", cb.State())
	}

	// Step 6: Successful probe request heals circuit
	err = cb.Execute(func() error {
		return nil
	})
	if err != nil {
		t.Fatalf("expected probe request to succeed, got %v", err)
	}

	// Step 7: Verify StateClosed
	if cb.State() != StateClosed {
		t.Fatalf("expected StateClosed after recovery, got %v", cb.State())
	}
	if cb.Failures() != 0 {
		t.Fatalf("expected 0 failures after recovery, got %d", cb.Failures())
	}
}

// TestChaos_CircuitBreakerSelfHealing_HTTPClient verifies circuit breaker self-healing
// end-to-end through HTTP client requests against an outage-recovering service.
func TestChaos_CircuitBreakerSelfHealing_HTTPClient(t *testing.T) {
	var failMode atomic.Bool
	failMode.Store(true)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failMode.Load() {
			http.Error(w, `{"error":"upstream unavailable"}`, http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	cooldown := 50 * time.Millisecond
	cb := NewCircuitBreaker(5, cooldown)
	c := NewClient(Config{
		BaseURL:        server.URL,
		CircuitBreaker: cb,
		MaxRetries:     1,
		RetryWaitMin:   1 * time.Millisecond,
		RetryWaitMax:   2 * time.Millisecond,
	})

	ctx := context.Background()

	// 5 requests fail, tripping the breaker
	for i := 0; i < 5; i++ {
		var resp map[string]string
		_ = c.doRequest(ctx, http.MethodGet, "/test", nil, &resp)
	}

	if cb.State() != StateOpen {
		t.Fatalf("expected StateOpen, got %v", cb.State())
	}

	// Wait for cooldown
	time.Sleep(cooldown + 15*time.Millisecond)

	if cb.State() != StateHalfOpen {
		t.Fatalf("expected StateHalfOpen, got %v", cb.State())
	}

	// Recover upstream
	failMode.Store(false)

	var resp map[string]string
	err := c.doRequest(ctx, http.MethodGet, "/test", nil, &resp)
	if err != nil {
		t.Fatalf("expected success after self-healing, got %v", err)
	}

	if cb.State() != StateClosed {
		t.Fatalf("expected StateClosed after recovery, got %v", cb.State())
	}
	if cb.Failures() != 0 {
		t.Fatalf("expected 0 failures after recovery, got %d", cb.Failures())
	}
}
