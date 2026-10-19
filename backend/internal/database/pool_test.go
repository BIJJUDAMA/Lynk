package database

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRouter_NilReplicaFallback(t *testing.T) {
	primary := &pgxpool.Pool{}

	router := NewRouter(primary, nil)
	if router == nil {
		t.Fatal("expected non-nil router instance")
	}

	if got := router.Primary(); got != primary {
		t.Errorf("Primary() = %p; want %p", got, primary)
	}

	if got := router.Replica(); got != primary {
		t.Errorf("Replica() = %p; want fallback to primary %p", got, primary)
	}
}

func TestRouter_DualPoolSegregation(t *testing.T) {
	primary := &pgxpool.Pool{}
	replica := &pgxpool.Pool{}

	if primary == replica {
		t.Fatal("expected primary and replica to be distinct pointer instances")
	}

	router := NewRouter(primary, replica)
	if router == nil {
		t.Fatal("expected non-nil router instance")
	}

	if got := router.Primary(); got != primary {
		t.Errorf("Primary() = %p; want primary %p", got, primary)
	}

	if got := router.Replica(); got != replica {
		t.Errorf("Replica() = %p; want replica %p", got, replica)
	}

	if router.Primary() == router.Replica() {
		t.Error("expected Primary() and Replica() to return distinct pool pointers")
	}
}

func TestRouter_UninitializedNilSafety(t *testing.T) {
	router := NewRouter(nil, nil)
	if router == nil {
		t.Fatal("expected non-nil router instance from NewRouter(nil, nil)")
	}

	if got := router.Primary(); got != nil {
		t.Errorf("Primary() = %p; want nil", got)
	}

	if got := router.Replica(); got != nil {
		t.Errorf("Replica() = %p; want nil", got)
	}
}

func TestRouter_NilReceiverSafety(t *testing.T) {
	var r *poolRouter

	if got := r.Primary(); got != nil {
		t.Errorf("nil receiver Primary() = %p; want nil", got)
	}

	if got := r.Replica(); got != nil {
		t.Errorf("nil receiver Replica() = %p; want nil", got)
	}
}
