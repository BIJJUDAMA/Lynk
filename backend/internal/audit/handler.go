// Package audit HTTP handler.
// 100% pure 7-bit ASCII
package audit

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/lynk/backend/internal/httputil"
)

// Handler serves administrative audit query endpoints.
type Handler struct {
	svc *Service
}

// NewHandler creates an audit HTTP handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// QueryLogs handles GET /api/v1/audit/logs.
func (h *Handler) QueryLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := Filter{
		ActorID:    q.Get("actor_id"),
		Action:     q.Get("action"),
		TargetType: q.Get("target_type"),
		TargetID:   q.Get("target_id"),
	}

	if limitStr := q.Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}

	if cursorTimeStr := q.Get("cursor_time"); cursorTimeStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, cursorTimeStr); err == nil {
			filter.CursorTime = &t
		} else if t, err := time.Parse(time.RFC3339, cursorTimeStr); err == nil {
			filter.CursorTime = &t
		}
	}

	if cursorIDStr := q.Get("cursor_id"); cursorIDStr != "" {
		if id, err := uuid.Parse(cursorIDStr); err == nil {
			filter.CursorID = &id
		}
	}

	entries, err := h.svc.Query(r.Context(), filter)
	if err != nil {
		httputil.WriteError(w, r, http.StatusInternalServerError, "AUDIT_QUERY_FAILED", "Failed to query audit logs", err)
		return
	}

	if entries == nil {
		entries = make([]*Entry, 0)
	}

	httputil.WriteSuccess(w, http.StatusOK, entries)
}
