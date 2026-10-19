package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/cache"
)

func TestIdempotency_NilCacherPassThrough(t *testing.T) {
	mw := Idempotency(nil, 24*time.Hour)
	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(`{"name":"test"}`))
	req.Header.Set("Idempotency-Key", "test-key-nil-cacher")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatalf("expected handler to be called when cacher is nil")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestIdempotency_NonMutatingMethodsBypassed(t *testing.T) {
	memCache := cache.NewMemoryCache()
	mw := Idempotency(memCache, time.Hour)

	methods := []string{http.MethodGet, http.MethodHead, http.MethodOptions}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			var callCount int32
			handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&callCount, 1)
				w.Header().Set("X-Custom", "value")
				w.WriteHeader(http.StatusOK)
				if r.Method != http.MethodHead {
					_, _ = w.Write([]byte("read-result"))
				}
			}))

			// Request 1
			req1 := httptest.NewRequest(method, "/items", nil)
			req1.Header.Set("Idempotency-Key", "bypass-key-1")
			rec1 := httptest.NewRecorder()
			handler.ServeHTTP(rec1, req1)

			if rec1.Header().Get("Idempotent-Replayed") == "true" {
				t.Fatalf("Idempotent-Replayed header must not be set on %s", method)
			}

			// Request 2 with same key
			req2 := httptest.NewRequest(method, "/items", nil)
			req2.Header.Set("Idempotency-Key", "bypass-key-1")
			rec2 := httptest.NewRecorder()
			handler.ServeHTTP(rec2, req2)

			if atomic.LoadInt32(&callCount) != 2 {
				t.Fatalf("expected 2 handler invocations for bypassed method %s, got %d", method, atomic.LoadInt32(&callCount))
			}
			if rec2.Header().Get("Idempotent-Replayed") == "true" {
				t.Fatalf("Idempotent-Replayed header must not be set on repeated %s", method)
			}
		})
	}
}

func TestIdempotency_MissingOrEmptyHeader(t *testing.T) {
	memCache := cache.NewMemoryCache()
	mw := Idempotency(memCache, time.Hour)

	testCases := []struct {
		name   string
		header string
	}{
		{name: "missing header", header: ""},
		{name: "whitespace only", header: "    "},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var callCount int32
			handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&callCount, 1)
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"id":"item-1"}`))
			}))

			req1 := httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(`{}`))
			if tc.header != "" {
				req1.Header.Set("Idempotency-Key", tc.header)
			}
			rec1 := httptest.NewRecorder()
			handler.ServeHTTP(rec1, req1)

			req2 := httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(`{}`))
			if tc.header != "" {
				req2.Header.Set("Idempotency-Key", tc.header)
			}
			rec2 := httptest.NewRecorder()
			handler.ServeHTTP(rec2, req2)

			if atomic.LoadInt32(&callCount) != 2 {
				t.Fatalf("expected 2 calls when header is missing/empty, got %d", atomic.LoadInt32(&callCount))
			}
			if rec1.Header().Get("Idempotent-Replayed") != "" || rec2.Header().Get("Idempotent-Replayed") != "" {
				t.Fatalf("Idempotent-Replayed header should not be set")
			}
		})
	}
}

func TestIdempotency_KeyLengthValidation(t *testing.T) {
	memCache := cache.NewMemoryCache()
	mw := Idempotency(memCache, time.Hour)

	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	longKey := strings.Repeat("x", 129)
	req := httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(`{}`))
	req.Header.Set("Idempotency-Key", longKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if called {
		t.Fatalf("handler must not be executed when key length > 128")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d", rec.Code)
	}

	var errBody map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("failed to decode error body: %v", err)
	}
	if !strings.Contains(errBody["error"], "128 characters") {
		t.Fatalf("expected error mentioning 128 characters, got: %v", errBody["error"])
	}
}

func TestIdempotency_SuccessAndReplay(t *testing.T) {
	memCache := cache.NewMemoryCache()
	// TTL <= 0 tests the default 24h fallback
	mw := Idempotency(memCache, 0)

	var callCount int32
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Entity-ID", "ent-999")
		w.Header().Set("Connection", "keep-alive") // Hop-by-hop header to test stripping
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"created","id":"ent-999"}`))
	}))

	key := "test-idemp-key-normal"

	// 1. Initial POST execution
	req1 := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(`{"item":"book"}`))
	req1.Header.Set("Idempotency-Key", key)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created on first request, got %d", rec1.Code)
	}
	if rec1.Header().Get("Idempotent-Replayed") == "true" {
		t.Fatalf("first execution must not have Idempotent-Replayed: true")
	}
	if rec1.Header().Get("X-Entity-ID") != "ent-999" {
		t.Fatalf("expected X-Entity-ID header on first request, got: %s", rec1.Header().Get("X-Entity-ID"))
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Fatalf("expected handler callCount = 1, got %d", atomic.LoadInt32(&callCount))
	}

	// 2. Exact replay
	req2 := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(`{"item":"book"}`))
	req2.Header.Set("Idempotency-Key", key)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created on replayed request, got %d", rec2.Code)
	}
	if rec2.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replayed request must have Idempotent-Replayed: true")
	}
	if rec2.Header().Get("X-Entity-ID") != "ent-999" {
		t.Fatalf("expected X-Entity-ID header on replayed request, got: %s", rec2.Header().Get("X-Entity-ID"))
	}
	// Hop-by-hop headers should not be replayed
	if rec2.Header().Get("Connection") != "" {
		t.Fatalf("hop-by-hop header 'Connection' must be stripped from replay")
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Fatalf("handler must NOT be invoked again on replay, got count %d", atomic.LoadInt32(&callCount))
	}
	if rec1.Body.String() != rec2.Body.String() {
		t.Fatalf("replayed body must match exactly: %s vs %s", rec1.Body.String(), rec2.Body.String())
	}
}

