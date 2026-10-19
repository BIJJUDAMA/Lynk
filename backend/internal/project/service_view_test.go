package project_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lynk/backend/internal/cache"
	"github.com/lynk/backend/internal/project"
)

func TestService_GetProjectBySlug_RecordsView(t *testing.T) {
	var mu sync.Mutex
	var flushed map[uuid.UUID]int64
	batcher := project.NewViewBatcher(time.Hour, func(ctx context.Context, batch map[uuid.UUID]int64) error {
		mu.Lock()
		defer mu.Unlock()
		flushed = batch
		return nil
	})

	repo := newMockProjectRepo()
	ctx := context.Background()
	ownerID := "usr_view_counter_test"
	p, err := project.NewService(repo).CreateProject(ctx, ownerID, project.CreateProjectInput{
		Title:       "View Counter Public Project",
		Description: "Public project used to verify that page views are batched and recorded.",
		Category:    "Engineering",
		Visibility:  project.VisibilityPublic,
	})
	if err != nil {
		t.Fatalf("unexpected error creating project: %v", err)
	}

	svc := project.NewService(repo).WithViewBatcher(batcher)
	if _, err := svc.GetProjectBySlug(ctx, p.Slug); err != nil {
		t.Fatalf("unexpected error getting project by slug: %v", err)
	}

	if err := batcher.Flush(ctx); err != nil {
		t.Fatalf("unexpected error flushing batcher: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if flushed == nil {
		t.Fatal("expected a flushed view batch, got none")
	}
	if got := flushed[p.ID]; got != 1 {
		t.Fatalf("expected 1 recorded view for project %s, got %d", p.ID, got)
	}
}

func TestService_GetProjectBySlug_RecordsViewOnCacheHit(t *testing.T) {
	var mu sync.Mutex
	var flushed map[uuid.UUID]int64
	batcher := project.NewViewBatcher(time.Hour, func(ctx context.Context, batch map[uuid.UUID]int64) error {
		mu.Lock()
		defer mu.Unlock()
		flushed = batch
		return nil
	})

	repo := newMockProjectRepo()
	ctx := context.Background()
	ownerID := "usr_view_cache_hit_test"
	p, err := project.NewService(repo).CreateProject(ctx, ownerID, project.CreateProjectInput{
		Title:       "View Counter Cache Hit Project",
		Description: "Public project used to verify that cached lookups still record a view.",
		Category:    "Engineering",
		Visibility:  project.VisibilityPublic,
	})
	if err != nil {
		t.Fatalf("unexpected error creating project: %v", err)
	}

	memCache := cache.NewMemoryCache()
	svc := project.NewService(repo).WithCache(memCache).WithViewBatcher(batcher)

	if _, err := svc.GetProjectBySlug(ctx, p.Slug); err != nil {
		t.Fatalf("unexpected error on first (DB) lookup: %v", err)
	}
	if _, err := svc.GetProjectBySlug(ctx, p.Slug); err != nil {
		t.Fatalf("unexpected error on second (cache hit) lookup: %v", err)
	}

	if err := batcher.Flush(ctx); err != nil {
		t.Fatalf("unexpected error flushing batcher: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if got := flushed[p.ID]; got != 2 {
		t.Fatalf("expected 2 recorded views (DB path + cache hit) for project %s, got %d", p.ID, got)
	}
}

func TestService_GetProjectBySlug_DoesNotRecordPrivateView(t *testing.T) {
	var mu sync.Mutex
	var flushed map[uuid.UUID]int64
	batcher := project.NewViewBatcher(time.Hour, func(ctx context.Context, batch map[uuid.UUID]int64) error {
		mu.Lock()
		defer mu.Unlock()
		flushed = batch
		return nil
	})

	repo := newMockProjectRepo()
	ctx := context.Background()
	ownerID := "usr_private_view_test"
	p, err := project.NewService(repo).CreateProject(ctx, ownerID, project.CreateProjectInput{
		Title:       "Private View Counter Project",
		Description: "Private project used to verify that unauthorized lookups record no view.",
		Category:    "Engineering",
		Visibility:  project.VisibilityPrivate,
	})
	if err != nil {
		t.Fatalf("unexpected error creating project: %v", err)
	}

	svc := project.NewService(repo).WithViewBatcher(batcher)

	stranger := "usr_someone_else"
	if _, err := svc.GetProjectBySlug(ctx, p.Slug, &stranger); err != project.ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized for private project, got %v", err)
	}

	if err := batcher.Flush(ctx); err != nil {
		t.Fatalf("unexpected error flushing batcher: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(flushed) != 0 {
		t.Fatalf("expected empty batch for private project, got %v", flushed)
	}
}
