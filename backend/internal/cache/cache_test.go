package cache

import (
	"context"
	"testing"
	"time"
)

type sampleData struct {
	Title string `json:"title"`
}

func TestMemoryCache_GetSetDelete(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()

	// 1. Get absent key
	var out sampleData
	found, err := c.Get(ctx, "job:123", &out)
	if err != nil {
		t.Fatalf("unexpected error on get: %v", err)
	}
	if found {
		t.Fatalf("expected key to be absent, got found=true")
	}

	// 2. Set key with TTL
	err = c.Set(ctx, "job:123", sampleData{Title: "Campus Gig"}, time.Minute)
	if err != nil {
		t.Fatalf("set failed: %v", err)
	}

	// 3. Get present key
	found, err = c.Get(ctx, "job:123", &out)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if !found {
		t.Fatalf("expected key to be found")
	}
	if out.Title != "Campus Gig" {
		t.Fatalf("expected title 'Campus Gig', got '%s'", out.Title)
	}

	// 4. Delete key
	err = c.Delete(ctx, "job:123")
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	found, err = c.Get(ctx, "job:123", &out)
	if err != nil {
		t.Fatalf("unexpected error after delete: %v", err)
	}
	if found {
		t.Fatalf("expected key to be deleted, got found=true")
	}
}

func TestMemoryCache_Expiration(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()

	err := c.Set(ctx, "expiring_key", sampleData{Title: "Expiring"}, 30*time.Millisecond)
	if err != nil {
		t.Fatalf("set failed: %v", err)
	}

	var out sampleData
	found, err := c.Get(ctx, "expiring_key", &out)
	if err != nil || !found {
		t.Fatalf("expected key to be found immediately, found=%v err=%v", found, err)
	}

	time.Sleep(50 * time.Millisecond)

	found, err = c.Get(ctx, "expiring_key", &out)
	if err != nil {
		t.Fatalf("unexpected error on expired key: %v", err)
	}
	if found {
		t.Fatalf("expected key to be expired, but found=true")
	}
}

func TestMemoryCache_DeletePattern(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()

	_ = c.Set(ctx, "jobs:search:1", sampleData{Title: "Job 1"}, time.Minute)
	_ = c.Set(ctx, "jobs:search:2", sampleData{Title: "Job 2"}, time.Minute)
	_ = c.Set(ctx, "profile:1", sampleData{Title: "Profile 1"}, time.Minute)

	err := c.DeletePattern(ctx, "jobs:search:*")
	if err != nil {
		t.Fatalf("DeletePattern failed: %v", err)
	}

	var out sampleData
	found1, _ := c.Get(ctx, "jobs:search:1", &out)
	if found1 {
		t.Errorf("expected jobs:search:1 to be deleted")
	}

	found2, _ := c.Get(ctx, "jobs:search:2", &out)
	if found2 {
		t.Errorf("expected jobs:search:2 to be deleted")
	}

	found3, _ := c.Get(ctx, "profile:1", &out)
	if !found3 {
		t.Errorf("expected profile:1 to still exist")
	}
}

func TestNew_Fallback(t *testing.T) {
	// Empty redis URL falls back to memory cache
	c1 := New("")
	if c1 == nil {
		t.Fatalf("expected non-nil cache")
	}

	// Invalid or unreachable redis URL falls back to memory cache
	c2 := New("redis://127.0.0.1:65534")
	if c2 == nil {
		t.Fatalf("expected non-nil cache")
	}

	ctx := context.Background()
	if err := c2.Set(ctx, "test_key", sampleData{Title: "Fallback"}, time.Minute); err != nil {
		t.Fatalf("expected set to succeed on fallback cache: %v", err)
	}
	var out sampleData
	found, err := c2.Get(ctx, "test_key", &out)
	if err != nil || !found || out.Title != "Fallback" {
		t.Fatalf("expected to read from fallback cache, found=%v err=%v", found, err)
	}
}

