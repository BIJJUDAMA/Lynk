package user_test

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
	"github.com/lynk/backend/internal/cache"
	"github.com/lynk/backend/internal/user"
)

// mockUserRepository is an in-memory thread-safe implementation of user.UserRepository for testing.
type mockUserRepository struct {
	mu       sync.RWMutex
	users    map[string]*user.User
	profiles map[string]*user.Profile // keyed by user_id

	lockCalls          int
	errWithProfileLock error

	errUpsertUser     error
	errGetUser        error
	errGetProfile     error
	errGetProfileByID error
	errUpsertProfile  error
	errUpdateResume   error
	errProvisionUser  error

	referencedResumes     map[string]bool
	errIsResumeReferenced error
}

func newMockUserRepository() *mockUserRepository {
	return &mockUserRepository{
		users:             make(map[string]*user.User),
		profiles:          make(map[string]*user.Profile),
		referencedResumes: make(map[string]bool),
	}
}

var _ user.UserRepository = (*mockUserRepository)(nil)

func (m *mockUserRepository) UpsertUser(ctx context.Context, u *user.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.errUpsertUser != nil {
		return m.errUpsertUser
	}
	now := time.Now()
	if existing, ok := m.users[u.ID]; ok {
		u.CreatedAt = existing.CreatedAt
	} else if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	u.UpdatedAt = now
	userCopy := *u
	m.users[u.ID] = &userCopy
	return nil
}

func (m *mockUserRepository) GetUserByID(ctx context.Context, id string) (*user.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.errGetUser != nil {
		return nil, m.errGetUser
	}
	u, ok := m.users[id]
	if !ok {
		return nil, nil
	}
	userCopy := *u
	return &userCopy, nil
}

func (m *mockUserRepository) GetProfile(ctx context.Context, userID string) (*user.Profile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.errGetProfile != nil {
		return nil, m.errGetProfile
	}
	p, ok := m.profiles[userID]
	if !ok {
		return nil, nil
	}
	pCopy := *p
	return &pCopy, nil
}

func (m *mockUserRepository) GetProfileByID(ctx context.Context, id string) (*user.Profile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.errGetProfileByID != nil {
		return nil, m.errGetProfileByID
	}
	for _, p := range m.profiles {
		if p.ID.String() == id || p.UserID == id {
			pCopy := *p
			return &pCopy, nil
		}
	}
	return nil, nil
}

func (m *mockUserRepository) UpsertProfile(ctx context.Context, p *user.Profile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.errUpsertProfile != nil {
		return m.errUpsertProfile
	}
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	p.UpdatedAt = time.Now()
	pCopy := *p
	m.profiles[p.UserID] = &pCopy
	return nil
}

func (m *mockUserRepository) UpdateResume(ctx context.Context, userID string, key, filename string, size int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.errUpdateResume != nil {
		return m.errUpdateResume
	}
	p, ok := m.profiles[userID]
	if !ok {
		p = &user.Profile{
			ID:     uuid.New(),
			UserID: userID,
		}
	}
	p.ResumeKey = &key
	p.ResumeFilename = &filename
	p.ResumeByteSize = size
	p.UpdatedAt = time.Now()
	pCopy := *p
	m.profiles[userID] = &pCopy
	return nil
}

func (m *mockUserRepository) ProvisionUser(ctx context.Context, u *user.User, profile *user.Profile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.errProvisionUser != nil {
		return m.errProvisionUser
	}
	if m.errUpsertUser != nil {
		return m.errUpsertUser
	}
	if profile != nil && m.errUpsertProfile != nil {
		return m.errUpsertProfile
	}
	now := time.Now()
	if existing, ok := m.users[u.ID]; ok {
		u.CreatedAt = existing.CreatedAt
	} else if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	u.UpdatedAt = now
	userCopy := *u
	m.users[u.ID] = &userCopy

	if profile != nil {
		if profile.ID == uuid.Nil {
			profile.ID = uuid.New()
		}
		profile.UpdatedAt = now
		pCopy := *profile
		m.profiles[profile.UserID] = &pCopy
	}
	return nil
}

