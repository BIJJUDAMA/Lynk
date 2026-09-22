package event

import (
	"context"
	"fmt"
	"sync"
)

// Handler handles an emitted domain event.
type Handler func(ctx context.Context, e Event) error

// EventBus defines the interface for publishing and subscribing to domain events.
type EventBus interface {
	Subscribe(topic Topic, h Handler)
	Publish(ctx context.Context, e Event) error
}

// Bus provides in-process publish/subscribe messaging for domain events.
type Bus struct {
	mu       sync.RWMutex
	handlers map[Topic][]Handler
}

// Ensure Bus implements EventBus interface.
var _ EventBus = (*Bus)(nil)

// NewBus creates a new in-memory synchronous event bus.
func NewBus() *Bus {
	return &Bus{
		handlers: make(map[Topic][]Handler),
	}
}

// Subscribe registers a handler for a given topic.
func (b *Bus) Subscribe(topic Topic, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[topic] = append(b.handlers[topic], h)
}

// Publish executes all subscribed handlers for the event synchronously.
func (b *Bus) Publish(ctx context.Context, e Event) error {
	b.mu.RLock()
	handlers := append([]Handler(nil), b.handlers[e.Type]...)
	b.mu.RUnlock()

	for _, h := range handlers {
		if err := h(ctx, e); err != nil {
			return fmt.Errorf("event handler for topic %q failed: %w", e.Type, err)
		}
	}
	return nil
}
