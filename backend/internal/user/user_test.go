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

	errUpsertUser     error
	errGetUser        error
	errGetProfile     error
	errGetProfileByID error
	errUpsertProfile  error
	errUpdateResume   error
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

func TestUpdateProfile_Validation(t *testing.T) {
	repo := newMockUserRepository()
	svc := user.NewService(repo)
	userID := "test-user-" + uuid.New().String()

	// 1. Valid update
	req := user.UpdateProfileRequest{
		FirstName:           "Alex",
		LastName:            "Rivera",
		Bio:                 "CS Senior at Stanford interested in distributed systems",
		Department:          "Computer Science",
		GraduationYear:      2025,
		Skills:              []string{"Go", "React", "Docker"},
		PortfolioLinks:      []string{"https://github.com/alexr"},
		Organization:        "Stanford ACM",
		OrganizationWebsite: "https://acm.stanford.edu",
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
	_, err = svc.UpdateProfile(context.Background(), userID, user.UpdateProfileRequest{FirstName: longName})
	if !errors.Is(err, user.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}

	// 3. Invalid graduation year
	_, err = svc.UpdateProfile(context.Background(), userID, user.UpdateProfileRequest{GraduationYear: 1800})
	if !errors.Is(err, user.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
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
		FirstName: "Sarah",
		LastName:  "Chen",
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
