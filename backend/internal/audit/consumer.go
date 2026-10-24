// Package audit queue consumer.
// 100% pure 7-bit ASCII
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ProcessDelivery decodes an AMQP delivery from events.audit and persists it via the audit service.
func ProcessDelivery(ctx context.Context, svc *Service, d amqp.Delivery, logger *slog.Logger) error {
	var entry Entry
	if err := json.Unmarshal(d.Body, &entry); err != nil {
		return fmt.Errorf("failed to unmarshal audit payload: %w", err)
	}

	// Extract distributed trace ID from AMQP headers if present
	if d.Headers != nil {
		if tp, ok := d.Headers["traceparent"].(string); ok && tp != "" {
			entry.TraceID = tp
		} else if tid, ok := d.Headers["trace_id"].(string); ok && tid != "" {
			entry.TraceID = tid
		}
	}

	if err := svc.Record(ctx, &entry); err != nil {
		return fmt.Errorf("failed to persist audit entry: %w", err)
	}

	return nil
}
