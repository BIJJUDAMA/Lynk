package httpx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/httpx"
)

func TestDefaultAuthMiddleware(t *testing.T) {
	customMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Custom-Auth", "true")
			next.ServeHTTP(w, r)
		})
	}

	mw := httpx.DefaultAuthMiddleware(customMW)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Custom-Auth") != "true" {
		t.Fatalf("expected custom auth middleware to be used")
	}

	fallbackMW := httpx.DefaultAuthMiddleware(nil)
	if fallbackMW == nil {
		t.Fatalf("expected non-nil fallback middleware")
	}
}

func TestTestAuth(t *testing.T) {
	claims := &auth.UserClaims{
		UserID: "usr-test-123",
		Email:  "student@campus.edu",
	}

	mw := httpx.TestAuth(claims)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	var extractedClaims *auth.UserClaims
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		extractedClaims, _ = auth.GetUserContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if extractedClaims == nil || extractedClaims.UserID != "usr-test-123" {
		t.Fatalf("expected UserID 'usr-test-123', got %+v", extractedClaims)
	}
}

func TestRequireAuthFromContext(t *testing.T) {
	mw := httpx.RequireAuthFromContext()

	// 1. Without context claims -> 401
	recNoAuth := httptest.NewRecorder()
	reqNoAuth := httptest.NewRequest(http.MethodGet, "/test", nil)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(recNoAuth, reqNoAuth)
	if recNoAuth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized without claims, got %d", recNoAuth.Code)
	}

	// 2. With context claims -> 200
	recAuth := httptest.NewRecorder()
	reqAuth := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx := auth.WithUserContext(context.Background(), &auth.UserClaims{UserID: "usr-456"})
	reqAuth = reqAuth.WithContext(ctx)
	handler.ServeHTTP(recAuth, reqAuth)
	if recAuth.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with claims, got %d", recAuth.Code)
	}
}
