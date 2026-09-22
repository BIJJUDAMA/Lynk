package event

import (
	"time"
)

// Topic defines standard domain event types.
type Topic string

const (
	TopicJobCreated      Topic = "job.created"
	TopicJobClosed       Topic = "job.closed"
	TopicApplicationSent Topic = "application.sent"
	TopicContractCreated Topic = "contract.created"
	TopicContractDone    Topic = "contract.completed"
	TopicReviewSubmitted Topic = "review.submitted"
	TopicProfileUpdated  Topic = "profile.updated"
)

// Event represents a standardized domain event payload.
type Event struct {
	Type      Topic          `json:"type"`
	EntityID  string         `json:"entity_id"`
	ActorID   string         `json:"actor_id"`
	Timestamp time.Time      `json:"timestamp"`
	Payload   map[string]any `json:"payload,omitempty"`
}
