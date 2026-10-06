package ratelimit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type mockStore struct {
	counts map[string]int
}

func (m *mockStore) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Duration, error) {
	m.counts[key]++
	if m.counts[key] > limit {
		return false, 0, window, nil
	}
	return true, limit - m.counts[key], window, nil
}

func TestSlidingWindowLimiter_RejectsOverLimit(t *testing.T) {
	store := &mockStore{counts: make(map[string]int)}
	middleware := NewMiddlewareWithStore(store, 2, time.Minute)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request 1: OK
	req1 := httptest.NewRequest("GET", "/api/v1/jobs", nil)
	req1.RemoteAddr = "192.168.1.1:1234"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w1.Code)
	}

	// Request 2: OK
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req1)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w2.Code)
	}

	// Request 3: 429 Too Many Requests
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req1)
	if w3.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests, got %d", w3.Code)
	}
	if w3.Header().Get("Retry-After") == "" {
		t.Fatalf("expected Retry-After header on 429 response")
	}

	var errResp map[string]string
	if err := json.Unmarshal(w3.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to decode json error body: %v", err)
	}
	if errResp["error"] != "too_many_requests" {
		t.Fatalf("expected error='too_many_requests', got %s", errResp["error"])
	}
}

func TestSlidingWindow_MemoryStore(t *testing.T) {
	store := NewMemoryStore()
	window := 50 * time.Millisecond
	limit := 3

	middleware := NewSlidingWindowMiddleware(store, limit, window)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/jobs", nil)
	req.RemoteAddr = "10.0.0.5:54321"

	// First N requests should succeed
	for i := 1; i <= limit; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d expected status 200, got %d", i, w.Code)
		}
		if rem := w.Header().Get("X-RateLimit-Remaining"); rem != string(rune('0'+(limit-i))) {
			t.Logf("request %d remaining header: %s", i, rem)
		}
	}

	// (N+1)th request should be blocked
	wBlocked := httptest.NewRecorder()
	handler.ServeHTTP(wBlocked, req)
	if wBlocked.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429 for blocked request, got %d", wBlocked.Code)
	}
	if retryAfter := wBlocked.Header().Get("Retry-After"); retryAfter == "" {
		t.Fatalf("expected non-empty Retry-After header")
	}

	// Wait for window to elapse
	time.Sleep(window + 10*time.Millisecond)

	// Next request after window elapsed should succeed
	wAfter := httptest.NewRecorder()
	handler.ServeHTTP(wAfter, req)
	if wAfter.Code != http.StatusOK {
		t.Fatalf("expected status 200 after window expiry, got %d", wAfter.Code)
	}
}

func TestExtractClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		expected   string
	}{
		{
			name:       "IPv4 with port",
			remoteAddr: "192.168.1.10:8080",
			xff:        "",
			expected:   "192.168.1.10",
		},
		{
			name:       "IPv4 without port",
			remoteAddr: "192.168.1.10",
			xff:        "",
			expected:   "192.168.1.10",
		},
		{
			name:       "IPv6 with port",
			remoteAddr: "[::1]:54321",
			xff:        "",
			expected:   "::1",
		},
		{
			name:       "IPv6 without port",
			remoteAddr: "::1",
			xff:        "",
			expected:   "::1",
		},
		{
			name:       "X-Forwarded-For header is ignored to prevent spoofing",
			remoteAddr: "127.0.0.1:80",
			xff:        "203.0.113.195, 70.41.3.18",
			expected:   "127.0.0.1",
		},
		{
			name:       "X-Forwarded-For with port is ignored",
			remoteAddr: "192.168.1.10:8080",
			xff:        "203.0.113.195:443",
			expected:   "192.168.1.10",
		},
		{
			name:       "Empty remote address defaults to localhost",
			remoteAddr: "",
			xff:        "",
			expected:   "127.0.0.1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			ip := ExtractClientIP(req)
			if ip != tc.expected {
				t.Fatalf("expected IP %s, got %s", tc.expected, ip)
			}
		})
	}
}

func TestSlidingWindow_RedisStore_Fallback(t *testing.T) {
	// RedisStore with nil client falls back cleanly to in-memory store
	store := NewRedisStore(nil)
	ctx := context.Background()

	allowed, rem, _, err := store.Allow(ctx, "test-key", 2, time.Second)
	if err != nil || !allowed || rem != 1 {
		t.Fatalf("expected fallback allow=true, rem=1; got allowed=%v, rem=%d, err=%v", allowed, rem, err)
	}

	allowed, rem, _, err = store.Allow(ctx, "test-key", 2, time.Second)
	if err != nil || !allowed || rem != 0 {
		t.Fatalf("expected fallback allow=true, rem=0; got allowed=%v, rem=%d, err=%v", allowed, rem, err)
	}

	allowed, rem, _, err = store.Allow(ctx, "test-key", 2, time.Second)
	if err != nil || allowed || rem != 0 {
		t.Fatalf("expected fallback allow=false, rem=0; got allowed=%v, rem=%d, err=%v", allowed, rem, err)
	}
}

func TestSlidingWindow_Concurrency(t *testing.T) {
	store := NewMemoryStore()
	limit := 10
	window := 100 * time.Millisecond
	ctx := context.Background()

	var wg sync.WaitGroup
	allowedCount := 0
	var mu sync.Mutex

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, _, _, err := store.Allow(ctx, "concurrent-key", limit, window)
			if err == nil && allowed {
				mu.Lock()
				allowedCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	if allowedCount != limit {
		t.Fatalf("expected exactly %d allowed requests under concurrency, got %d", limit, allowedCount)
	}
}

func TestMemoryStore_PrunesExpiredKeys(t *testing.T) {
	store := NewMemoryStore()
	key := "192.168.1.100"
	ctx := context.Background()

	// Make one request with 10ms window
	allowed, _, _, err := store.Allow(ctx, key, 5, 10*time.Millisecond)
	if err != nil || !allowed {
		t.Fatalf("expected allowed request, got allowed=%v, err=%v", allowed, err)
	}

	time.Sleep(20 * time.Millisecond)

	// Next request after window expiration should prune expired timestamps
	allowed, remaining, _, err := store.Allow(ctx, key, 5, 10*time.Millisecond)
	if err != nil || !allowed || remaining != 4 {
		t.Fatalf("expected fresh window with remaining=4, got remaining=%d", remaining)
	}

	// Verify that expired keys with len(valid) == 0 are removed from entries map
	expiredKey := "192.168.1.200"
	store.mu.Lock()
	store.entries[expiredKey] = []time.Time{time.Now().Add(-time.Hour)}
	store.mu.Unlock()

	// When limit is 0, expired timestamps are filtered out, len(valid) == 0,
	// and the key must be pruned/deleted from store.entries map
	allowed, _, _, err = store.Allow(ctx, expiredKey, 0, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Fatalf("expected allowed=false when limit=0")
	}

	store.mu.Lock()
	_, exists := store.entries[expiredKey]
	store.mu.Unlock()

	if exists {
		t.Fatalf("expected expired key %q to be pruned/deleted from memory map", expiredKey)
	}
}
