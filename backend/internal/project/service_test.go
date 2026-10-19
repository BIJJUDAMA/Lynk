package project_test

import (
	"context"
	"testing"

	"github.com/lynk/backend/internal/cache"
	"github.com/lynk/backend/internal/project"
)

func TestProjectService_CacheAside(t *testing.T) {
	repo := newMockProjectRepo()
	memCache := cache.NewMemoryCache()
	svc := project.NewService(repo).WithCache(memCache)

	ctx := context.Background()
	ownerID := "usr_cache_test"
	p, err := svc.CreateProject(ctx, ownerID, project.CreateProjectInput{
		Title:       "Distributed Cache Node",
		Description: "High-performance cache node for multi-tenant campus workloads.",
		Category:    "Engineering",
		Visibility:  project.VisibilityPublic,
	})
	if err != nil {
		t.Fatalf("unexpected error creating project: %v", err)
	}

	// 1. Initial list: populates cache
	list1, err := svc.ListPublicProjects(ctx, project.ListFilter{Limit: 10})
	if err != nil || len(list1) == 0 {
		t.Fatalf("expected non-empty list, got %v (err: %v)", list1, err)
	}

	// Verify key exists in cache
	var cached []project.Project
	cacheKey := "projects:list::open:10:0"
	ok, err := memCache.Get(ctx, cacheKey, &cached)
	if err != nil || !ok || len(cached) == 0 {
		t.Fatalf("expected cached project list, got ok=%v, err=%v", ok, err)
	}

	// Verify cache hit on second list call
	listHit, err := svc.ListPublicProjects(ctx, project.ListFilter{Limit: 10})
	if err != nil || len(listHit) != len(list1) {
		t.Fatalf("expected list cache hit, got %v (err: %v)", listHit, err)
	}

	// 2. Initial detail: populates cache
	detail1, err := svc.GetProject(ctx, p.Slug)
	if err != nil || detail1.Title != "Distributed Cache Node" {
		t.Fatalf("unexpected detail: %v (err: %v)", detail1, err)
	}

	var cachedDetail project.Project
	okDetail, _ := memCache.Get(ctx, "projects:detail:"+p.Slug, &cachedDetail)
	if !okDetail || cachedDetail.ID != p.ID {
		t.Fatalf("expected cached detail by slug, got ok=%v", okDetail)
	}

	var cachedDetailByID project.Project
	okDetailByID, _ := memCache.Get(ctx, "projects:detail:"+p.ID.String(), &cachedDetailByID)
	if !okDetailByID || cachedDetailByID.ID != p.ID {
		t.Fatalf("expected cached detail by id, got ok=%v", okDetailByID)
	}

	// Verify cache hit
	hitDetail, err := svc.GetProject(ctx, p.Slug)
	if err != nil || hitDetail.ID != p.ID {
		t.Fatalf("expected cached detail hit, got %v (err: %v)", hitDetail, err)
	}

	// 3. Update project: invalidates cache
	newDesc := "Updated cache node description exceeding min length requirement."
	_, err = svc.UpdateProject(ctx, p.ID, ownerID, project.UpdateProjectInput{
		Description: &newDesc,
	})
	if err != nil {
		t.Fatalf("unexpected error updating project: %v", err)
	}

	// List cache should be invalidated
	var invalidatedList []project.Project
	okListAfter, _ := memCache.Get(ctx, cacheKey, &invalidatedList)
	if okListAfter {
		t.Fatal("expected list cache to be invalidated after project update")
	}

	// Detail cache should be invalidated
	var invalidatedDetail project.Project
	okDetailAfter, _ := memCache.Get(ctx, "projects:detail:"+p.Slug, &invalidatedDetail)
	if okDetailAfter {
		t.Fatal("expected detail cache by slug to be invalidated after project update")
	}

	var invalidatedDetailByID project.Project
	okDetailByIDAfter, _ := memCache.Get(ctx, "projects:detail:"+p.ID.String(), &invalidatedDetailByID)
	if okDetailByIDAfter {
		t.Fatal("expected detail cache by id to be invalidated after project update")
	}

	// 4. Invalidation on CreateProject
	_, _ = svc.ListPublicProjects(ctx, project.ListFilter{Limit: 10})
	p2, err := svc.CreateProject(ctx, ownerID, project.CreateProjectInput{
		Title:       "Second Distributed Project",
		Description: "Second project description exceeding minimum length requirement.",
		Category:    "Engineering",
		Visibility:  project.VisibilityPublic,
	})
	if err != nil {
		t.Fatalf("unexpected error creating second project: %v", err)
	}
	var invalidatedList2 []project.Project
	okListAfterCreate, _ := memCache.Get(ctx, cacheKey, &invalidatedList2)
	if okListAfterCreate {
		t.Fatal("expected list cache to be invalidated after project create")
	}

	// 5. Invalidation on DeleteProject
	_, _ = svc.ListPublicProjects(ctx, project.ListFilter{Limit: 10})
	if err := svc.DeleteProject(ctx, p2.ID, ownerID); err != nil {
		t.Fatalf("unexpected error deleting project: %v", err)
	}
	var invalidatedList3 []project.Project
	okListAfterDelete, _ := memCache.Get(ctx, cacheKey, &invalidatedList3)
	if okListAfterDelete {
		t.Fatal("expected list cache to be invalidated after project delete")
	}

	// 6. Service without cache operates transparently
	svcNoCache := project.NewService(repo)
	noCacheList, err := svcNoCache.ListPublicProjects(ctx, project.ListFilter{Limit: 10})
	if err != nil || len(noCacheList) == 0 {
		t.Fatalf("expected list without cache to succeed, got %v (err: %v)", noCacheList, err)
	}
	noCacheDetail, err := svcNoCache.GetProject(ctx, p.Slug)
	if err != nil || noCacheDetail.ID != p.ID {
		t.Fatalf("expected get without cache to succeed, got %v (err: %v)", noCacheDetail, err)
	}
}
