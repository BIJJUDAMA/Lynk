// Package queue consumer with DLQ diversion and jittered retry.
// 100% pure 7-bit ASCII
package queue

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const HeaderRetryCount = "x-retry-count"

// HandlerFunc processes a single consumed delivery.
type HandlerFunc func(ctx context.Context, d amqp.Delivery) error

// Consumer wraps an AMQP queue consumer with prefetch and backoff policies.
type Consumer struct {
	client      *Client
	queueName   string
	consumerTag string
	prefetch    int
	policy      RetryPolicy
	logger      *slog.Logger
	stopChan    chan struct{}
	wg          sync.WaitGroup
}

// NewConsumer creates a consumer for a specific queue.
func NewConsumer(client *Client, queueName, consumerTag string, prefetch int, policy RetryPolicy, logger *slog.Logger) *Consumer {
	if prefetch <= 0 {
		prefetch = 20
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &Consumer{
		client:      client,
		queueName:   queueName,
		consumerTag: consumerTag,
		prefetch:    prefetch,
		policy:      policy,
		logger:      logger.With("subsystem", "queue.consumer", "queue", queueName),
		stopChan:    make(chan struct{}),
	}
}

// Start spawns worker goroutines to process incoming deliveries.
func (c *Consumer) Start(ctx context.Context, workers int, handler HandlerFunc) error {
	ch, err := c.client.Channel()
	if err != nil {
		return fmt.Errorf("consumer failed to get channel: %w", err)
	}

	if err := ch.Qos(c.prefetch, 0, false); err != nil {
		c.client.ReleaseChannel(ch)
		return fmt.Errorf("failed to set consumer QoS: %w", err)
	}

	deliveries, err := ch.ConsumeWithContext(
		ctx,
		c.queueName,
		c.consumerTag,
		false, // auto-ack (false for manual ack safety)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		c.client.ReleaseChannel(ch)
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	if workers <= 0 {
		workers = 4
	}

	for i := 0; i < workers; i++ {
		c.wg.Add(1)
		go c.workerLoop(ctx, deliveries, handler, i)
	}

	c.logger.Info("queue consumer started", "workers", workers, "prefetch", c.prefetch)
	return nil
}

// Stop shuts down the consumer and awaits in-flight message completion.
func (c *Consumer) Stop() {
	close(c.stopChan)
	c.wg.Wait()
	c.logger.Info("queue consumer stopped gracefully")
}

func (c *Consumer) workerLoop(ctx context.Context, deliveries <-chan amqp.Delivery, handler HandlerFunc, workerID int) {
	defer c.wg.Done()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ctx.Done():
			return
		case d, ok := <-deliveries:
			if !ok {
				return
			}

			if err := handler(ctx, d); err != nil {
				c.handleFailure(ctx, d, err)
			} else {
				_ = d.Ack(false)
			}
		}
	}
}

func (c *Consumer) handleFailure(ctx context.Context, d amqp.Delivery, err error) {
	attempt := getRetryCount(d.Headers)

	if attempt >= c.policy.MaxRetries {
		c.logger.Error("message exceeded max retries, rejecting to dead-letter exchange",
			"error", err,
			"attempt", attempt,
			"message_id", d.MessageId,
			"routing_key", d.RoutingKey,
		)
		// Nack without requeue triggers RabbitMQ's x-dead-letter-exchange routing
		_ = d.Nack(false, false)
		return
	}

	delay := c.policy.ComputeDelay(attempt + 1)
	c.logger.Warn("message handler failed, delaying before retry",
		"error", err,
		"attempt", attempt+1,
		"delay_ms", delay.Milliseconds(),
	)

	time.Sleep(delay)
	// Requeue for next attempt
	_ = d.Nack(false, true)
}

func getRetryCount(headers amqp.Table) int {
	if headers == nil {
		return 0
	}
	if val, ok := headers[HeaderRetryCount]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int32:
			return int(v)
		case int64:
			return int(v)
		}
	}
	return 0
}
