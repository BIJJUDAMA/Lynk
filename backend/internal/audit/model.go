// Package audit provides tamper-evident audit logging infrastructure for Lynk.
// 100% pure 7-bit ASCII
package audit

import (
	"time"

	"github.com/google/uuid"
)

// Entry represents an immutable audit log record.
type Entry struct {
	ID         uuid.UUID      `json:"id"`
	ActorID    string         `json:"actor_id"`
	Action     string         `json:"action"`
	TargetType string         `json:"target_type"`
	TargetID   string         `json:"target_id"`
	Metadata   map[string]any `json:"metadata"`
	IPAddress  string         `json:"ip_address,omitempty"`
	TraceID    string         `json:"trace_id,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

// Filter specifies query criteria for keyset-paginated audit trail retrieval.
type Filter struct {
	ActorID    string
	Action     string
	TargetType string
	TargetID   string
	CursorTime *time.Time
	CursorID   *uuid.UUID
	Limit      int
}
