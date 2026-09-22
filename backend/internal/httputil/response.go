package httputil

import (
	"encoding/json"
	"log/slog"
	"net/http"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

type Response[T any] struct {
	Success bool      `json:"success"`
	Data    T         `json:"data"`
	Error   *APIError `json:"error"`
}

func WriteSuccess[T any](w http.ResponseWriter, status int, data T) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Response[T]{
		Success: true,
		Data:    data,
		Error:   nil,
	})
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string, internalErr error) {
	var reqID string
	if r != nil {
		reqID = chimiddleware.GetReqID(r.Context())
	}
	if internalErr != nil {
		path := ""
		if r != nil {
			path = r.URL.Path
		}
		slog.Error("request failed", "status", status, "code", code, "message", message, "error", internalErr, "request_id", reqID, "path", path)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Response[interface{}]{
		Success: false,
		Data:    nil,
		Error: &APIError{
			Code:      code,
			Message:   message,
			RequestID: reqID,
		},
	})
}

// WriteBadRequest writes a 400 Bad Request response with the given code and message.
func WriteBadRequest(w http.ResponseWriter, r *http.Request, code, message string) {
	WriteError(w, r, http.StatusBadRequest, code, message, nil)
}

// WriteUnauthorized writes a 401 Unauthorized response with the given code and message.
func WriteUnauthorized(w http.ResponseWriter, r *http.Request, code, message string) {
	WriteError(w, r, http.StatusUnauthorized, code, message, nil)
}

// WriteForbidden writes a 403 Forbidden response with the given code and message.
func WriteForbidden(w http.ResponseWriter, r *http.Request, code, message string) {
	WriteError(w, r, http.StatusForbidden, code, message, nil)
}

// WriteNotFound writes a 404 Not Found response with the given code and message.
func WriteNotFound(w http.ResponseWriter, r *http.Request, code, message string) {
	WriteError(w, r, http.StatusNotFound, code, message, nil)
}

// WriteConflict writes a 409 Conflict response with the given code and message.
func WriteConflict(w http.ResponseWriter, r *http.Request, code, message string) {
	WriteError(w, r, http.StatusConflict, code, message, nil)
}

// WriteInternalServerError writes a 500 Internal Server Error response masking the raw error from the client while logging it.
func WriteInternalServerError(w http.ResponseWriter, r *http.Request, internalErr error) {
	WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred. Please try again later.", internalErr)
}
