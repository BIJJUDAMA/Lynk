// Package queue publisher with publisher confirms.
// 100% pure 7-bit ASCII
package queue

import (
	"context"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher defines message publishing with guaranteed delivery confirmation.
type Publisher interface {
	PublishPersistent(ctx context.Context, exchange, routingKey string, body []byte, headers map[string]interface{}) error
}

// ConfirmedPublisher publishes messages to RabbitMQ with mandatory publisher confirms.
type ConfirmedPublisher struct {
	client *Client
}

// NewConfirmedPublisher creates a publisher instance backed by the RabbitMQ client.
func NewConfirmedPublisher(client *Client) *ConfirmedPublisher {
	return &ConfirmedPublisher{client: client}
}

// PublishPersistent publishes a persistent message onto an exchange and blocks until the broker confirms.
func (p *ConfirmedPublisher) PublishPersistent(ctx context.Context, exchange, routingKey string, body []byte, headers map[string]interface{}) error {
	ch, err := p.client.Channel()
	if err != nil {
		return fmt.Errorf("publish failed to acquire channel: %w", err)
	}
	defer p.client.ReleaseChannel(ch)

	// Enable publisher confirms on this channel
	if err := ch.Confirm(false); err != nil {
		return fmt.Errorf("failed to put channel into confirm mode: %w", err)
	}

	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))

	var amqpTable amqp.Table
	if headers != nil {
		amqpTable = make(amqp.Table, len(headers))
		for k, v := range headers {
			amqpTable[k] = v
		}
	}

	msg := amqp.Publishing{
		DeliveryMode: amqp.Persistent, // Guaranteed disk persistence flag
		ContentType:  "application/json",
		Timestamp:    time.Now(),
		Body:         body,
		Headers:      amqpTable,
	}

	if err := ch.PublishWithContext(
		ctx,
		exchange,
		routingKey,
		false, // mandatory
		false, // immediate
		msg,
	); err != nil {
		return fmt.Errorf("publish error: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case confirmation, ok := <-confirms:
		if !ok {
			return ErrPublishTimeout
		}
		if !confirmation.Ack {
			return ErrPublishFailed
		}
		return nil
	}
}
