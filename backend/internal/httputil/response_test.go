package httputil

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func TestWriteSuccess(t *testing.T) {
	type samplePayload struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	payload := samplePayload{
		ID:   "user-123",
		Name: "Alice",
	}

	w := httptest.NewRecorder()
	WriteSuccess(w, http.StatusOK, payload)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	var resp Response[samplePayload]
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success to be true")
	}
	if resp.Error != nil {
		t.Errorf("expected error to be nil, got %+v", resp.Error)
	}
	if resp.Data.ID != "user-123" || resp.Data.Name != "Alice" {
		t.Errorf("unexpected data: %+v", resp.Data)
	}
}

func TestWriteSuccess_NilData(t *testing.T) {
	w := httptest.NewRecorder()
	WriteSuccess[any](w, http.StatusCreated, nil)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var resp Response[any]
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success to be true")
	}
	if resp.Data != nil {
		t.Errorf("expected data to be nil, got %v", resp.Data)
	}
}

func TestWriteError_WithRequestID(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	ctx := context.WithValue(r.Context(), chimiddleware.RequestIDKey, "req-xyz-789")
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()
	WriteError(w, r, http.StatusBadRequest, "INVALID_INPUT", "Input validation failed", errors.New("underlying syntax error"))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	var resp Response[any]
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Success {
		t.Errorf("expected success to be false")
	}
	if resp.Data != nil {
		t.Errorf("expected data to be nil, got %v", resp.Data)
	}
	if resp.Error == nil {
		t.Fatalf("expected error payload, got nil")
	}
	if resp.Error.Code != "INVALID_INPUT" {
		t.Errorf("expected code INVALID_INPUT, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "Input validation failed" {
		t.Errorf("expected message 'Input validation failed', got %s", resp.Error.Message)
	}
	if resp.Error.RequestID != "req-xyz-789" {
		t.Errorf("expected request_id 'req-xyz-789', got %s", resp.Error.RequestID)
	}
}

func TestWriteError_WithoutRequestID_NilInternalErr(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/me", nil)

	w := httptest.NewRecorder()
	WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials", nil)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var resp Response[any]
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Success {
		t.Errorf("expected success to be false")
	}
	if resp.Error == nil {
		t.Fatalf("expected error payload, got nil")
	}
	if resp.Error.Code != "UNAUTHORIZED" {
		t.Errorf("expected code UNAUTHORIZED, got %s", resp.Error.Code)
	}
	if resp.Error.RequestID != "" {
		t.Errorf("expected empty request_id, got %s", resp.Error.RequestID)
	}
}

func TestWriteError_NilRequest(t *testing.T) {
	w := httptest.NewRecorder()
	// Should not panic on nil request
	WriteError(w, nil, http.StatusInternalServerError, "INTERNAL_ERROR", "Unexpected server error", errors.New("db down"))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}
