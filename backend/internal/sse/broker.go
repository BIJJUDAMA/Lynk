package sse

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"sync"
)

// Message represents a Server-Sent Event payload.
type Message struct {
	Event string
	Data  any
}

// BrokerOption configures a Broker instance.
type BrokerOption func(*Broker)

// WithPubSub configures an explicit PubSub backend.
func WithPubSub(ps PubSub) BrokerOption {
	return func(b *Broker) {
		b.pubsub = ps
	}
}

// WithRedisURL configures a Redis-backed PubSub broker.
func WithRedisURL(redisURL string) BrokerOption {
	return func(b *Broker) {
		b.pubsub = NewPubSubFromURL(redisURL, DefaultRedisSSEChannel)
	}
}

// Broker maintains per-user SSE subscriber channels and coordinates with a PubSub bus.
type Broker struct {
	nodeID      string
	mu          sync.RWMutex
	subscribers map[string][]chan Message
	pubsub      PubSub
	closeOnce   sync.Once
}

// generateNodeID creates an ephemeral random node ID for sender loop filtering.
func generateNodeID() string {
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// NewBroker creates a new SSE Broker. If no PubSub option is provided,
// it operates as an isolated in-memory broker.
func NewBroker(opts ...BrokerOption) *Broker {
	b := &Broker{
		nodeID:      generateNodeID(),
		subscribers: make(map[string][]chan Message),
	}

	for _, opt := range opts {
		if opt != nil {
			opt(b)
		}
	}

	if b.pubsub != nil {
		err := b.pubsub.Subscribe(context.Background(), b.handleRemoteEnvelope)
		if err != nil {
			slog.Warn("sse broker: failed to bind pubsub subscriber", "error", err)
		}
	}

	return b
}

// IsDistributed returns true if the broker is connected to a multi-node pubsub bus.
func (b *Broker) IsDistributed() bool {
	return b.pubsub != nil && b.pubsub.IsDistributed()
}

// Subscribe registers a new channel for the given userID and returns it.
// The caller is responsible for calling Unsubscribe when the connection closes.
func (b *Broker) Subscribe(userID string) chan Message {
	ch := make(chan Message, 8)
	b.mu.Lock()
	b.subscribers[userID] = append(b.subscribers[userID], ch)
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a specific channel for the given userID.
func (b *Broker) Unsubscribe(userID string, ch chan Message) {
	b.mu.Lock()
	defer b.mu.Unlock()

	chans := b.subscribers[userID]
	var updated []chan Message
	for _, c := range chans {
		if c != ch {
			updated = append(updated, c)
		}
	}
	if len(updated) == 0 {
		delete(b.subscribers, userID)
	} else {
		b.subscribers[userID] = updated
	}
}

// SendToUser broadcasts msg to all active subscribers for userID across the cluster.
func (b *Broker) SendToUser(userID, event string, data any) {
	if b.pubsub != nil {
		env, err := NewEnvelope(b.nodeID, userID, event, data, false)
		if err == nil {
			if err := b.pubsub.Publish(context.Background(), env); err == nil {
				return
			}
			slog.Warn("sse broker: pubsub publish failed, falling back to local delivery", "error", err)
		}
	}

	b.deliverLocalUser(userID, event, data)
}

// Broadcast sends a message to every connected subscriber across the cluster.
func (b *Broker) Broadcast(event string, data any) {
	if b.pubsub != nil {
		env, err := NewEnvelope(b.nodeID, "", event, data, true)
		if err == nil {
			if err := b.pubsub.Publish(context.Background(), env); err == nil {
				return
			}
			slog.Warn("sse broker: pubsub broadcast failed, falling back to local broadcast", "error", err)
		}
	}

	b.deliverLocalBroadcast(event, data)
}

// handleRemoteEnvelope is invoked when a message arrives from the PubSub bus.
func (b *Broker) handleRemoteEnvelope(env *Envelope) {
	if env == nil {
		return
	}

	var data any
	if len(env.Data) > 0 {
		_ = json.Unmarshal(env.Data, &data)
	}

	if env.IsBroadcast {
		b.deliverLocalBroadcast(env.Event, data)
		return
	}

	if env.TargetUserID != "" {
		b.deliverLocalUser(env.TargetUserID, env.Event, data)
	}
}

// deliverLocalUser fans out an event to all local subscribers for the target user.
func (b *Broker) deliverLocalUser(userID, event string, data any) {
	msg := Message{Event: event, Data: data}
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, ch := range b.subscribers[userID] {
		select {
		case ch <- msg:
		default:
		}
	}
}

// deliverLocalBroadcast fans out an event to all locally connected subscribers.
func (b *Broker) deliverLocalBroadcast(event string, data any) {
	msg := Message{Event: event, Data: data}
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, chans := range b.subscribers {
		for _, ch := range chans {
			select {
			case ch <- msg:
			default:
			}
		}
	}
}

// Close gracefully terminates any active pub/sub subscriptions.
func (b *Broker) Close() error {
	var err error
	b.closeOnce.Do(func() {
		if b.pubsub != nil {
			err = b.pubsub.Close()
		}
	})
	return err
}
