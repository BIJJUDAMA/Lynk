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