func TestIdempotency_ConcurrentInProgress409Conflict(t *testing.T) {
	memCache := cache.NewMemoryCache()
	mw := Idempotency(memCache, time.Hour)

	inProgressGate := make(chan struct{})
	continueGate := make(chan struct{})
	var callCount int32

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		close(inProgressGate) // Signal that first request is actively in handler
		<-continueGate        // Wait until concurrent request test is completed
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"result":"first-done"}`))
	}))

	key := "test-idemp-key-concurrent"

	var wg sync.WaitGroup
	wg.Add(1)
	var rec1 *httptest.ResponseRecorder

	go func() {
		defer wg.Done()
		req1 := httptest.NewRequest(http.MethodPost, "/transfer", strings.NewReader(`{}`))
		req1.Header.Set("Idempotency-Key", key)
		rec1 = httptest.NewRecorder()
		handler.ServeHTTP(rec1, req1)
	}()

	// Wait until request 1 has locked the key and entered handler
	<-inProgressGate

	// Request 2 arrives while Request 1 is still in progress
	req2 := httptest.NewRequest(http.MethodPost, "/transfer", strings.NewReader(`{}`))
	req2.Header.Set("Idempotency-Key", key)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for in-progress request, got %d body=%s", rec2.Code, rec2.Body.String())
	}
	var conflictResp map[string]string
	if err := json.Unmarshal(rec2.Body.Bytes(), &conflictResp); err != nil {
		t.Fatalf("failed to parse conflict response JSON: %v", err)
	}
	if conflictResp["error"] != "Request with this Idempotency-Key is currently in progress" {
		t.Fatalf("unexpected error message: %s", conflictResp["error"])
	}

	// Now unblock request 1
	close(continueGate)
	wg.Wait()

	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected request 1 to complete with 201 Created, got %d", rec1.Code)
	}

	// Request 3 now replays the completed request
	req3 := httptest.NewRequest(http.MethodPost, "/transfer", strings.NewReader(`{}`))
	req3.Header.Set("Idempotency-Key", key)
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusCreated {
		t.Fatalf("expected request 3 to replay with 201 Created, got %d", rec3.Code)
	}
	if rec3.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("expected Idempotent-Replayed: true on request 3")
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Fatalf("expected handler to only have executed once, got %d", atomic.LoadInt32(&callCount))
	}
}

func TestIdempotency_ErrorEvictionAllowsRetry(t *testing.T) {
	memCache := cache.NewMemoryCache()
	mw := Idempotency(memCache, time.Hour)

	var attemptCount int32
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		att := atomic.AddInt32(&attemptCount, 1)
		if att == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"transient failure"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))

	key := "test-idemp-key-retry"

	// First attempt fails with 500
	req1 := httptest.NewRequest(http.MethodPost, "/action", strings.NewReader(`{}`))
	req1.Header.Set("Idempotency-Key", key)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on first attempt, got %d", rec1.Code)
	}

	// Verify key was purged from cache by verifying that second attempt is allowed and executes handler
	req2 := httptest.NewRequest(http.MethodPost, "/action", strings.NewReader(`{}`))
	req2.Header.Set("Idempotency-Key", key)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on retry attempt, got %d", rec2.Code)
	}
	if rec2.Header().Get("Idempotent-Replayed") == "true" {
		t.Fatalf("retry attempt must not be marked as replayed")
	}
	if atomic.LoadInt32(&attemptCount) != 2 {
		t.Fatalf("expected handler to be invoked twice (once failed, once successful), got %d", atomic.LoadInt32(&attemptCount))
	}
}

func TestIdempotency_PanicEvictionAllowsRetry(t *testing.T) {
	memCache := cache.NewMemoryCache()
	mw := Idempotency(memCache, time.Hour)

	var attemptCount int32
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		att := atomic.AddInt32(&attemptCount, 1)
		if att == 1 {
			panic("unexpected failure")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"recovered"}`))
	}))

	key := "test-idemp-key-panic"

	// Attempt 1: panics
	req1 := httptest.NewRequest(http.MethodPost, "/action", strings.NewReader(`{}`))
	req1.Header.Set("Idempotency-Key", key)
	rec1 := httptest.NewRecorder()

	func() {
		defer func() {
			_ = recover()
		}()
		handler.ServeHTTP(rec1, req1)
	}()

	// Attempt 2: key must be purged, so second request can execute cleanly
	req2 := httptest.NewRequest(http.MethodPost, "/action", strings.NewReader(`{}`))
	req2.Header.Set("Idempotency-Key", key)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on retry after panic, got %d", rec2.Code)
	}
	if atomic.LoadInt32(&attemptCount) != 2 {
		t.Fatalf("expected handler to execute twice, got %d", atomic.LoadInt32(&attemptCount))
	}
}