func (m *mockUserRepository) WithProfileLock(ctx context.Context, userID string, fn func(context.Context) error) error {
	m.mu.Lock()
	m.lockCalls++
	err := m.errWithProfileLock
	m.mu.Unlock()
	if err != nil {
		return err
	}
	return fn(ctx)
}

func (m *mockUserRepository) IsResumeReferenced(ctx context.Context, resumeKey string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.errIsResumeReferenced != nil {
		return false, m.errIsResumeReferenced
	}
	return m.referencedResumes[resumeKey], nil
}

func TestService_SyncUser_SurfacesProfileProvisionError(t *testing.T) {
	repo := newMockUserRepository()
	repo.errUpsertProfile = errors.New("profile write failed")
	svc := user.NewService(repo)
	claims := &auth.UserClaims{UserID: "u1", Email: "a@stanford.edu", EmailVerified: true, Roles: []string{"member"}}
	_, err := svc.SyncUser(context.Background(), claims, user.SyncUserRequest{})
	if err == nil {
		t.Fatal("expected profile provision error")
	}
}

func TestSyncUser_Success(t *testing.T) {
	repo := newMockUserRepository()
	svc := user.NewService(repo)

	claims := &auth.UserClaims{
		UserID:        uuid.New().String(),
		Email:         "student@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	u, err := svc.SyncUser(context.Background(), claims, user.SyncUserRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if u.Role != "member" {
		t.Errorf("expected role member, got %s", u.Role)
	}

	p, err := repo.GetProfile(context.Background(), u.ID)
	if err != nil || p == nil {
		t.Fatalf("expected profile auto-provisioned, err: %v", err)
	}
}

