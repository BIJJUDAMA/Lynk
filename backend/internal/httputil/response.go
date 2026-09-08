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
