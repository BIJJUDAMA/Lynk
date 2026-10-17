package project_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/httpx"
	"github.com/lynk/backend/internal/project"
)

type mockProjectRepo struct {
	mu       sync.RWMutex
	projects map[uuid.UUID]*project.Project
}

func newMockProjectRepo() *mockProjectRepo {
	return &mockProjectRepo{
		projects: make(map[uuid.UUID]*project.Project),
	}
}

func (m *mockProjectRepo) CreateProject(ctx context.Context, p *project.Project) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.projects {
		if existing.Slug == p.Slug {
			return project.ErrDuplicateSlug
		}
	}

	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = now
	}

	copy := *p
	m.projects[p.ID] = &copy
	return nil
}

func (m *mockProjectRepo) GetProjectByID(ctx context.Context, id uuid.UUID) (*project.Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.projects[id]
	if !ok {
		return nil, project.ErrProjectNotFound
	}
	copy := *p
	return &copy, nil
}

func (m *mockProjectRepo) GetProjectBySlug(ctx context.Context, slug string) (*project.Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, p := range m.projects {
		if p.Slug == slug {
			copy := *p
			return &copy, nil
		}
	}
	return nil, project.ErrProjectNotFound
}

func (m *mockProjectRepo) ListProjects(ctx context.Context, filter project.ListFilter) ([]project.Project, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []project.Project
	for _, p := range m.projects {
		if filter.Visibility != "" && p.Visibility != filter.Visibility {
			continue
		}
		if filter.RecruitmentStatus != "" && p.RecruitmentStatus != filter.RecruitmentStatus {
			continue
		}
		if filter.LifecycleStatus != "" && p.LifecycleStatus != filter.LifecycleStatus {
			continue
		}
		if filter.Category != "" && p.Category != filter.Category {
			continue
		}
		if filter.OwnerMemberID != "" && p.OwnerMemberID != filter.OwnerMemberID {
			continue
		}
		if filter.SearchQuery != "" {
			q := strings.ToLower(filter.SearchQuery)
			t := strings.ToLower(p.Title)
			d := strings.ToLower(p.Description)
			if !strings.Contains(t, q) && !strings.Contains(d, q) {
				continue
			}
		}
		list = append(list, *p)
	}

	total := len(list)
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	if offset >= len(list) {
		return []project.Project{}, total, nil
	}
	end := offset + limit
	if end > len(list) {
		end = len(list)
	}

	return list[offset:end], total, nil
}

func (m *mockProjectRepo) UpdateProject(ctx context.Context, p *project.Project) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.projects[p.ID]; !ok {
		return project.ErrProjectNotFound
	}
	copy := *p
	copy.UpdatedAt = time.Now().UTC()
	m.projects[p.ID] = &copy
	return nil
}

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Autonomous Drone Mapping", "autonomous-drone-mapping"},
		{"Robotics & AI 2026: Next-Gen!", "robotics-ai-2026-next-gen"},
		{"   --Spaced Out Title--   ", "spaced-out-title"},
		{"Multi---Dash---Cleanup", "multi-dash-cleanup"},
	}

	for _, tt := range tests {
		got := project.GenerateSlug(tt.input)
		if got != tt.expected {
			t.Errorf("GenerateSlug(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}

	fallback := project.GenerateSlug("???")
	if !strings.HasPrefix(fallback, "project-") {
		t.Errorf("expected fallback slug starting with project-, got %q", fallback)
	}
}

func TestProjectService_CreateAndRetrieve(t *testing.T) {
	repo := newMockProjectRepo()
	svc := project.NewService(repo)

	ownerID := "user-student-1"
	created, err := svc.CreateProject(context.Background(), ownerID, project.CreateProjectInput{
		Title:       "Autonomous Drone Mapping",
		Description: "Mapping campus buildings with computer vision.",
		Category:    "Robotics",
		Visibility:  project.VisibilityPublic,
	})
	if err != nil {
		t.Fatalf("unexpected error creating project: %v", err)
	}

	if created.Slug != "autonomous-drone-mapping" {
		t.Errorf("expected slug 'autonomous-drone-mapping', got %q", created.Slug)
	}
	if created.OwnerMemberID != ownerID {
		t.Errorf("expected owner %q, got %q", ownerID, created.OwnerMemberID)
	}
	if created.RecruitmentStatus != project.RecruitmentOpen {
		t.Errorf("expected default recruitment open, got %q", created.RecruitmentStatus)
	}
	if created.LifecycleStatus != project.LifecycleActive {
		t.Errorf("expected default lifecycle active, got %q", created.LifecycleStatus)
	}

	fetchedBySlug, err := svc.GetProjectBySlug(context.Background(), "autonomous-drone-mapping", nil)
	if err != nil {
		t.Fatalf("unexpected error fetching project by slug: %v", err)
	}
	if fetchedBySlug.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, fetchedBySlug.ID)
	}

	fetchedByID, err := svc.GetProjectByID(context.Background(), created.ID, nil)
	if err != nil {
		t.Fatalf("unexpected error fetching project by ID: %v", err)
	}
	if fetchedByID.Slug != created.Slug {
		t.Errorf("expected Slug %s, got %s", created.Slug, fetchedByID.Slug)
	}
}

