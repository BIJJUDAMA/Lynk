package user_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
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
}

func newMockUserRepository() *mockUserRepository {
	return &mockUserRepository{
		users:    make(map[string]*user.User),
		profiles: make(map[string]*user.Profile),
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