func TestSyncUser_AdminPrivilege(t *testing.T) {
	repo := newMockUserRepository()
	svc := user.NewService(repo)

	claims := &auth.UserClaims{
		UserID:        uuid.New().String(),
		Email:         "admin@lynk.test",
		EmailVerified: true,
		Roles:         []string{"admin"},
	}

	u, err := svc.SyncUser(context.Background(), claims, user.SyncUserRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if u.Role != "admin" {
		t.Errorf("expected role admin, got %s", u.Role)
	}
}

func TestSyncUser_AutoProvisionsProfileWithNames(t *testing.T) {
	repo := newMockUserRepository()
	svc := user.NewService(repo)

	claims := &auth.UserClaims{
		UserID:        uuid.New().String(),
		Email:         "taylor@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	req := user.SyncUserRequest{
		FirstName: "Taylor",
		LastName:  "Swift",
	}

	u, err := svc.SyncUser(context.Background(), claims, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Role != user.RoleMember {
		t.Errorf("expected role member, got %s", u.Role)
	}

	p, err := repo.GetProfile(context.Background(), u.ID)
	if err != nil || p == nil {
		t.Fatalf("expected profile to be auto-provisioned, err: %v", err)
	}
	if p.FirstName != "Taylor" || p.LastName != "Swift" {
		t.Errorf("expected profile name 'Taylor Swift', got '%s %s'", p.FirstName, p.LastName)
	}
}

func TestGetMe_Success(t *testing.T) {
	repo := newMockUserRepository()
	svc := user.NewService(repo)

	userID := "test-user-" + uuid.New().String()
	claims := &auth.UserClaims{
		UserID:        userID,
		Email:         "member@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	summary, err := svc.GetMe(context.Background(), claims)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary.User.Email != "member@stanford.edu" {
		t.Errorf("expected email member@stanford.edu, got %s", summary.User.Email)
	}
	if !summary.EmailVerified {
		t.Error("expected EmailVerified true")
	}
	if summary.Profile == nil {
		t.Error("expected Profile to be present")
	}
}

func TestService_UpdateProfile_OmitsFirstNamePreservesExisting(t *testing.T) {
	repo := newMockUserRepository()
	svc := user.NewService(repo)
	userID := "usr_1"
	_ = repo.UpsertProfile(context.Background(), &user.Profile{
		UserID: userID, FirstName: "Ada", LastName: "Lovelace", GraduationYear: 2026,
		Skills: []string{"Go"}, PortfolioLinks: []string{},
	})
	updated, err := svc.UpdateProfile(context.Background(), userID, user.UpdateProfileRequest{})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if updated.FirstName != "Ada" || updated.LastName != "Lovelace" {
		t.Fatalf("omit must preserve names, got %+v", updated)
	}
	if len(updated.Skills) != 1 || updated.Skills[0] != "Go" {
		t.Fatalf("omit must preserve skills, got %+v", updated.Skills)
	}
}

func TestService_UpdateProfile_OmitsGraduationYearPreservesExisting(t *testing.T) {
	repo := newMockUserRepository()
	svc := user.NewService(repo)

	userID := "st_member_1"
	_ = repo.UpsertUser(context.Background(), &user.User{ID: userID, Email: "a@stanford.edu", Role: user.RoleMember})
	_ = repo.UpsertProfile(context.Background(), &user.Profile{
		UserID:         userID,
		FirstName:      "Ada",
		LastName:       "Lovelace",
		GraduationYear: 2026,
		Skills:         []string{},
		PortfolioLinks: []string{},
	})

	updated, err := svc.UpdateProfile(context.Background(), userID, user.UpdateProfileRequest{
		FirstName:      ptr("Ada"),
		LastName:       ptr("Lovelace"),
		Bio:            ptr("Updated bio"),
		GraduationYear: nil,
		Skills:         ptr([]string{"go"}),
		PortfolioLinks: ptr([]string{}),
	})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if updated.GraduationYear != 2026 {
		t.Fatalf("expected graduation_year 2026 preserved, got %d", updated.GraduationYear)
	}
}

func TestService_UpdateProfile_ExplicitGraduationYearUpdates(t *testing.T) {
	repo := newMockUserRepository()
	svc := user.NewService(repo)
	userID := "st_member_2"
	_ = repo.UpsertUser(context.Background(), &user.User{ID: userID, Email: "b@stanford.edu", Role: user.RoleMember})
	_ = repo.UpsertProfile(context.Background(), &user.Profile{UserID: userID, GraduationYear: 2025, Skills: []string{}, PortfolioLinks: []string{}})

	year := 2028
	updated, err := svc.UpdateProfile(context.Background(), userID, user.UpdateProfileRequest{
		FirstName:      ptr("X"),
		LastName:       ptr("Y"),
		GraduationYear: &year,
		Skills:         ptr([]string{}),
		PortfolioLinks: ptr([]string{}),
	})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if updated.GraduationYear != 2028 {
		t.Fatalf("expected 2028, got %d", updated.GraduationYear)
	}
}

func TestUpdateProfile_Validation(t *testing.T) {
	repo := newMockUserRepository()
	svc := user.NewService(repo)
	userID := "test-user-" + uuid.New().String()

	// 1. Valid update
	year2025 := 2025
	req := user.UpdateProfileRequest{
		FirstName:           ptr("Alex"),
		LastName:            ptr("Rivera"),
		Bio:                 ptr("CS Senior at Stanford interested in distributed systems"),
		Department:          ptr("Computer Science"),
		GraduationYear:      &year2025,
		Skills:              ptr([]string{"Go", "React", "Docker"}),
		PortfolioLinks:      ptr([]string{"https://github.com/alexr"}),
		Organization:        ptr("Stanford ACM"),
		OrganizationWebsite: ptr("https://acm.stanford.edu"),
	}

	p, err := svc.UpdateProfile(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.FirstName != "Alex" || p.Organization != "Stanford ACM" {
		t.Errorf("unexpected profile values: %+v", p)
	}

	// 2. Invalid first name
	longName := strings.Repeat("A", 101)
	_, err = svc.UpdateProfile(context.Background(), userID, user.UpdateProfileRequest{FirstName: ptr(longName)})
	if !errors.Is(err, user.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}

	// 3. Invalid graduation year
	invalidYear := 1800
	_, err = svc.UpdateProfile(context.Background(), userID, user.UpdateProfileRequest{GraduationYear: &invalidYear})
	if !errors.Is(err, user.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}

	// 4. Organization website length exceeds 255
	longWebsite := "https://" + strings.Repeat("a", 250) + ".edu" // length > 255
	_, err = svc.UpdateProfile(context.Background(), userID, user.UpdateProfileRequest{OrganizationWebsite: ptr(longWebsite)})
	if !errors.Is(err, user.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for organization website > 255 chars, got %v", err)
	}
}

func TestUpdateProfile_RejectsInvalidURLScheme(t *testing.T) {
	repo := newMockUserRepository()
	svc := user.NewService(repo)
	userID := "test-user-" + uuid.New().String()

	// 1. Portfolio links with invalid schemes should be rejected
	invalidPortfolioLinks := []string{
		"javascript:alert(document.cookie)",
		"data:text/html,<script>alert(1)</script>",
		"ftp://files.stanford.edu/resume.pdf",
		"file:///etc/passwd",
		"not-a-valid-url",
	}

	for _, badLink := range invalidPortfolioLinks {
		t.Run("rejects portfolio link: "+badLink, func(t *testing.T) {
			_, err := svc.UpdateProfile(context.Background(), userID, user.UpdateProfileRequest{
				PortfolioLinks: ptr([]string{badLink}),
			})
			if !errors.Is(err, user.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput for link %q, got %v", badLink, err)
			}
		})
	}

	// 2. Organization website with invalid schemes should be rejected
	invalidOrgWebsites := []string{
		"javascript:alert(1)",
		"data:text/plain,hello",
		"ftp://org.stanford.edu",
		"invalid-website",
	}

	for _, badWebsite := range invalidOrgWebsites {
		t.Run("rejects organization website: "+badWebsite, func(t *testing.T) {
			_, err := svc.UpdateProfile(context.Background(), userID, user.UpdateProfileRequest{
				OrganizationWebsite: ptr(badWebsite),
			})
			if !errors.Is(err, user.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput for org website %q, got %v", badWebsite, err)
			}
		})
	}

	// 3. Valid HTTP and HTTPS URLs should be accepted
	validReq := user.UpdateProfileRequest{
		PortfolioLinks:      ptr([]string{"http://portfolio.stanford.edu", "https://github.com/student"}),
		OrganizationWebsite: ptr("https://acm.stanford.edu"),
	}
	p, err := svc.UpdateProfile(context.Background(), userID, validReq)
	if err != nil {
		t.Fatalf("expected valid URLs to be accepted, got: %v", err)
	}
	if len(p.PortfolioLinks) != 2 || p.OrganizationWebsite != "https://acm.stanford.edu" {
		t.Fatalf("unexpected profile after valid update: %+v", p)
	}

	// 4. Organization website with http should be accepted
	httpOrgReq := user.UpdateProfileRequest{
		OrganizationWebsite: ptr("http://lab.stanford.edu"),
	}
	p, err = svc.UpdateProfile(context.Background(), userID, httpOrgReq)
	if err != nil {
		t.Fatalf("expected http org website to be accepted, got: %v", err)
	}
	if p.OrganizationWebsite != "http://lab.stanford.edu" {
		t.Fatalf("unexpected organization website: %s", p.OrganizationWebsite)
	}

	// 5. Empty organization website should clear it without error
	emptyOrgReq := user.UpdateProfileRequest{
		OrganizationWebsite: ptr(""),
	}
	p, err = svc.UpdateProfile(context.Background(), userID, emptyOrgReq)
	if err != nil {
		t.Fatalf("expected empty org website to be accepted, got: %v", err)
	}
	if p.OrganizationWebsite != "" {
		t.Fatalf("expected cleared organization website, got: %s", p.OrganizationWebsite)
	}
}

func TestHandler_Routes(t *testing.T) {
	repo := newMockUserRepository()
	svc := user.NewService(repo)
	h := user.NewHandler(svc, repo)

	userID := "test-user-" + uuid.New().String()
	claims := &auth.UserClaims{
		UserID:        userID,
		Email:         "test@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	r := h.Routes(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithUserContext(req.Context(), claims)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	// 1. GET /auth/me
	req := httptest.NewRequest("GET", "/auth/me", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// 2. PUT /profile/me
	body, _ := json.Marshal(user.UpdateProfileRequest{
		FirstName: ptr("Sarah"),
		LastName:  ptr("Chen"),
	})
	req = httptest.NewRequest("PUT", "/profile/me", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// 3. GET /profile/me
	req = httptest.NewRequest("GET", "/profile/me", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRepository_GetProfileByID_SARGableQueryBranching(t *testing.T) {
	// 1. Valid UUID lookup branches to WHERE id = $1 OR user_id = $2
	validUUID := uuid.New()
	uuidStr := validUUID.String()

	queryWithUUID, argsWithUUID := user.BuildGetProfileByIDQuery(uuidStr)
	if strings.Contains(queryWithUUID, "id::text") {
		t.Errorf("expected SARGable query without id::text cast, got: %s", queryWithUUID)
	}
	if !strings.Contains(queryWithUUID, "WHERE id = $1 OR user_id = $2") {
		t.Errorf("expected query with 'WHERE id = $1 OR user_id = $2', got: %s", queryWithUUID)
	}
	if len(argsWithUUID) != 2 {
		t.Fatalf("expected 2 query arguments for UUID lookup, got %d", len(argsWithUUID))
	}
	if argsWithUUID[0] != validUUID {
		t.Errorf("expected first argument to be parsed uuid.UUID %v, got %v", validUUID, argsWithUUID[0])
	}
	if argsWithUUID[1] != uuidStr {
		t.Errorf("expected second argument to be id string %s, got %v", uuidStr, argsWithUUID[1])
	}

	// 2. Non-UUID lookup branches to WHERE user_id = $1
	nonUUID := "usr_supertokens_auth_string_12345"
	queryNonUUID, argsNonUUID := user.BuildGetProfileByIDQuery(nonUUID)
	if strings.Contains(queryNonUUID, "id::text") {
		t.Errorf("expected SARGable query without id::text cast, got: %s", queryNonUUID)
	}
	if !strings.Contains(queryNonUUID, "WHERE user_id = $1") {
		t.Errorf("expected query with 'WHERE user_id = $1', got: %s", queryNonUUID)
	}
	if len(argsNonUUID) != 1 {
		t.Fatalf("expected 1 query argument for non-UUID lookup, got %d", len(argsNonUUID))
	}
	if argsNonUUID[0] != nonUUID {
		t.Errorf("expected argument to be %s, got %v", nonUUID, argsNonUUID[0])
	}
}

func ptr[T any](v T) *T {
	return &v
}

func TestService_GetProfile_CacheAside(t *testing.T) {
	repo := newMockUserRepository()
	memCache := cache.NewMemoryCache()
	svc := user.NewService(repo).WithCache(memCache)

	profile := &user.Profile{
		UserID:    "user-cache-1",
		FirstName: "Alice",
		LastName:  "Smith",
		Bio:       "Original Bio",
	}
	if err := repo.UpsertProfile(context.Background(), profile); err != nil {
		t.Fatalf("failed to insert profile: %v", err)
	}

	// 1. Initial read populates cache
	p1, err := svc.GetProfile(context.Background(), "user-cache-1")
	if err != nil {
		t.Fatalf("failed to get profile: %v", err)
	}
	if p1.Bio != "Original Bio" {
		t.Fatalf("expected 'Original Bio', got %q", p1.Bio)
	}

	// 2. Direct repo modification bypasses service/cache
	repo.profiles["user-cache-1"].Bio = "Directly Modified Bio"

	// 3. Second read should return cached profile with original bio
	p2, err := svc.GetProfile(context.Background(), "user-cache-1")
	if err != nil {
		t.Fatalf("failed to get profile: %v", err)
	}
	if p2.Bio != "Original Bio" {
		t.Fatalf("expected cached 'Original Bio', got %q", p2.Bio)
	}

	// 4. UpdateProfile via service invalidates the cache
	newBio := "Updated via Service"
	_, err = svc.UpdateProfile(context.Background(), "user-cache-1", user.UpdateProfileRequest{
		Bio: &newBio,
	})
	if err != nil {
		t.Fatalf("failed to update profile: %v", err)
	}

	// 5. Subsequent read returns fresh profile
	p3, err := svc.GetProfile(context.Background(), "user-cache-1")
	if err != nil {
		t.Fatalf("failed to get profile: %v", err)
	}
	if p3.Bio != "Updated via Service" {
		t.Fatalf("expected 'Updated via Service', got %q", p3.Bio)
	}
}

func TestWithProfileLock_MockBehavior(t *testing.T) {
	repo := newMockUserRepository()
	ctx := context.Background()
	userID := "test-user-lock"

	// 1. Success case
	executed := false
	err := repo.WithProfileLock(ctx, userID, func(lockCtx context.Context) error {
		executed = true
		if lockCtx != ctx {
			t.Errorf("expected lockCtx to match passed ctx")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !executed {
		t.Fatalf("expected fn to be executed")
	}
	if repo.lockCalls != 1 {
		t.Fatalf("expected 1 lock call, got %d", repo.lockCalls)
	}

	// 2. Error propagation from fn
	expectedErr := errors.New("fn failed")
	err = repo.WithProfileLock(ctx, userID, func(lockCtx context.Context) error {
		return expectedErr
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
	if repo.lockCalls != 2 {
		t.Fatalf("expected 2 lock calls, got %d", repo.lockCalls)
	}

	// 3. Error in WithProfileLock itself
	lockErr := errors.New("lock acquisition failed")
	repo.errWithProfileLock = lockErr
	fnExecuted := false
	err = repo.WithProfileLock(ctx, userID, func(lockCtx context.Context) error {
		fnExecuted = true
		return nil
	})
	if !errors.Is(err, lockErr) {
		t.Fatalf("expected %v, got %v", lockErr, err)
	}
	if fnExecuted {
		t.Fatalf("expected fn not to execute when lock acquisition fails")
	}
}

func TestRepository_WithProfileLock_Structure(t *testing.T) {
	content, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("failed to read repository.go: %v", err)
	}
	src := string(content)

	fnStart := strings.Index(src, "func (r *Repository) WithProfileLock(")
	if fnStart == -1 {
		t.Fatalf("WithProfileLock function not found in repository.go")
	}
	fnBody := src[fnStart:]
	fnEnd := strings.Index(fnBody, "\nfunc ")
	if fnEnd != -1 {
		fnBody = fnBody[:fnEnd]
	}

	// 1. Verifies connection acquisition from pool
	if !strings.Contains(fnBody, "r.db.Acquire(ctx)") {
		t.Errorf("expected WithProfileLock to acquire dedicated conn with r.db.Acquire(ctx)")
	}

	// 2. Verifies connection release deferral
	if !strings.Contains(fnBody, "defer conn.Release()") {
		t.Errorf("expected WithProfileLock to defer conn.Release()")
	}

	// 3. Verifies session-level advisory lock
	if !strings.Contains(fnBody, "pg_advisory_lock(hashtext('profile_lock:' || $1))") {
		t.Errorf("expected WithProfileLock to acquire session-level advisory lock pg_advisory_lock")
	}

	// 4. Verifies deferred unlock
	if !strings.Contains(fnBody, "pg_advisory_unlock(hashtext('profile_lock:' || $1))") {
		t.Errorf("expected WithProfileLock to defer pg_advisory_unlock")
	}

	// 5. Verifies context.Background() used for cleanup to prevent lock leaks on cancelled ctx
	if !strings.Contains(fnBody, "conn.Exec(context.Background()") {
		t.Errorf("expected WithProfileLock to unlock using context.Background() to prevent lock leaks on cancelled context")
	}

	// 6. Verifies no transaction is opened (which caused connection starvation)
	if strings.Contains(fnBody, "r.db.Begin") || strings.Contains(fnBody, "pg_advisory_xact_lock") {
		t.Errorf("WithProfileLock must not use transaction-based advisory lock to prevent pool starvation")
	}
}

func TestRepository_WithProfileLock_Integration(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping user repository integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	repo := user.NewRepository(pool)
	userID := "test-user-lock-" + uuid.New().String()

	executed := false
	err = repo.WithProfileLock(ctx, userID, func(lockCtx context.Context) error {
		executed = true
		return nil
	})
	if err != nil {
		t.Fatalf("WithProfileLock failed: %v", err)
	}
	if !executed {
		t.Fatalf("expected fn to execute under lock")
	}
}

func TestUploadResume_RetainsReferencedResume(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()
	svc := user.NewService(repo, s3Client)

	userUUID := uuid.New().String()
	oldKey := "resumes/" + userUUID + "/old_cv.pdf"
	s3Client.uploads[oldKey] = []byte("%PDF-1.4 old resume")
	if err := repo.UpdateResume(context.Background(), userUUID, oldKey, "old_cv.pdf", 500); err != nil {
		t.Fatalf("failed to seed initial resume: %v", err)
	}

	// Mark the old resume as referenced by a submitted application
	repo.referencedResumes[oldKey] = true

	claims := &auth.UserClaims{
		UserID:        userUUID,
		Email:         "student@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	newPDF := bytes.NewReader([]byte("%PDF-1.7 new resume content"))
	updated, err := svc.UploadResume(context.Background(), claims, "new_cv.pdf", int64(newPDF.Len()), "application/pdf", newPDF)
	if err != nil {
		t.Fatalf("unexpected upload error: %v", err)
	}

	if updated.ResumeKey == nil || *updated.ResumeKey == oldKey {
		t.Fatalf("expected resume key to be updated")
	}

	// Verify oldKey was NOT deleted from storage because it is referenced in an application
	for _, k := range s3Client.deletedKeys {
		if k == oldKey {
			t.Fatalf("referenced resume key %q should NOT have been deleted from storage", oldKey)
		}
	}
	if _, ok := s3Client.uploads[oldKey]; !ok {
		t.Fatalf("referenced resume key %q must remain present in storage", oldKey)
	}
}

func TestConfirmDirectResumeUpload_RetainsReferencedResume(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()
	svc := user.NewService(repo, s3Client)

	userUUID := uuid.New().String()
	oldKey := "resumes/" + userUUID + "/old_cv.pdf"
	s3Client.uploads[oldKey] = []byte("%PDF-1.4 old resume")
	if err := repo.UpdateResume(context.Background(), userUUID, oldKey, "old_cv.pdf", 500); err != nil {
		t.Fatalf("failed to seed initial resume: %v", err)
	}

	// Mark the old resume as referenced by a submitted application
	repo.referencedResumes[oldKey] = true

	claims := &auth.UserClaims{
		UserID:        userUUID,
		Email:         "student@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	newKey := "resumes/" + userUUID + "/new_cv.pdf"
	updated, err := svc.ConfirmDirectResumeUpload(context.Background(), claims, newKey)
	if err != nil {
		t.Fatalf("unexpected confirm error: %v", err)
	}

	if updated.ResumeKey == nil || *updated.ResumeKey != newKey {
		t.Fatalf("expected resume key to be updated to %s", newKey)
	}

	// Verify oldKey was NOT deleted from storage because it is referenced
	for _, k := range s3Client.deletedKeys {
		if k == oldKey {
			t.Fatalf("referenced resume key %q should NOT have been deleted from storage", oldKey)
		}
	}
	if _, ok := s3Client.uploads[oldKey]; !ok {
		t.Fatalf("referenced resume key %q must remain present in storage", oldKey)
	}
}

func TestUploadResume_DeletesUnreferencedResume(t *testing.T) {
	repo := newMockUserRepository()
	s3Client := newMockStorageClient()
	svc := user.NewService(repo, s3Client)

	userUUID := uuid.New().String()
	oldKey := "resumes/" + userUUID + "/old_cv.pdf"
	s3Client.uploads[oldKey] = []byte("%PDF-1.4 old resume")
	if err := repo.UpdateResume(context.Background(), userUUID, oldKey, "old_cv.pdf", 500); err != nil {
		t.Fatalf("failed to seed initial resume: %v", err)
	}

	// Explicitly unreferenced
	repo.referencedResumes[oldKey] = false

	claims := &auth.UserClaims{
		UserID:        userUUID,
		Email:         "student@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	newPDF := bytes.NewReader([]byte("%PDF-1.7 new resume content"))
	_, err := svc.UploadResume(context.Background(), claims, "new_cv.pdf", int64(newPDF.Len()), "application/pdf", newPDF)
	if err != nil {
		t.Fatalf("unexpected upload error: %v", err)
	}

	// Verify oldKey WAS deleted
	found := false
	for _, k := range s3Client.deletedKeys {
		if k == oldKey {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("unreferenced resume key %q should have been deleted from storage", oldKey)
	}
}

func TestRepository_IsResumeReferenced_EmptyKey(t *testing.T) {
	repo := &user.Repository{}
	referenced, err := repo.IsResumeReferenced(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error for empty key: %v", err)
	}
	if referenced {
		t.Fatalf("expected empty key not to be referenced")
	}
}