func TestProjectService_SlugCollisionHandling(t *testing.T) {
	repo := newMockProjectRepo()
	svc := project.NewService(repo)

	ownerID := "user-student-1"
	p1, err := svc.CreateProject(context.Background(), ownerID, project.CreateProjectInput{
		Title:       "Smart Campus Parking",
		Description: "Real-time parking spot occupancy tracking.",
	})
	if err != nil {
		t.Fatalf("unexpected error creating p1: %v", err)
	}
	if p1.Slug != "smart-campus-parking" {
		t.Errorf("expected slug smart-campus-parking, got %q", p1.Slug)
	}

	p2, err := svc.CreateProject(context.Background(), ownerID, project.CreateProjectInput{
		Title:       "Smart Campus Parking",
		Description: "Another parking management initiative.",
	})
	if err != nil {
		t.Fatalf("unexpected error creating p2: %v", err)
	}
	if p2.Slug != "smart-campus-parking-2" {
		t.Errorf("expected collision resolution slug smart-campus-parking-2, got %q", p2.Slug)
	}

	p3, err := svc.CreateProject(context.Background(), ownerID, project.CreateProjectInput{
		Title:       "Smart Campus Parking",
		Description: "Third parking management initiative.",
	})
	if err != nil {
		t.Fatalf("unexpected error creating p3: %v", err)
	}
	if p3.Slug != "smart-campus-parking-3" {
		t.Errorf("expected collision resolution slug smart-campus-parking-3, got %q", p3.Slug)
	}
}

func TestProjectService_ValidationErrors(t *testing.T) {
	repo := newMockProjectRepo()
	svc := project.NewService(repo)

	ctx := context.Background()
	ownerID := "user-123"

	// Unauthorized (empty owner)
	_, err := svc.CreateProject(ctx, "", project.CreateProjectInput{
		Title:       "Valid Title",
		Description: "Valid description of sufficient length.",
	})
	if err != project.ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}

	// Title too short
	_, err = svc.CreateProject(ctx, ownerID, project.CreateProjectInput{
		Title:       "AI",
		Description: "Valid description of sufficient length.",
	})
	if err != project.ErrInvalidTitle {
		t.Errorf("expected ErrInvalidTitle, got %v", err)
	}

	// Title too long
	_, err = svc.CreateProject(ctx, ownerID, project.CreateProjectInput{
		Title:       strings.Repeat("A", 201),
		Description: "Valid description of sufficient length.",
	})
	if err != project.ErrInvalidTitle {
		t.Errorf("expected ErrInvalidTitle, got %v", err)
	}

	// Description too short
	_, err = svc.CreateProject(ctx, ownerID, project.CreateProjectInput{
		Title:       "Valid Title",
		Description: "Too short",
	})
	if err != project.ErrInvalidDescription {
		t.Errorf("expected ErrInvalidDescription, got %v", err)
	}

	// Invalid visibility
	_, err = svc.CreateProject(ctx, ownerID, project.CreateProjectInput{
		Title:       "Valid Title",
		Description: "Valid description of sufficient length.",
		Visibility:  project.Visibility("secret"),
	})
	if err != project.ErrInvalidVisibility {
		t.Errorf("expected ErrInvalidVisibility, got %v", err)
	}
}

