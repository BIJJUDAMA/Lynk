package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPprofAuthMiddleware_Rejection(t *testing.T) {
	handler := pprofAuthMiddleware("secret-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Unauthorized request
	req := httptest.NewRequest("GET", "/debug/pprof/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
	}

	// Authorized request
	reqAuth := httptest.NewRequest("GET", "/debug/pprof/", nil)
	reqAuth.Header.Set("Authorization", "Bearer secret-token")
	recAuth := httptest.NewRecorder()
	handler.ServeHTTP(recAuth, reqAuth)

	if recAuth.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", recAuth.Code)
	}
}

func TestNewPprofMux_Endpoints(t *testing.T) {
	mux := NewPprofMux("secret")
	req := httptest.NewRequest("GET", "/debug/pprof/", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from pprof index, got %d", rec.Code)
	}
}
