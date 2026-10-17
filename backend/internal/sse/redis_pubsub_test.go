package sse

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestEnvelope_Serialization(t *testing.T) {
	payload := map[string]string{"status": "active", "contract_id": "c-123"}
	env, err := NewEnvelope("node-1", "user-456", "contract_status_changed", payload, false)
	if err != nil {
		t.Fatalf("unexpected error creating envelope: %v", err)
	}

	if env.SenderNodeID != "node-1" {
		t.Errorf("expected SenderNodeID node-1, got %s", env.SenderNodeID)
	}
	if env.TargetUserID != "user-456" {
		t.Errorf("expected TargetUserID user-456, got %s", env.TargetUserID)
	}
	if env.Event != "contract_status_changed" {
		t.Errorf("expected Event contract_status_changed, got %s", env.Event)
	}
	if env.IsBroadcast {
		t.Errorf("expected IsBroadcast false, got true")
	}

	bytes, err := env.Marshal()
	if err != nil {
		t.Fatalf("unexpected error marshaling envelope: %v", err)
	}

	decoded, err := UnmarshalEnvelope(bytes)
	if err != nil {
		t.Fatalf("unexpected error unmarshaling envelope: %v", err)
	}

	if decoded.SenderNodeID != env.SenderNodeID {
		t.Errorf("expected sender %s, got %s", env.SenderNodeID, decoded.SenderNodeID)
	}
	if decoded.TargetUserID != env.TargetUserID {
		t.Errorf("expected target %s, got %s", env.TargetUserID, decoded.TargetUserID)
	}
	if decoded.Event != env.Event {
		t.Errorf("expected event %s, got %s", env.Event, decoded.Event)
	}

	var decodedPayload map[string]string
	if err := json.Unmarshal(decoded.Data, &decodedPayload); err != nil {
		t.Fatalf("unexpected error unmarshaling payload: %v", err)
	}
	if decodedPayload["contract_id"] != "c-123" {
		t.Errorf("expected contract_id c-123, got %s", decodedPayload["contract_id"])
	}
}

func TestEnvelope_BroadcastFlag(t *testing.T) {
	env, err := NewEnvelope("node-2", "", "system_announcement", "hello all", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !env.IsBroadcast {
		t.Errorf("expected IsBroadcast to be true")
	}
	if env.TargetUserID != "" {
		t.Errorf("expected empty TargetUserID for broadcast, got %s", env.TargetUserID)
	}
}

func TestInMemoryPubSub_PublishAndReceive(t *testing.T) {
	ps := NewInMemoryPubSub()
	defer ps.Close()

	if ps.IsDistributed() {
		t.Errorf("expected InMemoryPubSub IsDistributed to be false")
	}

	received := make(chan *Envelope, 1)
	err := ps.Subscribe(context.Background(), func(env *Envelope) {
		received <- env
	})
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	env, err := NewEnvelope("node-1", "user-1", "test_event", map[string]string{"msg": "hi"}, false)
	if err != nil {
		t.Fatalf("failed to create envelope: %v", err)
	}

	if err := ps.Publish(context.Background(), env); err != nil {
		t.Fatalf("failed to publish: %v", err)
	}

	select {
	case got := <-received:
		if got.Event != "test_event" || got.TargetUserID != "user-1" {
			t.Errorf("unexpected envelope received: %+v", got)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for in-memory pubsub message")
	}
}

func TestNewPubSubFromURL_FallbackOnEmptyOrInvalidURL(t *testing.T) {
	// Empty URL should fall back to in-memory
	ps1 := NewPubSubFromURL("", "test:channel")
	defer ps1.Close()
	if ps1.IsDistributed() {
		t.Errorf("expected empty URL to return in-memory pubsub")
	}

	// Invalid URL should fall back to in-memory
	ps2 := NewPubSubFromURL("invalid-redis-protocol://localhost:6379", "test:channel")
	defer ps2.Close()
	if ps2.IsDistributed() {
		t.Errorf("expected invalid URL to return in-memory pubsub")
	}

	// Unreachable Redis port should fall back to in-memory gracefully
	ps3 := NewPubSubFromURL("redis://127.0.0.1:54321", "test:channel")
	defer ps3.Close()
	if ps3.IsDistributed() {
		t.Errorf("expected unreachable Redis URL to return in-memory pubsub")
	}
}