func TestProjectService_VisibilityAccessRules(t *testing.T) {
	repo := newMockProjectRepo()
	svc := project.NewService(repo)
	ctx := context.Background()

	ownerID := "owner-user"
	otherUser := "other-user"

	// 1. Private project
	privProj, err := svc.CreateProject(ctx, ownerID, project.CreateProjectInput{
		Title:       "Confidential Research Initiative",
		Description: "Proprietary algorithm development for security.",
		Visibility:  project.VisibilityPrivate,
	})
	if err != nil {
		t.Fatalf("failed to create private project: %v", err)
	}

	// Anonymous viewer -> denied
	_, err = svc.GetProjectBySlug(ctx, privProj.Slug, nil)
	if err != project.ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized for anonymous viewer of private project, got %v", err)
	}

	// Other user -> denied
	_, err = svc.GetProjectBySlug(ctx, privProj.Slug, &otherUser)
	if err != project.ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized for other user of private project, got %v", err)
	}

	// Owner -> allowed
	fetched, err := svc.GetProjectBySlug(ctx, privProj.Slug, &ownerID)
	if err != nil {
		t.Errorf("expected owner access to private project, got error %v", err)
	}
	if fetched.ID != privProj.ID {
		t.Errorf("expected project ID %s, got %s", privProj.ID, fetched.ID)
	}

	// 2. Unlisted project
	unlistedProj, err := svc.CreateProject(ctx, ownerID, project.CreateProjectInput{
		Title:       "Unlisted Workshop Draft",
		Description: "Draft workshop not listed on main directory.",
		Visibility:  project.VisibilityUnlisted,
	})
	if err != nil {
		t.Fatalf("failed to create unlisted project: %v", err)
	}

	// Anyone knowing slug can access unlisted project
	fetchedUnlisted, err := svc.GetProjectBySlug(ctx, unlistedProj.Slug, nil)
	if err != nil {
		t.Errorf("expected anonymous access to unlisted project with slug, got error %v", err)
	}
	if fetchedUnlisted.ID != unlistedProj.ID {
		t.Errorf("expected project ID %s, got %s", unlistedProj.ID, fetchedUnlisted.ID)
	}
}

