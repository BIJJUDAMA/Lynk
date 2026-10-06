package sse

import (
	"sync"
)

// Message represents a Server-Sent Event payload.
type Message struct {
	Event string
	Data  any
}

// Broker maintains per-user SSE subscriber channels.
type Broker struct {
	mu          sync.RWMutex
	subscribers map[string][]chan Message
}

// NewBroker creates a new SSE Broker.
func NewBroker() *Broker {
	return &Broker{
		subscribers: make(map[string][]chan Message),
	}
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

// SendToUser broadcasts msg to all active subscribers for userID.
// Delivers are non-blocking; slow subscribers are skipped silently.
func (b *Broker) SendToUser(userID, event string, data any) {
	msg := Message{Event: event, Data: data}
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, ch := range b.subscribers[userID] {
		select {
		case ch <- msg:
		default:
			// subscriber is slow; skip to avoid blocking the broker
		}
	}
}

// Broadcast sends a message to every connected subscriber.
func (b *Broker) Broadcast(event string, data any) {
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
