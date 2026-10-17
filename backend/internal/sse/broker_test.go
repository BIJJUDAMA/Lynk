package sse

import (
	"sync"
	"testing"
	"time"
)

func TestBroker_SubscribeAndBroadcast(t *testing.T) {
	broker := NewBroker()
	ch := broker.Subscribe("user-123")
	defer broker.Unsubscribe("user-123", ch)

	broker.SendToUser("user-123", "proposal_accepted", map[string]string{"contract_id": "c-1"})

	select {
	case msg := <-ch:
		if msg.Event != "proposal_accepted" {
			t.Fatalf("expected event proposal_accepted, got %s", msg.Event)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timed out waiting for SSE message")
	}
}

func TestBroker_UnsubscribeRemovesChannel(t *testing.T) {
	broker := NewBroker()
	ch := broker.Subscribe("user-456")
	broker.Unsubscribe("user-456", ch)

	// After unsubscribe, the user entry should be gone from the map
	broker.mu.RLock()
	_, exists := broker.subscribers["user-456"]
	broker.mu.RUnlock()

	if exists {
		t.Fatalf("expected subscriber to be removed after Unsubscribe")
	}
}

func TestBroker_Broadcast(t *testing.T) {
	broker := NewBroker()
	ch1 := broker.Subscribe("user-a")
	ch2 := broker.Subscribe("user-b")
	defer broker.Unsubscribe("user-a", ch1)
	defer broker.Unsubscribe("user-b", ch2)

	broker.Broadcast("system_notice", "maintenance")

	for _, ch := range []chan Message{ch1, ch2} {
		select {
		case msg := <-ch:
			if msg.Event != "system_notice" {
				t.Fatalf("expected system_notice, got %s", msg.Event)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("timed out waiting for broadcast message")
		}
	}
}

func TestBroker_MultipleSubscribersPerUser(t *testing.T) {
	broker := NewBroker()
	ch1 := broker.Subscribe("user-x")
	ch2 := broker.Subscribe("user-x")
	defer broker.Unsubscribe("user-x", ch1)
	defer broker.Unsubscribe("user-x", ch2)

	broker.SendToUser("user-x", "ping", nil)
	broker.SendToUser("user-x", "ping", nil)

	// Both channels should receive the message
	for _, ch := range []chan Message{ch1, ch2} {
		select {
		case msg := <-ch:
			if msg.Event != "ping" {
				t.Fatalf("expected ping, got %s", msg.Event)
			}
		case <-time.After(300 * time.Millisecond):
			t.Fatalf("timed out waiting for message on one of the channels")
		}
	}
}

func TestBroker_ConcurrentSendAndUnsubscribe(t *testing.T) {
	broker := NewBroker()
	userID := "user-concurrent"

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Senders sending events to user
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					broker.SendToUser(userID, "test_event", map[string]int{"id": workerID})
					time.Sleep(100 * time.Microsecond)
				}
			}
		}(i)
	}

	// Concurrent subscribers and unsubscribers
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					ch := broker.Subscribe(userID)
					select {
					case <-ch:
					case <-time.After(500 * time.Microsecond):
					}
					broker.Unsubscribe(userID, ch)
				}
			}
		}()
	}

	time.Sleep(200 * time.Millisecond)
	close(stop)
	wg.Wait()
}

func TestBroker_MultiNodeSimulationWithPubSub(t *testing.T) {
	sharedBus := NewInMemoryPubSub()
	defer func() { _ = sharedBus.Close() }()

	// Node 1 (has subscriber for user-A)
	node1 := NewBroker(WithPubSub(sharedBus))
	defer func() { _ = node1.Close() }()

	// Node 2 (has subscriber for user-B)
	node2 := NewBroker(WithPubSub(sharedBus))
	defer func() { _ = node2.Close() }()

	chUserA := node1.Subscribe("user-A")
	defer node1.Unsubscribe("user-A", chUserA)

	chUserB := node2.Subscribe("user-B")
	defer node2.Unsubscribe("user-B", chUserB)

	// Node 2 sends targeted message to user-A
	node2.SendToUser("user-A", "remote_update", map[string]string{"foo": "bar"})

	select {
	case msg := <-chUserA:
		if msg.Event != "remote_update" {
			t.Errorf("expected event remote_update, got %s", msg.Event)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for cross-node event on node 1")
	}

	// Verify User-B on Node 2 did not receive the user-A targeted message
	select {
	case msg := <-chUserB:
		t.Fatalf("unexpected message received by user-B: %+v", msg)
	default:
		// Expected: no cross-user leakage
	}

	// Node 1 broadcasts announcement -> both User-A and User-B should receive it
	node1.Broadcast("system_alert", "all hands")

	select {
	case msg := <-chUserA:
		if msg.Event != "system_alert" {
			t.Errorf("expected event system_alert on user-A, got %s", msg.Event)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for broadcast on user-A")
	}

	select {
	case msg := <-chUserB:
		if msg.Event != "system_alert" {
			t.Errorf("expected event system_alert on user-B, got %s", msg.Event)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for broadcast on user-B")
	}
}

func TestBroker_FallbackWithoutPubSub(t *testing.T) {
	b := NewBroker()
	defer func() { _ = b.Close() }()

	if b.IsDistributed() {
		t.Errorf("expected default broker to not be distributed")
	}

	ch := b.Subscribe("local-user")
	defer b.Unsubscribe("local-user", ch)

	b.SendToUser("local-user", "ping", "pong")

	select {
	case msg := <-ch:
		if msg.Event != "ping" {
			t.Errorf("expected ping, got %s", msg.Event)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for local delivery")
	}
}
