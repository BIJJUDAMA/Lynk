package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Envelope encapsulates an SSE event message transmitted across nodes via Pub/Sub.
type Envelope struct {
	SenderNodeID string          `json:"sender_node_id"`
	TargetUserID string          `json:"target_user_id,omitempty"`
	IsBroadcast  bool            `json:"is_broadcast"`
	Event        string          `json:"event"`
	Data         json.RawMessage `json:"data"`
}

// NewEnvelope constructs a new Envelope, serializing data to json.RawMessage.
func NewEnvelope(senderNodeID, targetUserID, event string, data any, isBroadcast bool) (*Envelope, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sse data payload: %w", err)
	}
	return &Envelope{
		SenderNodeID: senderNodeID,
		TargetUserID: targetUserID,
		IsBroadcast:  isBroadcast,
		Event:        event,
		Data:         raw,
	}, nil
}

// Marshal encodes the Envelope as JSON bytes.
func (e *Envelope) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// UnmarshalEnvelope decodes raw JSON bytes into an Envelope.
func UnmarshalEnvelope(data []byte) (*Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("failed to decode sse envelope: %w", err)
	}
	return &env, nil
}

// PubSub defines an abstraction for distributing SSE envelopes across backend nodes.
type PubSub interface {
	Publish(ctx context.Context, env *Envelope) error
	Subscribe(ctx context.Context, handler func(env *Envelope)) error
	Close() error
	IsDistributed() bool
}

// InMemoryPubSub implements PubSub within a single process without external network dependencies.
type InMemoryPubSub struct {
	mu       sync.RWMutex
	handlers []func(env *Envelope)
	closed   bool
}

// NewInMemoryPubSub creates a local in-memory PubSub instance.
func NewInMemoryPubSub() *InMemoryPubSub {
	return &InMemoryPubSub{
		handlers: make([]func(env *Envelope), 0),
	}
}

// Publish delivers the envelope synchronously to all registered in-memory handlers.
func (m *InMemoryPubSub) Publish(ctx context.Context, env *Envelope) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return fmt.Errorf("in-memory pubsub is closed")
	}

	for _, handler := range m.handlers {
		if handler != nil {
			handler(env)
		}
	}
	return nil
}

// Subscribe registers a message callback on the in-memory pubsub bus.
func (m *InMemoryPubSub) Subscribe(ctx context.Context, handler func(env *Envelope)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return fmt.Errorf("in-memory pubsub is closed")
	}

	m.handlers = append(m.handlers, handler)
	return nil
}

// Close marks the in-memory pubsub as terminated.
func (m *InMemoryPubSub) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	m.handlers = nil
	return nil
}

// IsDistributed returns false for InMemoryPubSub.
func (m *InMemoryPubSub) IsDistributed() bool {
	return false
}

const DefaultRedisSSEChannel = "lynk:sse:events"

// RedisPubSub implements PubSub using a Redis server connection.
type RedisPubSub struct {
	client    *redis.Client
	channel   string
	pubsub    *redis.PubSub
	closeOnce sync.Once
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewRedisPubSub constructs a RedisPubSub adapter.
func NewRedisPubSub(client *redis.Client, channel string) *RedisPubSub {
	if strings.TrimSpace(channel) == "" {
		channel = DefaultRedisSSEChannel
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &RedisPubSub{
		client:  client,
		channel: channel,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Publish serializes the Envelope to JSON and sends it to the Redis channel.
func (r *RedisPubSub) Publish(ctx context.Context, env *Envelope) error {
	if r.client == nil {
		return fmt.Errorf("redis client is not initialized")
	}
	bytes, err := env.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal envelope for redis: %w", err)
	}
	return r.client.Publish(ctx, r.channel, bytes).Err()
}

// Subscribe begins listening on the Redis channel and invokes handler on each received message.
func (r *RedisPubSub) Subscribe(ctx context.Context, handler func(env *Envelope)) error {
	if r.client == nil {
		return fmt.Errorf("redis client is not initialized")
	}

	r.pubsub = r.client.Subscribe(ctx, r.channel)
	// Verify subscription handshake
	if _, err := r.pubsub.Receive(ctx); err != nil {
		_ = r.pubsub.Close()
		return fmt.Errorf("failed to establish redis subscription: %w", err)
	}

	go func() {
		ch := r.pubsub.Channel()
		for {
			select {
			case <-r.ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				env, err := UnmarshalEnvelope([]byte(msg.Payload))
				if err != nil {
					slog.Warn("redis pubsub: dropped malformed envelope", "error", err)
					continue
				}
				if handler != nil {
					handler(env)
				}
			}
		}
	}()

	return nil
}

// Close gracefully terminates the Redis PubSub subscriber and closes the client.
func (r *RedisPubSub) Close() error {
	var closeErr error
	r.closeOnce.Do(func() {
		r.cancel()
		if r.pubsub != nil {
			if err := r.pubsub.Close(); err != nil {
				closeErr = err
			}
		}
		if r.client != nil {
			if err := r.client.Close(); err != nil && closeErr == nil {
				closeErr = err
			}
		}
	})
	return closeErr
}

// IsDistributed returns true for RedisPubSub.
func (r *RedisPubSub) IsDistributed() bool {
	return true
}

// NewPubSubFromURL attempts to connect to Redis at redisURL. If redisURL is empty,
// invalid, or unreachable, it logs a warning and returns an InMemoryPubSub fallback.
func NewPubSubFromURL(redisURL, channel string) PubSub {
	trimmed := strings.TrimSpace(redisURL)
	if trimmed == "" {
		return NewInMemoryPubSub()
	}

	opt, err := redis.ParseURL(trimmed)
	if err != nil {
		slog.Warn("invalid redis url for sse pubsub, falling back to in-memory", "error", err)
		return NewInMemoryPubSub()
	}

	opt.MaxRetries = 1
	opt.DialTimeout = 200 * time.Millisecond
	opt.ReadTimeout = 200 * time.Millisecond
	opt.WriteTimeout = 200 * time.Millisecond

	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		slog.Warn("cannot ping redis for sse pubsub, falling back to in-memory", "error", err)
		_ = client.Close()
		return NewInMemoryPubSub()
	}

	slog.Info("connected to redis sse pubsub successfully", "channel", channel)
	return NewRedisPubSub(client, channel)
}