func TestIdempotency_UserIsolation(t *testing.T) {
	memCache := cache.NewMemoryCache()
	mw := Idempotency(memCache, time.Hour)

	userExecutions := make(map[string]int)
	var mu sync.Mutex

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserContext(r.Context())
		if err != nil {
			t.Fatalf("expected user claims in context: %v", err)
		}
		mu.Lock()
		userExecutions[claims.UserID]++
		mu.Unlock()

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"user_id": claims.UserID,
		})
	}))

	sameKey := "shared-idempotency-key-100"

	// User A first request
	ctxA := auth.WithUserContext(context.Background(), &auth.UserClaims{UserID: "user-A"})
	reqA1 := httptest.NewRequest(http.MethodPost, "/resource", strings.NewReader(`{}`)).WithContext(ctxA)
	reqA1.Header.Set("Idempotency-Key", sameKey)
	recA1 := httptest.NewRecorder()
	handler.ServeHTTP(recA1, reqA1)

	if recA1.Code != http.StatusCreated {
		t.Fatalf("expected User A request 1 to return 201, got %d", recA1.Code)
	}

	// User B with IDENTICAL key must execute independently (isolated by userID)
	ctxB := auth.WithUserContext(context.Background(), &auth.UserClaims{UserID: "user-B"})
	reqB1 := httptest.NewRequest(http.MethodPost, "/resource", strings.NewReader(`{}`)).WithContext(ctxB)
	reqB1.Header.Set("Idempotency-Key", sameKey)
	recB1 := httptest.NewRecorder()
	handler.ServeHTTP(recB1, reqB1)

	if recB1.Code != http.StatusCreated {
		t.Fatalf("expected User B request 1 to return 201, got %d", recB1.Code)
	}
	if recB1.Header().Get("Idempotent-Replayed") == "true" {
		t.Fatalf("User B must not replay User A's response")
	}

	mu.Lock()
	countA := userExecutions["user-A"]
	countB := userExecutions["user-B"]
	mu.Unlock()

	if countA != 1 || countB != 1 {
		t.Fatalf("expected exactly 1 execution for both User A and User B, got A=%d B=%d", countA, countB)
	}

	// Replay for User A
	reqA2 := httptest.NewRequest(http.MethodPost, "/resource", strings.NewReader(`{}`)).WithContext(ctxA)
	reqA2.Header.Set("Idempotency-Key", sameKey)
	recA2 := httptest.NewRecorder()
	handler.ServeHTTP(recA2, reqA2)

	if recA2.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("expected Idempotent-Replayed: true for User A replay")
	}
	var resA2 map[string]string
	_ = json.Unmarshal(recA2.Body.Bytes(), &resA2)
	if resA2["user_id"] != "user-A" {
		t.Fatalf("expected User A response, got %s", resA2["user_id"])
	}

	// Replay for User B
	reqB2 := httptest.NewRequest(http.MethodPost, "/resource", strings.NewReader(`{}`)).WithContext(ctxB)
	reqB2.Header.Set("Idempotency-Key", sameKey)
	recB2 := httptest.NewRecorder()
	handler.ServeHTTP(recB2, reqB2)

	if recB2.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("expected Idempotent-Replayed: true for User B replay")
	}
	var resB2 map[string]string
	_ = json.Unmarshal(recB2.Body.Bytes(), &resB2)
	if resB2["user_id"] != "user-B" {
		t.Fatalf("expected User B response, got %s", resB2["user_id"])
	}

	// Ensure handler was not executed again
	mu.Lock()
	finalCountA := userExecutions["user-A"]
	finalCountB := userExecutions["user-B"]
	mu.Unlock()

	if finalCountA != 1 || finalCountB != 1 {
		t.Fatalf("expected no additional executions on replays, got A=%d B=%d", finalCountA, finalCountB)
	}
}

func TestIdempotency_ASCIICompliance(t *testing.T) {
	files := []string{
		"idempotency.go",
		"idempotency_test.go",
		"../../cmd/api/main.go",
		"../../cmd/api/main_test.go",
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read %s: %v", file, err)
		}
		for i, b := range data {
			if b >= 128 {
				t.Fatalf("non-ASCII byte %d at index %d in %s", b, i, file)
			}
		}
	}
}