func TestProjectService_ListProjects(t *testing.T) {
	repo := newMockProjectRepo()
	svc := project.NewService(repo)
	ctx := context.Background()

	owner1 := "owner-1"
	owner2 := "owner-2"

	_, _ = svc.CreateProject(ctx, owner1, project.CreateProjectInput{
		Title:       "Public Open Source Robotics",
		Description: "Autonomous systems library for campus robotics team.",
		Category:    "Robotics",
		Visibility:  project.VisibilityPublic,
	})
	_, _ = svc.CreateProject(ctx, owner1, project.CreateProjectInput{
		Title:       "Private Stealth Project",
		Description: "Undercover startup preparation for tech incubator.",
		Category:    "Software",
		Visibility:  project.VisibilityPrivate,
	})
	_, _ = svc.CreateProject(ctx, owner2, project.CreateProjectInput{
		Title:       "Public Web Framework",
		Description: "Lightweight frontend toolkit for university clubs.",
		Category:    "Software",
		Visibility:  project.VisibilityPublic,
	})

	// Anonymous listing: only sees public projects (count 2)
	list, total, err := svc.ListProjects(ctx, project.ListFilter{}, nil)
	if err != nil {
		t.Fatalf("unexpected error listing projects: %v", err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("expected 2 public projects for anonymous, got total=%d len=%d", total, len(list))
	}

	// Filter by category
	robList, totalRob, err := svc.ListProjects(ctx, project.ListFilter{Category: "Robotics"}, nil)
	if err != nil {
		t.Fatalf("unexpected error listing robotics: %v", err)
	}
	if totalRob != 1 || len(robList) != 1 {
		t.Fatalf("expected 1 robotics project, got %d", totalRob)
	}

	// Owner 1 listing private projects: sees own private project
	privList, privTotal, err := svc.ListProjects(ctx, project.ListFilter{Visibility: project.VisibilityPrivate}, &owner1)
	if err != nil {
		t.Fatalf("unexpected error listing private projects: %v", err)
	}
	if privTotal != 1 || len(privList) != 1 {
		t.Fatalf("expected 1 private project for owner1, got %d", privTotal)
	}

	// Anonymous listing private projects: sees 0
	anonPrivList, anonPrivTotal, err := svc.ListProjects(ctx, project.ListFilter{Visibility: project.VisibilityPrivate}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if anonPrivTotal != 0 || len(anonPrivList) != 0 {
		t.Fatalf("expected 0 private projects for anonymous, got %d", anonPrivTotal)
	}
}

func TestProjectService_UpdateAndVisibility(t *testing.T) {
	repo := newMockProjectRepo()
	svc := project.NewService(repo)
	ctx := context.Background()

	owner := "owner-student"
	nonOwner := "hacker-student"

	p, err := svc.CreateProject(ctx, owner, project.CreateProjectInput{
		Title:       "Initial Project Title",
		Description: "Initial description describing the project goals.",
		Category:    "Design",
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Unauthorized update attempt
	newTitle := "Hacked Title"
	_, err = svc.UpdateProject(ctx, p.ID, nonOwner, project.UpdateProjectInput{
		Title: &newTitle,
	})
	if err != project.ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized for non-owner update, got %v", err)
	}

	// Authorized update
	updatedTitle := "Redesigned Campus Experience"
	updatedDesc := "Completely rewritten description of the redesign project."
	newRecruitment := project.RecruitmentPaused
	updated, err := svc.UpdateProject(ctx, p.ID, owner, project.UpdateProjectInput{
		Title:             &updatedTitle,
		Description:       &updatedDesc,
		RecruitmentStatus: &newRecruitment,
	})
	if err != nil {
		t.Fatalf("failed to update project: %v", err)
	}
	if updated.Title != updatedTitle {
		t.Errorf("expected title %q, got %q", updatedTitle, updated.Title)
	}
	if updated.RecruitmentStatus != project.RecruitmentPaused {
		t.Errorf("expected recruitment paused, got %q", updated.RecruitmentStatus)
	}

	// Visibility update: non-owner rejected
	_, err = svc.UpdateVisibility(ctx, p.ID, nonOwner, project.VisibilityPrivate)
	if err != project.ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized for non-owner visibility update, got %v", err)
	}

	// Visibility update: owner changes to private
	visUpdated, err := svc.UpdateVisibility(ctx, p.ID, owner, project.VisibilityPrivate)
	if err != nil {
		t.Fatalf("failed to update visibility: %v", err)
	}
	if visUpdated.Visibility != project.VisibilityPrivate {
		t.Errorf("expected visibility private, got %q", visUpdated.Visibility)
	}

	// Visibility update: invalid enum rejected
	_, err = svc.UpdateVisibility(ctx, p.ID, owner, project.Visibility("unknown_vis"))
	if err != project.ErrInvalidVisibility {
		t.Errorf("expected ErrInvalidVisibility, got %v", err)
	}
}

func TestProjectHandler_HTTPFlow(t *testing.T) {
	repo := newMockProjectRepo()
	svc := project.NewService(repo)
	handler := project.NewHandler(svc, repo)

	authMiddleware := httpx.RequireAuthFromContext()
	router := handler.Routes(authMiddleware)

	verifiedUser := &auth.UserClaims{
		UserID:        "user-verified-123",
		Email:         "alex@stanford.edu",
		EmailVerified: true,
	}

	unverifiedUser := &auth.UserClaims{
		UserID:        "user-unverified-456",
		Email:         "unverified@mit.edu",
		EmailVerified: false,
	}

	// 1. Create project without credentials -> 401
	payload := `{"title":"Autonomous Navigation System","description":"Developing lidar-based mapping algorithms."}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(payload))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated create, got %d", rec.Code)
	}

	// 2. Create project with unverified email -> 403 EMAIL_NOT_VERIFIED
	req = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(payload))
	req = req.WithContext(auth.WithUserContext(req.Context(), unverifiedUser))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for unverified user create, got %d", rec.Code)
	}

	// 3. Create project with verified credentials -> 201
	req = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(payload))
	req = req.WithContext(auth.WithUserContext(req.Context(), verifiedUser))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for verified user create, got %d: %s", rec.Code, rec.Body.String())
	}

	var createResp struct {
		Success bool            `json:"success"`
		Data    project.Project `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&createResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	createdProject := createResp.Data
	if createdProject.Slug != "autonomous-navigation-system" {
		t.Errorf("expected slug autonomous-navigation-system, got %q", createdProject.Slug)
	}

	// 4. List projects -> 200
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for list projects, got %d", rec.Code)
	}

	// 5. Get project by slug -> 200
	req = httptest.NewRequest(http.MethodGet, "/"+createdProject.Slug, nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for get project by slug, got %d", rec.Code)
	}

	// 6. Get project by UUID -> 200
	req = httptest.NewRequest(http.MethodGet, "/"+createdProject.ID.String(), nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for get project by UUID, got %d", rec.Code)
	}

	// 7. Get nonexistent project -> 404
	req = httptest.NewRequest(http.MethodGet, "/nonexistent-project-slug", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent project, got %d", rec.Code)
	}

	// 8. Update project by non-owner -> 403
	updatePayload := `{"title":"Hacked Navigation"}`
	req = httptest.NewRequest(http.MethodPatch, "/"+createdProject.ID.String(), bytes.NewBufferString(updatePayload))
	req = req.WithContext(auth.WithUserContext(req.Context(), &auth.UserClaims{UserID: "other-user", EmailVerified: true}))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-owner update, got %d", rec.Code)
	}

	// 9. Update project by owner -> 200
	validUpdate := `{"title":"Advanced Autonomous Navigation System"}`
	req = httptest.NewRequest(http.MethodPatch, "/"+createdProject.ID.String(), bytes.NewBufferString(validUpdate))
	req = req.WithContext(auth.WithUserContext(req.Context(), verifiedUser))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for owner update, got %d: %s", rec.Code, rec.Body.String())
	}

	// 10. Update visibility by owner -> 200
	visPayload := `{"visibility":"private"}`
	req = httptest.NewRequest(http.MethodPatch, "/"+createdProject.ID.String()+"/visibility", bytes.NewBufferString(visPayload))
	req = req.WithContext(auth.WithUserContext(req.Context(), verifiedUser))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for owner visibility update, got %d: %s", rec.Code, rec.Body.String())
	}

	// 11. Anonymous access to now-private project -> 401
	req = httptest.NewRequest(http.MethodGet, "/"+createdProject.Slug, nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for anonymous access to private project, got %d", rec.Code)
	}

	// 12. Non-owner access to private project -> 403
	req = httptest.NewRequest(http.MethodGet, "/"+createdProject.Slug, nil)
	req = req.WithContext(auth.WithUserContext(req.Context(), &auth.UserClaims{UserID: "random-user", EmailVerified: true}))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-owner access to private project, got %d", rec.Code)
	}
}

