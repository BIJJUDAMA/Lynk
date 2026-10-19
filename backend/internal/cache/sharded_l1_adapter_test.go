package cache

import (
	"context"
	"testing"
	"time"
)

type adapterPayload struct {
	Name string `json:"name"`
	N    int    `json:"n"`
}

func TestShardedL1Cacher_SetGetRoundTrip(t *testing.T) {
	c := NewShardedL1Cacher(NewShardedL1Cache(4, time.Minute))
	ctx := context.Background()

	want := adapterPayload{Name: "lynk", N: 42}
	if err := c.Set(ctx, "k1", want, time.Minute); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	var got adapterPayload
	ok, err := c.Get(ctx, "k1", &got)
	if err != nil || !ok {
		t.Fatalf("Get failed: ok=%v err=%v", ok, err)
	}
	if got != want {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, want)
	}

	var missing adapterPayload
	ok, err = c.Get(ctx, "absent", &missing)
	if err != nil {
		t.Fatalf("Get on absent key returned error: %v", err)
	}
	if ok {
		t.Fatal("expected miss for absent key")
	}
}

func TestShardedL1Cacher_Delete(t *testing.T) {
	c := NewShardedL1Cacher(NewShardedL1Cache(4, time.Minute))
	ctx := context.Background()

	for _, k := range []string{"a", "b", "c"} {
		if err := c.Set(ctx, k, adapterPayload{Name: k}, time.Minute); err != nil {
			t.Fatalf("Set %s failed: %v", k, err)
		}
	}

	if err := c.Delete(ctx, "a", "b"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	for _, k := range []string{"a", "b"} {
		var out adapterPayload
		ok, err := c.Get(ctx, k, &out)
		if err != nil {
			t.Fatalf("Get %s failed: %v", k, err)
		}
		if ok {
			t.Fatalf("expected key %s to be deleted", k)
		}
	}

	var survivor adapterPayload
	ok, err := c.Get(ctx, "c", &survivor)
	if err != nil || !ok {
		t.Fatalf("expected key c to survive Delete: ok=%v err=%v", ok, err)
	}
}

func TestShardedL1Cacher_DeletePattern(t *testing.T) {
	ctx := context.Background()
	inner := NewShardedL1Cache(4, time.Minute)
	c := NewShardedL1Cacher(inner)
	_ = c.Set(ctx, "projects:slug:a", 1, time.Minute)
	_ = c.Set(ctx, "projects:slug:b", 2, time.Minute)
	_ = c.Set(ctx, "jobs:search:1", 3, time.Minute)

	if err := c.DeletePattern(ctx, "projects:slug:*"); err != nil {
		t.Fatalf("DeletePattern prefix: %v", err)
	}
	if _, found, _ := inner.Get(ctx, "projects:slug:a"); found {
		t.Fatal("prefix pattern did not evict projects:slug:a")
	}
	if _, found, _ := inner.Get(ctx, "jobs:search:1"); !found {
		t.Fatal("prefix pattern evicted an unrelated key")
	}
	if err := c.DeletePattern(ctx, "jobs:search:1"); err != nil {
		t.Fatalf("DeletePattern exact: %v", err)
	}
	if _, found, _ := inner.Get(ctx, "jobs:search:1"); found {
		t.Fatal("exact pattern did not evict jobs:search:1")
	}
}

func TestShardedL1Cacher_AssignableToCacher(t *testing.T) {
	var _ Cacher = NewShardedL1Cacher(NewShardedL1Cache(1, time.Minute))
}