func TestTwoTierCache_HitMissBackfillAndInvalidation(t *testing.T) {
	ctx := context.Background()
	l1 := NewMemoryCache()
	l2 := NewMemoryCache()
	ttc := NewTwoTierCache(l1, l2, 5*time.Second)

	// 1. Initial miss on empty cache
	var out sampleData
	found, err := ttc.Get(ctx, "k1", &out)
	if err != nil {
		t.Fatalf("unexpected error on initial get: %v", err)
	}
	if found {
		t.Fatalf("expected key to be absent from empty cache, got found=true")
	}

	// 2. Set writes to both L1 and L2
	err = ttc.Set(ctx, "k1", sampleData{Title: "Job Alpha"}, time.Minute)
	if err != nil {
		t.Fatalf("set failed on TwoTierCache: %v", err)
	}

	// Verify L1 has key
	var l1Out sampleData
	found, err = l1.Get(ctx, "k1", &l1Out)
	if err != nil || !found || l1Out.Title != "Job Alpha" {
		t.Fatalf("expected L1 to contain key, found=%v, err=%v, val=%+v", found, err, l1Out)
	}

	// Verify L2 has key
	var l2Out sampleData
	found, err = l2.Get(ctx, "k1", &l2Out)
	if err != nil || !found || l2Out.Title != "Job Alpha" {
		t.Fatalf("expected L2 to contain key, found=%v, err=%v, val=%+v", found, err, l2Out)
	}

	// Verify TwoTierCache.Get returns value (served from L1)
	out = sampleData{}
	found, err = ttc.Get(ctx, "k1", &out)
	if err != nil || !found || out.Title != "Job Alpha" {
		t.Fatalf("expected TwoTierCache to return key, found=%v, err=%v, val=%+v", found, err, out)
	}

	// 3. L1 miss with L2 hit automatically backfills L1
	// Evict from L1 only
	if err := l1.Delete(ctx, "k1"); err != nil {
		t.Fatalf("failed to delete from L1: %v", err)
	}

	// Verify L1 is missing the key
	found, _ = l1.Get(ctx, "k1", &l1Out)
	if found {
		t.Fatalf("expected L1 to NOT contain key after direct deletion")
	}

	// Verify L2 still has the key
	found, _ = l2.Get(ctx, "k1", &l2Out)
	if !found {
		t.Fatalf("expected L2 to still contain key")
	}

	// Query TwoTierCache - should query L2 and backfill L1
	out = sampleData{}
	found, err = ttc.Get(ctx, "k1", &out)
	if err != nil || !found || out.Title != "Job Alpha" {
		t.Fatalf("expected TwoTierCache to return key from L2, found=%v, err=%v", found, err)
	}

	// Verify L1 is now backfilled
	l1Out = sampleData{}
	found, err = l1.Get(ctx, "k1", &l1Out)
	if err != nil || !found || l1Out.Title != "Job Alpha" {
		t.Fatalf("expected L1 to be backfilled after L2 hit, found=%v, val=%+v", found, l1Out)
	}

	// 4. Delete removes from both L1 and L2
	if err := ttc.Delete(ctx, "k1"); err != nil {
		t.Fatalf("failed to delete from TwoTierCache: %v", err)
	}

	found, _ = l1.Get(ctx, "k1", &l1Out)
	if found {
		t.Fatalf("expected L1 to NOT contain key after ttc.Delete")
	}

	found, _ = l2.Get(ctx, "k1", &l2Out)
	if found {
		t.Fatalf("expected L2 to NOT contain key after ttc.Delete")
	}

	found, _ = ttc.Get(ctx, "k1", &out)
	if found {
		t.Fatalf("expected TwoTierCache to NOT return key after ttc.Delete")
	}

	// 5. DeletePattern purges matching keys from both L1 and L2
	_ = ttc.Set(ctx, "jobs:search:1", sampleData{Title: "Job 1"}, time.Minute)
	_ = ttc.Set(ctx, "jobs:search:2", sampleData{Title: "Job 2"}, time.Minute)
	_ = ttc.Set(ctx, "profile:100", sampleData{Title: "Profile 100"}, time.Minute)

	if err := ttc.DeletePattern(ctx, "jobs:search:*"); err != nil {
		t.Fatalf("DeletePattern failed on TwoTierCache: %v", err)
	}

	for _, k := range []string{"jobs:search:1", "jobs:search:2"} {
		if f, _ := l1.Get(ctx, k, &out); f {
			t.Fatalf("expected %s deleted from L1", k)
		}
		if f, _ := l2.Get(ctx, k, &out); f {
			t.Fatalf("expected %s deleted from L2", k)
		}
		if f, _ := ttc.Get(ctx, k, &out); f {
			t.Fatalf("expected %s deleted from ttc", k)
		}
	}

	// Non-matching key remains in both
	if f, _ := l1.Get(ctx, "profile:100", &out); !f {
		t.Fatalf("expected profile:100 to remain in L1")
	}
	if f, _ := l2.Get(ctx, "profile:100", &out); !f {
		t.Fatalf("expected profile:100 to remain in L2")
	}
	if f, _ := ttc.Get(ctx, "profile:100", &out); !f || out.Title != "Profile 100" {
		t.Fatalf("expected profile:100 to remain in ttc")
	}
}

func TestTwoTierCache_OptionsAndAccessors(t *testing.T) {
	ctx := context.Background()
	l1 := NewMemoryCache()
	l2 := NewMemoryCache()

	// Default L1 TTL when <= 0
	ttc := NewTwoTierCache(l1, l2, 0)
	if ttc.l1TTL != 10*time.Second {
		t.Fatalf("expected default l1TTL 10s, got %v", ttc.l1TTL)
	}

	if ttc.L1() != l1 {
		t.Fatalf("expected L1() accessor to return l1")
	}
	if ttc.L2() != l2 {
		t.Fatalf("expected L2() accessor to return l2")
	}

	// Short TTL lower than l1TTL
	shortTTC := NewTwoTierCache(l1, l2, 5*time.Second)
	err := shortTTC.Set(ctx, "short_key", sampleData{Title: "Short"}, 25*time.Millisecond)
	if err != nil {
		t.Fatalf("set failed: %v", err)
	}

	time.Sleep(40 * time.Millisecond)
	var out sampleData
	found, _ := shortTTC.Get(ctx, "short_key", &out)
	if found {
		t.Fatalf("expected short_key to be expired in both tiers")
	}
}

func TestTwoTierCache_NilResilience(t *testing.T) {
	ctx := context.Background()

	// Nil L1 and Nil L2 should not panic
	ttcNil := NewTwoTierCache(nil, nil, 10*time.Second)

	var out sampleData
	found, err := ttcNil.Get(ctx, "key", &out)
	if err != nil || found {
		t.Fatalf("expected nil cache Get to return (false, nil), got (%v, %v)", found, err)
	}

	if err := ttcNil.Set(ctx, "key", sampleData{Title: "Test"}, time.Minute); err != nil {
		t.Fatalf("expected nil cache Set to return nil, got %v", err)
	}

	if err := ttcNil.Delete(ctx, "key"); err != nil {
		t.Fatalf("expected nil cache Delete to return nil, got %v", err)
	}

	if err := ttcNil.DeletePattern(ctx, "*"); err != nil {
		t.Fatalf("expected nil cache DeletePattern to return nil, got %v", err)
	}
}

