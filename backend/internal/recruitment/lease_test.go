package recruitment

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
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

func newTestRedisCapacityLeaser(t *testing.T) (*RedisCapacityLeaser, redis.UniversalClient) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisCapacityLeaser(client), client
}

func redisCapacityKey(projectID uuid.UUID) string {
	return fmt.Sprintf("recruitment:capacity:%s", projectID.String())
}

func redisCapacityCounter(t *testing.T, client redis.UniversalClient, key string) int {
	t.Helper()
	value, err := client.Get(context.Background(), key).Int()
	if err != nil {
		t.Fatalf("read counter %q: %v", key, err)
	}
	return value
}

func TestRedisCapacityLeaser_AcquireRelease(t *testing.T) {
	leaser, client := newTestRedisCapacityLeaser(t)
	ctx := context.Background()
	projID := uuid.New()
	key := redisCapacityKey(projID)
	maxMembers := 3

	for i := 0; i < maxMembers; i++ {
		ok, err := leaser.AcquireLease(ctx, projID, maxMembers)
		if err != nil || !ok {
			t.Fatalf("expected lease %d to succeed: ok=%v, err=%v", i+1, ok, err)
		}
	}
	if got := redisCapacityCounter(t, client, key); got != maxMembers {
		t.Fatalf("expected counter %d after acquires, got %d", maxMembers, got)
	}

	ok, err := leaser.AcquireLease(ctx, projID, maxMembers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected lease to be rejected when capacity reached")
	}
	if got := redisCapacityCounter(t, client, key); got != maxMembers {
		t.Fatalf("expected counter to stay at %d after rejection, got %d", maxMembers, got)
	}

	if err := leaser.ReleaseLease(ctx, projID); err != nil {
		t.Fatalf("ReleaseLease failed: %v", err)
	}
	if got := redisCapacityCounter(t, client, key); got != maxMembers-1 {
		t.Fatalf("expected counter %d after release, got %d", maxMembers-1, got)
	}

	ok, err = leaser.AcquireLease(ctx, projID, maxMembers)
	if err != nil || !ok {
		t.Fatalf("expected re-acquire to succeed after release: ok=%v, err=%v", ok, err)
	}
	if got := redisCapacityCounter(t, client, key); got != maxMembers {
		t.Fatalf("expected counter %d after re-acquire, got %d", maxMembers, got)
	}

	ok, err = leaser.AcquireLease(ctx, projID, maxMembers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected lease to be rejected after re-acquiring to capacity")
	}
}

func TestRedisCapacityLeaser_NonPositiveMaxIsUnlimited(t *testing.T) {
	leaser, client := newTestRedisCapacityLeaser(t)
	ctx := context.Background()

	for _, maxMembers := range []int{0, -1} {
		t.Run(fmt.Sprintf("max=%d", maxMembers), func(t *testing.T) {
			projID := uuid.New()
			key := redisCapacityKey(projID)

			const attempts = 8
			var wg sync.WaitGroup
			results := make([]bool, attempts)
			errs := make([]error, attempts)
			for i := 0; i < attempts; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					ok, err := leaser.AcquireLease(ctx, projID, maxMembers)
					results[idx] = ok
					errs[idx] = err
				}(i)
			}
			wg.Wait()

			for i := 0; i < attempts; i++ {
				if errs[i] != nil {
					t.Fatalf("acquire %d failed: %v", i, errs[i])
				}
				if !results[i] {
					t.Fatalf("acquire %d rejected despite unlimited capacity", i)
				}
			}
			if got := redisCapacityCounter(t, client, key); got != attempts {
				t.Fatalf("expected counter %d after unlimited acquires, got %d", attempts, got)
			}
		})
	}
}

func TestRedisCapacityLeaser_ReleaseFloorsAtZero(t *testing.T) {
	leaser, client := newTestRedisCapacityLeaser(t)
	ctx := context.Background()
	projID := uuid.New()
	key := redisCapacityKey(projID)
	maxMembers := 2

	if err := leaser.ReleaseLease(ctx, projID); err != nil {
		t.Fatalf("ReleaseLease on missing key failed: %v", err)
	}
	exists, err := client.Exists(ctx, key).Result()
	if err != nil {
		t.Fatalf("check key existence: %v", err)
	}
	if exists != 0 {
		t.Fatalf("expected release on missing key to leave no key, got %d keys", exists)
	}

	for i := 0; i < maxMembers; i++ {
		ok, err := leaser.AcquireLease(ctx, projID, maxMembers)
		if err != nil || !ok {
			t.Fatalf("expected lease %d to succeed: ok=%v, err=%v", i+1, ok, err)
		}
	}

	for i := 0; i < maxMembers+3; i++ {
		if err := leaser.ReleaseLease(ctx, projID); err != nil {
			t.Fatalf("ReleaseLease %d failed: %v", i+1, err)
		}
	}
	if got := redisCapacityCounter(t, client, key); got != 0 {
		t.Fatalf("expected counter to floor at 0 after spurious releases, got %d", got)
	}

	for i := 0; i < maxMembers; i++ {
		ok, err := leaser.AcquireLease(ctx, projID, maxMembers)
		if err != nil || !ok {
			t.Fatalf("expected lease %d after spurious releases: ok=%v, err=%v", i+1, ok, err)
		}
	}
	ok, err := leaser.AcquireLease(ctx, projID, maxMembers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected capacity to still be enforced after spurious releases")
	}

	if err := client.Del(ctx, key).Err(); err != nil {
		t.Fatalf("delete key: %v", err)
	}
	if err := leaser.ReleaseLease(ctx, projID); err != nil {
		t.Fatalf("ReleaseLease after key deletion failed: %v", err)
	}
	exists, err = client.Exists(ctx, key).Result()
	if err != nil {
		t.Fatalf("check key existence after deletion: %v", err)
	}
	if exists != 0 {
		t.Fatalf("expected release after key deletion to leave no key, got %d keys", exists)
	}

	for i := 0; i < maxMembers; i++ {
		ok, err := leaser.AcquireLease(ctx, projID, maxMembers)
		if err != nil || !ok {
			t.Fatalf("expected lease %d after key deletion: ok=%v, err=%v", i+1, ok, err)
		}
	}
	ok, err = leaser.AcquireLease(ctx, projID, maxMembers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected capacity to still be enforced after key deletion and release")
	}
}
