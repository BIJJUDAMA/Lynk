package recruitment

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestCapacityLeaser_MockAcquire(t *testing.T) {
	// Standalone in-memory fallback leaser test
	leaser := NewInMemoryCapacityLeaser()
	ctx := context.Background()
	projID := uuid.New()
	maxMembers := 3

	// Acquire up to capacity
	for i := 0; i < maxMembers; i++ {
		ok, err := leaser.AcquireLease(ctx, projID, maxMembers)
		if err != nil || !ok {
			t.Fatalf("expected lease %d to succeed: ok=%v, err=%v", i+1, ok, err)
		}
	}

	// Over-capacity attempt must fail immediately
	ok, err := leaser.AcquireLease(ctx, projID, maxMembers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected lease to be rejected when capacity reached")
	}

	// Release 1 and acquire again
	err = leaser.ReleaseLease(ctx, projID)
	if err != nil {
		t.Fatalf("ReleaseLease failed: %v", err)
	}

	ok, err = leaser.AcquireLease(ctx, projID, maxMembers)
	if err != nil || !ok {
		t.Fatalf("expected re-acquire to succeed after release: ok=%v, err=%v", ok, err)
	}
}
