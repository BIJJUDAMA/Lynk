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

func TestWriteBadRequest(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", nil)
	ctx := context.WithValue(r.Context(), chimiddleware.RequestIDKey, "req-bad-req-1")
	r = r.WithContext(ctx)

	WriteBadRequest(w, r, "INVALID_INPUT", "Username cannot be empty")

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
	if resp.Error.Message != "Username cannot be empty" {
		t.Errorf("expected message 'Username cannot be empty', got %s", resp.Error.Message)
	}
	if resp.Error.RequestID != "req-bad-req-1" {
		t.Errorf("expected request_id req-bad-req-1, got %s", resp.Error.RequestID)
	}
}

func TestWriteUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)

	WriteUnauthorized(w, r, "UNAUTHORIZED", "Missing session token")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
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
	if resp.Error == nil {
		t.Fatalf("expected error payload, got nil")
	}
	if resp.Error.Code != "UNAUTHORIZED" {
		t.Errorf("expected code UNAUTHORIZED, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "Missing session token" {
		t.Errorf("expected message 'Missing session token', got %s", resp.Error.Message)
	}
}

func TestWriteForbidden(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/jobs/123", nil)

	WriteForbidden(w, r, "FORBIDDEN", "You do not have permission to delete this job")

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, w.Code)
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
	if resp.Error == nil {
		t.Fatalf("expected error payload, got nil")
	}
	if resp.Error.Code != "FORBIDDEN" {
		t.Errorf("expected code FORBIDDEN, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "You do not have permission to delete this job" {
		t.Errorf("unexpected message: %s", resp.Error.Message)
	}
}

func TestWriteNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/nonexistent", nil)

	WriteNotFound(w, r, "JOB_NOT_FOUND", "The requested job was not found")

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, w.Code)
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
	if resp.Error == nil {
		t.Fatalf("expected error payload, got nil")
	}
	if resp.Error.Code != "JOB_NOT_FOUND" {
		t.Errorf("expected code JOB_NOT_FOUND, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "The requested job was not found" {
		t.Errorf("unexpected message: %s", resp.Error.Message)
	}
}

func TestWriteConflict(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/applications", nil)

	WriteConflict(w, r, "APPLICATION_EXISTS", "You have already applied to this job")

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, w.Code)
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
	if resp.Error == nil {
		t.Fatalf("expected error payload, got nil")
	}
	if resp.Error.Code != "APPLICATION_EXISTS" {
		t.Errorf("expected code APPLICATION_EXISTS, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "You have already applied to this job" {
		t.Errorf("unexpected message: %s", resp.Error.Message)
	}
}

func TestWriteInternalServerError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	ctx := context.WithValue(r.Context(), chimiddleware.RequestIDKey, "req-500-test")
	r = r.WithContext(ctx)

	WriteInternalServerError(w, r, errors.New("database connection timeout"))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
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
	if resp.Error == nil {
		t.Fatalf("expected error payload, got nil")
	}
	if resp.Error.Code != "INTERNAL_ERROR" {
		t.Errorf("expected code INTERNAL_ERROR, got %s", resp.Error.Code)
	}
	// Raw error details must be masked from client
	expectedMsg := "An unexpected error occurred. Please try again later."
	if resp.Error.Message != expectedMsg {
		t.Errorf("expected generic message %q, got %q", expectedMsg, resp.Error.Message)
	}
	if resp.Error.RequestID != "req-500-test" {
		t.Errorf("expected request_id req-500-test, got %s", resp.Error.RequestID)
	}
}
