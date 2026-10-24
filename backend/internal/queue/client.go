// Package queue provides resilient AMQP 0-9-1 messaging infrastructure for Lynk.
// 100% pure 7-bit ASCII
package queue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	ErrNotConnected         = errors.New("queue: client not connected to rabbitmq broker")
	ErrPublishFailed        = errors.New("queue: message publication rejected by broker nack")
	ErrPublishTimeout       = errors.New("queue: publisher confirmation timed out")
	ErrChannelPoolExhausted = errors.New("queue: channel pool exhausted")
)

// Config holds broker connection parameters.
type Config struct {
	URL            string
	ReconnectDelay time.Duration
	MaxChannels    int
}

// Client manages resilient AMQP connections and channel lifecycle.
type Client struct {
	cfg        Config
	logger     *slog.Logger
	mu         sync.RWMutex
	conn       *amqp.Connection
	closed     bool
	closeChan  chan struct{}
	notifyConn chan *amqp.Error

	chPoolMu sync.Mutex
	chPool   []*amqp.Channel
}

// NewClient initializes a new RabbitMQ client with background reconnection supervision.
func NewClient(cfg Config, logger *slog.Logger) *Client {
	if cfg.ReconnectDelay <= 0 {
		cfg.ReconnectDelay = 2 * time.Second
	}
	if cfg.MaxChannels <= 0 {
		cfg.MaxChannels = 10
	}
	if logger == nil {
		logger = slog.Default()
	}

	c := &Client{
		cfg:       cfg,
		logger:    logger.With("subsystem", "queue.client"),
		closeChan: make(chan struct{}),
		chPool:    make([]*amqp.Channel, 0, cfg.MaxChannels),
	}

	return c
}

// Connect establishes the AMQP connection and starts the reconnection watcher.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return errors.New("queue: client is closed")
	}

	conn, err := amqp.Dial(c.cfg.URL)
	if err != nil {
		return fmt.Errorf("queue dial failed: %w", err)
	}

	c.conn = conn
	c.notifyConn = conn.NotifyClose(make(chan *amqp.Error, 1))

	go c.reconnectWatcher()

	c.logger.Info("connected to rabbitmq broker", "url", redactURL(c.cfg.URL))
	return nil
}

// IsConnected returns whether the AMQP connection is healthy.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn != nil && !c.conn.IsClosed()
}

// Channel acquires a channel from the pool or opens a new one.
func (c *Client) Channel() (*amqp.Channel, error) {
	c.mu.RLock()
	conn := c.conn
	closed := c.closed
	c.mu.RUnlock()

	if closed || conn == nil || conn.IsClosed() {
		return nil, ErrNotConnected
	}

	c.chPoolMu.Lock()
	if len(c.chPool) > 0 {
		ch := c.chPool[len(c.chPool)-1]
		c.chPool = c.chPool[:len(c.chPool)-1]
		c.chPoolMu.Unlock()
		if !ch.IsClosed() {
			return ch, nil
		}
	} else {
		c.chPoolMu.Unlock()
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}
	return ch, nil
}

// ReleaseChannel returns an open channel to the pool.
func (c *Client) ReleaseChannel(ch *amqp.Channel) {
	if ch == nil || ch.IsClosed() {
		return
	}

	c.chPoolMu.Lock()
	defer c.chPoolMu.Unlock()

	if len(c.chPool) < c.cfg.MaxChannels {
		c.chPool = append(c.chPool, ch)
	} else {
		_ = ch.Close()
	}
}

// Close shuts down channels, connections, and background supervisors.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	close(c.closeChan)

	c.chPoolMu.Lock()
	for _, ch := range c.chPool {
		if !ch.IsClosed() {
			_ = ch.Close()
		}
	}
	c.chPool = nil
	c.chPoolMu.Unlock()

	var err error
	if c.conn != nil && !c.conn.IsClosed() {
		err = c.conn.Close()
	}
	c.mu.Unlock()

	c.logger.Info("rabbitmq client closed")
	return err
}

func (c *Client) reconnectWatcher() {
	for {
		select {
		case <-c.closeChan:
			return
		case err, ok := <-c.notifyConn:
			if !ok {
				return
			}
			c.logger.Warn("rabbitmq connection lost, attempting reconnect", "error", err)

			c.chPoolMu.Lock()
			c.chPool = nil
			c.chPoolMu.Unlock()

			c.mu.Lock()
			c.conn = nil
			c.mu.Unlock()

			backoff := c.cfg.ReconnectDelay
			for {
				select {
				case <-c.closeChan:
					return
				case <-time.After(backoff):
					conn, dialErr := amqp.Dial(c.cfg.URL)
					if dialErr != nil {
						c.logger.Error("rabbitmq reconnect failed", "error", dialErr, "next_retry", backoff)
						backoff = min(backoff*2, 30*time.Second)
						continue
					}

					c.mu.Lock()
					c.conn = conn
					c.notifyConn = conn.NotifyClose(make(chan *amqp.Error, 1))
					c.mu.Unlock()

					c.logger.Info("reconnected to rabbitmq broker successfully")
					break
				}
				break
			}
		}
	}
}

func redactURL(raw string) string {
	if len(raw) > 15 {
		return raw[:10] + "...redacted"
	}
	return "amqp://...redacted"
}
