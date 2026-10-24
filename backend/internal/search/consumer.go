// Package search provides AMQP consumers for asynchronous index updates.
// 100% pure 7-bit ASCII
package search

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// IndexEventPayload describes the event payload received from the transactional outbox.
type IndexEventPayload struct {
	ID                string `json:"id"`
	Slug              string `json:"slug"`
	Title             string `json:"title"`
	Description       string `json:"description"`
	Category          string `json:"category"`
	Visibility        string `json:"visibility"`
	RecruitmentStatus string `json:"recruitment_status"`
	OwnerMemberID     string `json:"owner_member_id"`
	Action            string `json:"action"` // "create", "update", "delete"
}

// ProcessDelivery processes an inbound RabbitMQ delivery for search indexing.
func ProcessDelivery(ctx context.Context, svc Service, d amqp.Delivery, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}

	var payload IndexEventPayload
	if err := json.Unmarshal(d.Body, &payload); err != nil {
		logger.Error("failed to unmarshal search index payload", "error", err, "body", string(d.Body))
		// Bad payload should not be retried indefinitely
		return nil
	}

	if payload.ID == "" {
		return fmt.Errorf("search index payload missing project id")
	}

	routingKey := d.RoutingKey
	logger.Debug("processing search index delivery", "routing_key", routingKey, "project_id", payload.ID, "action", payload.Action)

	if payload.Action == "delete" || routingKey == "project.deleted" {
		if err := svc.DeleteProject(ctx, payload.ID); err != nil {
			return fmt.Errorf("delete project from search index: %w", err)
		}
		logger.Info("deleted project from search index", "id", payload.ID)
		return nil
	}

	doc := ProjectDocument{
		ID:                payload.ID,
		Slug:              payload.Slug,
		Title:             payload.Title,
		Description:       payload.Description,
		Category:          payload.Category,
		Visibility:        payload.Visibility,
		RecruitmentStatus: payload.RecruitmentStatus,
		OwnerMemberID:     payload.OwnerMemberID,
		CreatedAt:         time.Now().Unix(),
	}

	if err := svc.IndexProject(ctx, doc); err != nil {
		return fmt.Errorf("index project document: %w", err)
	}

	logger.Info("indexed project into search engine", "id", doc.ID, "slug", doc.Slug)
	return nil
}
