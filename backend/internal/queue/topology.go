// Package queue topology declaration.
// 100% pure 7-bit ASCII
package queue

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	// Exchanges
	ExchangeEvents = "lynk.events" // Topic exchange for all platform domain events
	ExchangeDLX    = "lynk.dlx"    // Direct exchange for dead-letter isolation

	// Queues
	QueueAudit         = "events.audit"
	QueueAIEmbeddings  = "tasks.ai.embeddings"
	QueueNotifications = "tasks.notifications"
	QueueDeadLetter    = "tasks.dead_letter"

	// Routing Keys
	RoutingAuditPrefix        = "audit."
	RoutingAIEmbeddingPrefix  = "ai.embedding."
	RoutingNotificationPrefix = "notification."
	RoutingDeadLetterAudit    = "events.audit.dead"
	RoutingDeadLetterAI       = "tasks.ai.dead"
	RoutingDeadLetterNotif    = "tasks.notifications.dead"
)

// DeclareTopology sets up durable exchanges, queues, and dead-letter pipelines.
func (c *Client) DeclareTopology() error {
	ch, err := c.Channel()
	if err != nil {
		return fmt.Errorf("topology setup failed to get channel: %w", err)
	}
	defer c.ReleaseChannel(ch)

	// 1. Declare Main Topic Exchange (Durable)
	if err := ch.ExchangeDeclare(
		ExchangeEvents,
		"topic",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	); err != nil {
		return fmt.Errorf("failed to declare exchange %s: %w", ExchangeEvents, err)
	}

	// 2. Declare Dead Letter Exchange (Durable)
	if err := ch.ExchangeDeclare(
		ExchangeDLX,
		"direct",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	); err != nil {
		return fmt.Errorf("failed to declare exchange %s: %w", ExchangeDLX, err)
	}

	// 3. Declare Global Dead Letter Queue
	if _, err := ch.QueueDeclare(
		QueueDeadLetter,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	); err != nil {
		return fmt.Errorf("failed to declare dead letter queue: %w", err)
	}

	// Bind DLQ to DLX for all dead-letter routing keys
	for _, key := range []string{RoutingDeadLetterAudit, RoutingDeadLetterAI, RoutingDeadLetterNotif} {
		if err := ch.QueueBind(QueueDeadLetter, key, ExchangeDLX, false, nil); err != nil {
			return fmt.Errorf("failed to bind DLQ for key %s: %w", key, err)
		}
	}

	// 4. Declare Queues with Dead Letter routing
	queues := []struct {
		name       string
		routingKey string
		dlKey      string
	}{
		{
			name:       QueueAudit,
			routingKey: "audit.*",
			dlKey:      RoutingDeadLetterAudit,
		},
		{
			name:       QueueAIEmbeddings,
			routingKey: "ai.embedding.*",
			dlKey:      RoutingDeadLetterAI,
		},
		{
			name:       QueueNotifications,
			routingKey: "notification.*",
			dlKey:      RoutingDeadLetterNotif,
		},
	}

	for _, q := range queues {
		args := amqp.Table{
			"x-dead-letter-exchange":    ExchangeDLX,
			"x-dead-letter-routing-key": q.dlKey,
			"x-queue-type":              "classic", // or quorum where cluster is present
		}

		if _, err := ch.QueueDeclare(
			q.name,
			true,  // durable
			false, // auto-delete
			false, // exclusive
			false, // no-wait
			args,
		); err != nil {
			return fmt.Errorf("failed to declare queue %s: %w", q.name, err)
		}

		if err := ch.QueueBind(q.name, q.routingKey, ExchangeEvents, false, nil); err != nil {
			return fmt.Errorf("failed to bind queue %s to %s: %w", q.name, ExchangeEvents, err)
		}
	}

	c.logger.Info("rabbitmq topology declared successfully (exchanges, queues, and dlx)")
	return nil
}