func TestPgxRepository_Integration(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://lynk_user:lynk_password@localhost:5432/lynk_db?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping integration test: database connection failed: %v", err)
		return
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("skipping integration test: database ping failed: %v", err)
		return
	}

	repo := project.NewRepository(pool)
	testUserID := "st_proj_user_" + uuid.New().String()[:8]
	_, err = pool.Exec(ctx, `INSERT INTO users (id, email, role) VALUES ($1, $2, 'member')`, testUserID, testUserID+"@test.edu")
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, testUserID)
	}()

	// 1. Create project
	projID := uuid.New()
	slug := "integration-test-proj-" + uuid.New().String()[:8]
	p := &project.Project{
		ID:                projID,
		Slug:              slug,
		OwnerMemberID:     testUserID,
		Title:             "Integration Test Project",
		Description:       "Real PostgreSQL integration test project verifying schema and repository.",
		Category:          "Engineering",
		Visibility:        project.VisibilityPublic,
		RecruitmentStatus: project.RecruitmentOpen,
		LifecycleStatus:   project.LifecycleActive,
		Settings:          map[string]any{"theme": "dark", "features": []any{"tasks", "wiki"}},
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}

	if err := repo.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id = $1`, projID)
	}()

	// 2. Get by ID
	byID, err := repo.GetProjectByID(ctx, projID)
	if err != nil {
		t.Fatalf("GetProjectByID failed: %v", err)
	}
	if byID.Title != p.Title {
		t.Errorf("expected title %q, got %q", p.Title, byID.Title)
	}
	if byID.Settings["theme"] != "dark" {
		t.Errorf("expected settings theme dark, got %v", byID.Settings["theme"])
	}

	// 3. Get by Slug
	bySlug, err := repo.GetProjectBySlug(ctx, slug)
	if err != nil {
		t.Fatalf("GetProjectBySlug failed: %v", err)
	}
	if bySlug.ID != projID {
		t.Errorf("expected ID %s, got %s", projID, bySlug.ID)
	}

	// 4. List projects
	list, total, err := repo.ListProjects(ctx, project.ListFilter{
		SearchQuery: "Integration Test Project",
	})
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	if total < 1 || len(list) < 1 {
		t.Fatalf("expected at least 1 project in list, got total=%d len=%d", total, len(list))
	}

	// 5. Update project
	byID.Title = "Updated Integration Test Project"
	byID.LifecycleStatus = project.LifecycleCompleted
	if err := repo.UpdateProject(ctx, byID); err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}

	updated, err := repo.GetProjectByID(ctx, projID)
	if err != nil {
		t.Fatalf("GetProjectByID after update failed: %v", err)
	}
	if updated.Title != "Updated Integration Test Project" {
		t.Errorf("expected updated title, got %q", updated.Title)
	}
	if updated.LifecycleStatus != project.LifecycleCompleted {
		t.Errorf("expected status completed, got %q", updated.LifecycleStatus)
	}

	// 6. Duplicate slug error check
	dup := &project.Project{
		ID:                uuid.New(),
		Slug:              slug,
		OwnerMemberID:     testUserID,
		Title:             "Duplicate Slug Project",
		Description:       "Should fail due to duplicate slug constraint in PostgreSQL.",
		Category:          "Engineering",
		Visibility:        project.VisibilityPublic,
		RecruitmentStatus: project.RecruitmentOpen,
		LifecycleStatus:   project.LifecycleActive,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}
	err = repo.CreateProject(ctx, dup)
	if !errors.Is(err, project.ErrDuplicateSlug) {
		t.Errorf("expected ErrDuplicateSlug on constraint violation, got %v", err)
	}
}
