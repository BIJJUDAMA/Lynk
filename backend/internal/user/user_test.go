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
	mu               sync.RWMutex
	users            map[uuid.UUID]*user.User
	studentProfiles  map[uuid.UUID]*user.StudentProfile // keyed by user_id
	employerProfiles map[uuid.UUID]*user.EmployerProfile // keyed by user_id

	errUpsertUser            error
	errGetUser               error
	errGetStudentProfile     error
	errGetStudentProfileByID error
	errUpsertStudentProfile  error
	errUpdateStudentResume   error
	errGetEmployerProfile    error
	errUpsertEmployerProfile error
}

func newMockUserRepository() *mockUserRepository {
	return &mockUserRepository{
		users:            make(map[uuid.UUID]*user.User),
		studentProfiles:  make(map[uuid.UUID]*user.StudentProfile),
		employerProfiles: make(map[uuid.UUID]*user.EmployerProfile),
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

func (m *mockUserRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
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

func (m *mockUserRepository) GetStudentProfile(ctx context.Context, userID uuid.UUID) (*user.StudentProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.errGetStudentProfile != nil {
		return nil, m.errGetStudentProfile
	}
	sp, ok := m.studentProfiles[userID]
	if !ok {
		return nil, nil
	}
	spCopy := *sp
	return &spCopy, nil
}

func (m *mockUserRepository) GetStudentProfileByID(ctx context.Context, id uuid.UUID) (*user.StudentProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.errGetStudentProfileByID != nil {
		return nil, m.errGetStudentProfileByID
	}
	for _, sp := range m.studentProfiles {
		if sp.ID == id || sp.UserID == id {
			spCopy := *sp
			return &spCopy, nil
		}
	}
	return nil, nil
}

func (m *mockUserRepository) UpsertStudentProfile(ctx context.Context, sp *user.StudentProfile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.errUpsertStudentProfile != nil {
		return m.errUpsertStudentProfile
	}
	if sp.ID == uuid.Nil {
		sp.ID = uuid.New()
	}
	sp.UpdatedAt = time.Now()
	if sp.Skills == nil {
		sp.Skills = []string{}
	}
	if sp.PortfolioLinks == nil {
		sp.PortfolioLinks = []string{}
	}
	spCopy := *sp
	m.studentProfiles[sp.UserID] = &spCopy
	return nil
}

func (m *mockUserRepository) UpdateStudentResume(ctx context.Context, userID uuid.UUID, key, filename string, size int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.errUpdateStudentResume != nil {
		return m.errUpdateStudentResume
	}
	sp, ok := m.studentProfiles[userID]
	if !ok {
		sp = &user.StudentProfile{
			ID:             uuid.New(),
			UserID:         userID,
			Skills:         []string{},
			PortfolioLinks: []string{},
		}
	}
	sp.ResumeKey = &key
	sp.ResumeFilename = &filename
	sp.ResumeByteSize = size
	sp.UpdatedAt = time.Now()
	m.studentProfiles[userID] = sp
	return nil
}

func (m *mockUserRepository) GetEmployerProfile(ctx context.Context, userID uuid.UUID) (*user.EmployerProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.errGetEmployerProfile != nil {
		return nil, m.errGetEmployerProfile
	}
	ep, ok := m.employerProfiles[userID]
	if !ok {
		return nil, nil
	}
	epCopy := *ep
	return &epCopy, nil
}

func (m *mockUserRepository) UpsertEmployerProfile(ctx context.Context, ep *user.EmployerProfile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.errUpsertEmployerProfile != nil {
		return m.errUpsertEmployerProfile
	}
	if ep.ID == uuid.Nil {
		ep.ID = uuid.New()
	}
	ep.UpdatedAt = time.Now()
	epCopy := *ep
	m.employerProfiles[ep.UserID] = &epCopy
	return nil
}

// ---------------------------------------------------------------------------
// 1. Model Serialization Tests
// ---------------------------------------------------------------------------

func TestModelSerialization(t *testing.T) {
	userID := uuid.New()
	profileID := uuid.New()
	resumeKey := "resumes/key.pdf"
	resumeFile := "resume.pdf"

	sp := user.StudentProfile{
		ID:             profileID,
		UserID:         userID,
		FirstName:      "Jane",
		LastName:       "Doe",
		Bio:            "CS Undergrad",
		Department:     "Computer Science",
		GraduationYear: 2026,
		Skills:         []string{"Go", "React"},
		PortfolioLinks: []string{"https://github.com/janedoe"},
		ResumeKey:      &resumeKey,
		ResumeFilename: &resumeFile,
		ResumeByteSize: 1024,
		UpdatedAt:      time.Now(),
	}

	data, err := json.Marshal(sp)
	if err != nil {
		t.Fatalf("failed to marshal StudentProfile: %v", err)
	}

	var parsed user.StudentProfile
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal StudentProfile: %v", err)
	}

	if parsed.FirstName != "Jane" || parsed.LastName != "Doe" || parsed.GraduationYear != 2026 {
		t.Errorf("unmarshaled values do not match: %+v", parsed)
	}
	if len(parsed.Skills) != 2 || parsed.Skills[0] != "Go" {
		t.Errorf("skills unmarshaled incorrectly: %v", parsed.Skills)
	}

	ep := user.EmployerProfile{
		ID:           profileID,
		UserID:       userID,
		CompanyOrOrg: "Acme Corp",
		ContactName:  "John Smith",
		Description:  "Tech startup",
		Website:      "https://acme.org",
		UpdatedAt:    time.Now(),
	}

	epData, err := json.Marshal(ep)
	if err != nil {
		t.Fatalf("failed to marshal EmployerProfile: %v", err)
	}

	var parsedEP user.EmployerProfile
	if err := json.Unmarshal(epData, &parsedEP); err != nil {
		t.Fatalf("failed to unmarshal EmployerProfile: %v", err)
	}

	if parsedEP.CompanyOrOrg != "Acme Corp" || parsedEP.ContactName != "John Smith" {
		t.Errorf("employer profile mismatch: %+v", parsedEP)
	}
}

// ---------------------------------------------------------------------------
// 2. Service Unit Tests
// ---------------------------------------------------------------------------

func TestService_SyncUser(t *testing.T) {
	repo := newMockUserRepository()
	svc := user.NewService(repo)
	ctx := context.Background()

	t.Run("Sync student from claims", func(t *testing.T) {
		userUUID := uuid.New()
		claims := &auth.UserClaims{
			UserID:        userUUID.String(),
			Email:         "student@campus.edu",
			EmailVerified: true,
			Roles:         []string{"student"},
		}

		u, err := svc.SyncUser(ctx, claims, user.SyncUserRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if u.ID != userUUID || u.Role != "student" || u.Email != "student@campus.edu" {
			t.Errorf("unexpected user values: %+v", u)
		}

		// Ensure profile was auto-provisioned
		sp, err := repo.GetStudentProfile(ctx, userUUID)
		if err != nil || sp == nil {
			t.Errorf("expected auto-provisioned student profile, got: %v", sp)
		}
	})

	t.Run("Sync employer with explicit role request when claims has no roles", func(t *testing.T) {
		userUUID := uuid.New()
		claims := &auth.UserClaims{
			UserID:        userUUID.String(),
			Email:         "employer@company.com",
			EmailVerified: true,
			Roles:         []string{},
		}

		u, err := svc.SyncUser(ctx, claims, user.SyncUserRequest{Role: "employer"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if u.Role != "employer" {
			t.Errorf("expected employer role, got %s", u.Role)
		}

		// Ensure employer profile was auto-provisioned
		ep, err := repo.GetEmployerProfile(ctx, userUUID)
		if err != nil || ep == nil {
			t.Errorf("expected auto-provisioned employer profile, got: %v", ep)
		}
	})

	t.Run("Claims.Roles is source of truth when caller passes conflicting role", func(t *testing.T) {
		userUUID := uuid.New()
		studentClaims := &auth.UserClaims{
			UserID:        userUUID.String(),
			Email:         "verified_student@univ.edu",
			EmailVerified: true,
			Roles:         []string{"student"},
		}

		// Student caller attempts to request employer role
		u, err := svc.SyncUser(ctx, studentClaims, user.SyncUserRequest{Role: "employer"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if u.Role != "student" {
			t.Errorf("expected claims.Roles source of truth 'student', got %s", u.Role)
		}
	})

	t.Run("Privilege escalation: Non-admin requesting admin role returns ErrInvalidRole", func(t *testing.T) {
		userUUID := uuid.New()
		studentClaims := &auth.UserClaims{
			UserID:        userUUID.String(),
			Email:         "student@univ.edu",
			EmailVerified: true,
			Roles:         []string{"student"},
		}

		_, err := svc.SyncUser(ctx, studentClaims, user.SyncUserRequest{Role: "admin"})
		if !errors.Is(err, user.ErrInvalidRole) {
			t.Errorf("expected ErrInvalidRole when non-admin requests admin, got: %v", err)
		}

		// Also test when user has no roles in claims
		noRoleClaims := &auth.UserClaims{
			UserID:        uuid.New().String(),
			Email:         "user@univ.edu",
			EmailVerified: true,
			Roles:         []string{},
		}
		_, err = svc.SyncUser(ctx, noRoleClaims, user.SyncUserRequest{Role: "admin"})
		if !errors.Is(err, user.ErrInvalidRole) {
			t.Errorf("expected ErrInvalidRole when unprivileged user requests admin, got: %v", err)
		}
	})

	t.Run("Privilege escalation: Admin claim user can assign admin role", func(t *testing.T) {
		userUUID := uuid.New()
		adminClaims := &auth.UserClaims{
			UserID:        userUUID.String(),
			Email:         "admin@lynk.internal",
			EmailVerified: true,
			Roles:         []string{"admin"},
		}

		u, err := svc.SyncUser(ctx, adminClaims, user.SyncUserRequest{Role: "admin"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if u.Role != "admin" {
			t.Errorf("expected admin role, got %s", u.Role)
		}
	})

	t.Run("Invalid role returns ErrInvalidRole", func(t *testing.T) {
		claims := &auth.UserClaims{
			UserID: uuid.New().String(),
			Email:  "test@univ.edu",
		}
		_, err := svc.SyncUser(ctx, claims, user.SyncUserRequest{Role: "superadmin"})
		if !errors.Is(err, user.ErrInvalidRole) {
			t.Errorf("expected ErrInvalidRole, got: %v", err)
		}
	})

	t.Run("Missing credentials returns ErrUnauthorized", func(t *testing.T) {
		_, err := svc.SyncUser(ctx, nil, user.SyncUserRequest{})
		if !errors.Is(err, user.ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got: %v", err)
		}
	})

	t.Run("Invalid UUID returns ErrInvalidInput", func(t *testing.T) {
		claims := &auth.UserClaims{UserID: "not-a-uuid"}
		_, err := svc.SyncUser(ctx, claims, user.SyncUserRequest{})
		if !errors.Is(err, user.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput, got: %v", err)
		}
	})
}

func TestService_GetMe(t *testing.T) {
	repo := newMockUserRepository()
	svc := user.NewService(repo)
	ctx := context.Background()

	userUUID := uuid.New()
	claims := &auth.UserClaims{
		UserID:        userUUID.String(),
		Email:         "student@harvard.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}

	t.Run("Auto-syncs user if not yet in database", func(t *testing.T) {
		summary, err := svc.GetMe(ctx, claims)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if summary.User == nil || summary.User.ID != userUUID {
			t.Errorf("unexpected user in summary: %+v", summary.User)
		}
		if !summary.EmailVerified {
			t.Errorf("expected email_verified to be true")
		}
		if summary.StudentProfile == nil {
			t.Errorf("expected student profile in summary")
		}
	})

	t.Run("Unauthorized on nil claims", func(t *testing.T) {
		_, err := svc.GetMe(ctx, nil)
		if !errors.Is(err, user.ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got: %v", err)
		}
	})
}

func TestService_StudentProfileOperations(t *testing.T) {
	repo := newMockUserRepository()
	svc := user.NewService(repo)
	ctx := context.Background()
	userUUID := uuid.New()

	t.Run("Get non-existent profile returns ErrProfileNotFound", func(t *testing.T) {
		_, err := svc.GetStudentProfile(ctx, userUUID)
		if !errors.Is(err, user.ErrProfileNotFound) {
			t.Errorf("expected ErrProfileNotFound, got %v", err)
		}
	})

	t.Run("Update profile validates graduation year", func(t *testing.T) {
		_, err := svc.UpdateStudentProfile(ctx, userUUID, user.UpdateStudentProfileRequest{
			GraduationYear: 1800,
		})
		if !errors.Is(err, user.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput for year 1800, got %v", err)
		}
	})

	t.Run("Update profile validates string lengths", func(t *testing.T) {
		longStr101 := strings.Repeat("a", 101)
		longBio5001 := strings.Repeat("b", 5001)

		// First name > 100
		_, err := svc.UpdateStudentProfile(ctx, userUUID, user.UpdateStudentProfileRequest{FirstName: longStr101})
		if !errors.Is(err, user.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput for first_name > 100, got %v", err)
		}

		// Last name > 100
		_, err = svc.UpdateStudentProfile(ctx, userUUID, user.UpdateStudentProfileRequest{LastName: longStr101})
		if !errors.Is(err, user.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput for last_name > 100, got %v", err)
		}

		// Department > 100
		_, err = svc.UpdateStudentProfile(ctx, userUUID, user.UpdateStudentProfileRequest{Department: longStr101})
		if !errors.Is(err, user.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput for department > 100, got %v", err)
		}

		// Bio > 5000
		_, err = svc.UpdateStudentProfile(ctx, userUUID, user.UpdateStudentProfileRequest{Bio: longBio5001})
		if !errors.Is(err, user.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput for bio > 5000, got %v", err)
		}

		// Skill tag > 100
		_, err = svc.UpdateStudentProfile(ctx, userUUID, user.UpdateStudentProfileRequest{Skills: []string{longStr101}})
		if !errors.Is(err, user.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput for skill > 100, got %v", err)
		}

		// Portfolio link > 2048
		longLink2049 := strings.Repeat("c", 2049)
		_, err = svc.UpdateStudentProfile(ctx, userUUID, user.UpdateStudentProfileRequest{PortfolioLinks: []string{longLink2049}})
		if !errors.Is(err, user.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput for link > 2048, got %v", err)
		}
	})

	t.Run("Successful update and fetch", func(t *testing.T) {
		req := user.UpdateStudentProfileRequest{
			FirstName:      "Alice",
			LastName:       "Smith",
			Bio:            "Aspiring software engineer",
			Department:     "ECE",
			GraduationYear: 2025,
			Skills:         []string{"Go", "Python"},
			PortfolioLinks: []string{"https://alice.dev"},
		}

		updated, err := svc.UpdateStudentProfile(ctx, userUUID, req)
		if err != nil {
			t.Fatalf("failed to update student profile: %v", err)
		}
		if updated.FirstName != "Alice" || updated.GraduationYear != 2025 {
			t.Errorf("unexpected profile data: %+v", updated)
		}

		fetched, err := svc.GetStudentProfile(ctx, userUUID)
		if err != nil {
			t.Fatalf("failed to get student profile: %v", err)
		}
		if fetched.FirstName != "Alice" || len(fetched.Skills) != 2 {
			t.Errorf("fetched profile mismatch: %+v", fetched)
		}

		// Test GetStudentProfileByID
		byID, err := svc.GetStudentProfileByID(ctx, fetched.ID)
		if err != nil {
			t.Fatalf("failed to get student profile by ID: %v", err)
		}
		if byID.UserID != userUUID {
			t.Errorf("expected UserID %v, got %v", userUUID, byID.UserID)
		}
	})
}

func TestService_EmployerProfileOperations(t *testing.T) {
	repo := newMockUserRepository()
	svc := user.NewService(repo)
	ctx := context.Background()
	userUUID := uuid.New()

	t.Run("Get non-existent employer profile returns ErrProfileNotFound", func(t *testing.T) {
		_, err := svc.GetEmployerProfile(ctx, userUUID)
		if !errors.Is(err, user.ErrProfileNotFound) {
			t.Errorf("expected ErrProfileNotFound, got %v", err)
		}
	})

	t.Run("Update employer profile validates string lengths", func(t *testing.T) {
		longStr201 := strings.Repeat("x", 201)
		longStr101 := strings.Repeat("y", 101)
		longStr256 := strings.Repeat("z", 256)
		longDesc5001 := strings.Repeat("w", 5001)

		// CompanyOrOrg > 200
		_, err := svc.UpdateEmployerProfile(ctx, userUUID, user.UpdateEmployerProfileRequest{CompanyOrOrg: longStr201})
		if !errors.Is(err, user.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput for company_or_org > 200, got %v", err)
		}

		// ContactName > 100
		_, err = svc.UpdateEmployerProfile(ctx, userUUID, user.UpdateEmployerProfileRequest{ContactName: longStr101})
		if !errors.Is(err, user.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput for contact_name > 100, got %v", err)
		}

		// Website > 255
		_, err = svc.UpdateEmployerProfile(ctx, userUUID, user.UpdateEmployerProfileRequest{Website: longStr256})
		if !errors.Is(err, user.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput for website > 255, got %v", err)
		}

		// Description > 5000
		_, err = svc.UpdateEmployerProfile(ctx, userUUID, user.UpdateEmployerProfileRequest{Description: longDesc5001})
		if !errors.Is(err, user.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput for description > 5000, got %v", err)
		}
	})

	t.Run("Successful update and fetch", func(t *testing.T) {
		req := user.UpdateEmployerProfileRequest{
			CompanyOrOrg: "TechLabs",
			ContactName:  "Bob Builder",
			Description:  "Autonomous robotics lab",
			Website:      "https://techlabs.io",
		}

		updated, err := svc.UpdateEmployerProfile(ctx, userUUID, req)
		if err != nil {
			t.Fatalf("failed to update employer profile: %v", err)
		}
		if updated.CompanyOrOrg != "TechLabs" || updated.ContactName != "Bob Builder" {
			t.Errorf("unexpected updated employer data: %+v", updated)
		}

		fetched, err := svc.GetEmployerProfile(ctx, userUUID)
		if err != nil {
			t.Fatalf("failed to get employer profile: %v", err)
		}
		if fetched.Website != "https://techlabs.io" {
			t.Errorf("unexpected fetched website: %s", fetched.Website)
		}
	})
}

// ---------------------------------------------------------------------------
// 3. HTTP Handler Tests
// ---------------------------------------------------------------------------

func setupRouter(repo user.UserRepository) (http.Handler, *user.Handler) {
	svc := user.NewService(repo)
	h := user.NewHandler(svc, repo)
	// Pass a pass-through middleware for testing route registration
	router := h.Routes(func(next http.Handler) http.Handler {
		return next
	})
	return router, h
}

type responseEnvelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func TestHandler_SyncUser(t *testing.T) {
	repo := newMockUserRepository()
	router, _ := setupRouter(repo)

	userUUID := uuid.New()
	claims := &auth.UserClaims{
		UserID:        userUUID.String(),
		Email:         "synctest@campus.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}

	t.Run("Success 200 OK", func(t *testing.T) {
		body, _ := json.Marshal(user.SyncUserRequest{Role: "student"})
		req := httptest.NewRequest("POST", "/auth/sync", bytes.NewReader(body))
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
		}

		var env responseEnvelope
		if err := json.NewDecoder(rr.Body).Decode(&env); err != nil {
			t.Fatalf("failed to decode response envelope: %v", err)
		}
		if !env.Success || env.Error != nil {
			t.Errorf("expected success envelope, got: %+v", env)
		}

		var u user.User
		if err := json.Unmarshal(env.Data, &u); err != nil {
			t.Fatalf("failed to unmarshal user data: %v", err)
		}
		if u.ID != userUUID || u.Role != "student" {
			t.Errorf("unexpected user data: %+v", u)
		}
	})

	t.Run("Missing credentials returns 401 UNAUTHORIZED", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/auth/sync", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
		var env responseEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&env)
		if env.Success || env.Error == nil || env.Error.Code != "UNAUTHORIZED" {
			t.Errorf("expected UNAUTHORIZED error code, got: %+v", env.Error)
		}
	})

	t.Run("Invalid role returns 400 INVALID_ROLE", func(t *testing.T) {
		body, _ := json.Marshal(user.SyncUserRequest{Role: "invalid_role"})
		req := httptest.NewRequest("POST", "/auth/sync", bytes.NewReader(body))
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
		var env responseEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&env)
		if env.Error == nil || env.Error.Code != "INVALID_ROLE" {
			t.Errorf("expected INVALID_ROLE error, got: %+v", env.Error)
		}
	})

	t.Run("Admin role request by non-admin returns 400 INVALID_ROLE", func(t *testing.T) {
		body, _ := json.Marshal(user.SyncUserRequest{Role: "admin"})
		req := httptest.NewRequest("POST", "/auth/sync", bytes.NewReader(body))
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
		var env responseEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&env)
		if env.Error == nil || env.Error.Code != "INVALID_ROLE" {
			t.Errorf("expected INVALID_ROLE error, got: %+v", env.Error)
		}
	})
}

func TestHandler_GetMe(t *testing.T) {
	repo := newMockUserRepository()
	router, _ := setupRouter(repo)

	userUUID := uuid.New()
	claims := &auth.UserClaims{
		UserID:        userUUID.String(),
		Email:         "me@mit.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}

	t.Run("Success 200 OK", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/auth/me", nil)
		req = req.WithContext(auth.WithUserContext(req.Context(), claims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d. Body: %s", rr.Code, rr.Body.String())
		}

		var env responseEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&env)
		if !env.Success {
			t.Errorf("expected success true")
		}

		var summary user.UserProfileSummary
		if err := json.Unmarshal(env.Data, &summary); err != nil {
			t.Fatalf("failed to unmarshal UserProfileSummary: %v", err)
		}
		if summary.User.ID != userUUID || !summary.EmailVerified {
			t.Errorf("unexpected summary data: %+v", summary)
		}
	})

	t.Run("Unauthorized 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/auth/me", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})
}

func TestHandler_StudentProfileEndpoints(t *testing.T) {
	repo := newMockUserRepository()
	router, _ := setupRouter(repo)

	studentUUID := uuid.New()
	studentClaims := &auth.UserClaims{
		UserID:        studentUUID.String(),
		Email:         "student@stanford.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}

	employerUUID := uuid.New()
	employerClaims := &auth.UserClaims{
		UserID:        employerUUID.String(),
		Email:         "employer@corp.com",
		EmailVerified: true,
		Roles:         []string{"employer"},
	}

	t.Run("GET /profile/student by Employer returns 403 FORBIDDEN (Affirmative Authorization)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/profile/student", nil)
		req = req.WithContext(auth.WithUserContext(req.Context(), employerClaims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d. Body: %s", rr.Code, rr.Body.String())
		}
		var env responseEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&env)
		if env.Error == nil || env.Error.Code != "FORBIDDEN" {
			t.Errorf("expected FORBIDDEN code, got: %+v", env.Error)
		}
	})

	t.Run("GET /profile/student by Student before creation returns 404 PROFILE_NOT_FOUND", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/profile/student", nil)
		req = req.WithContext(auth.WithUserContext(req.Context(), studentClaims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d. Body: %s", rr.Code, rr.Body.String())
		}
		var env responseEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&env)
		if env.Error == nil || env.Error.Code != "PROFILE_NOT_FOUND" {
			t.Errorf("expected PROFILE_NOT_FOUND, got: %+v", env.Error)
		}
	})

	t.Run("PUT /profile/student by Employer returns 403 FORBIDDEN (Affirmative Authorization)", func(t *testing.T) {
		reqBody, _ := json.Marshal(user.UpdateStudentProfileRequest{FirstName: "Hacker"})
		req := httptest.NewRequest("PUT", "/profile/student", bytes.NewReader(reqBody))
		req = req.WithContext(auth.WithUserContext(req.Context(), employerClaims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
		var env responseEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&env)
		if env.Error == nil || env.Error.Code != "FORBIDDEN" {
			t.Errorf("expected FORBIDDEN code, got: %+v", env.Error)
		}
	})

	t.Run("PUT /profile/student validates length and returns 400 INVALID_INPUT", func(t *testing.T) {
		reqBody, _ := json.Marshal(user.UpdateStudentProfileRequest{
			FirstName: strings.Repeat("A", 101),
		})
		req := httptest.NewRequest("PUT", "/profile/student", bytes.NewReader(reqBody))
		req = req.WithContext(auth.WithUserContext(req.Context(), studentClaims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
		var env responseEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&env)
		if env.Error == nil || env.Error.Code != "INVALID_INPUT" {
			t.Errorf("expected INVALID_INPUT, got: %+v", env.Error)
		}
	})

	t.Run("PUT /profile/student by Student creates/updates profile (200 OK)", func(t *testing.T) {
		reqBody, _ := json.Marshal(user.UpdateStudentProfileRequest{
			FirstName:      "Grace",
			LastName:       "Hopper",
			Bio:            "Computer scientist",
			Department:     "Computer Science",
			GraduationYear: 2027,
			Skills:         []string{"Compilers", "Go"},
			PortfolioLinks: []string{"https://grace.io"},
		})
		req := httptest.NewRequest("PUT", "/profile/student", bytes.NewReader(reqBody))
		req = req.WithContext(auth.WithUserContext(req.Context(), studentClaims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d. Body: %s", rr.Code, rr.Body.String())
		}

		var env responseEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&env)
		var sp user.StudentProfile
		_ = json.Unmarshal(env.Data, &sp)
		if sp.FirstName != "Grace" || sp.GraduationYear != 2027 {
			t.Errorf("unexpected profile data: %+v", sp)
		}
	})

	t.Run("GET /profile/student after update returns profile (200 OK)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/profile/student", nil)
		req = req.WithContext(auth.WithUserContext(req.Context(), studentClaims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		var env responseEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&env)
		var sp user.StudentProfile
		_ = json.Unmarshal(env.Data, &sp)
		if sp.FirstName != "Grace" {
			t.Errorf("expected FirstName 'Grace', got '%s'", sp.FirstName)
		}
	})

	t.Run("GET /profile/student/{id} by Employer views student profile", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/profile/student/"+studentUUID.String(), nil)
		req = req.WithContext(auth.WithUserContext(req.Context(), employerClaims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d. Body: %s", rr.Code, rr.Body.String())
		}
		var env responseEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&env)
		var sp user.StudentProfile
		_ = json.Unmarshal(env.Data, &sp)
		if sp.UserID != studentUUID || sp.FirstName != "Grace" {
			t.Errorf("unexpected student profile view: %+v", sp)
		}
	})

	t.Run("GET /profile/student/{id} with invalid ID returns 400", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/profile/student/invalid-uuid", nil)
		req = req.WithContext(auth.WithUserContext(req.Context(), employerClaims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("GET /profile/student/{id} with non-existent ID returns 404", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/profile/student/"+uuid.New().String(), nil)
		req = req.WithContext(auth.WithUserContext(req.Context(), employerClaims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rr.Code)
		}
	})
}

func TestHandler_EmployerProfileEndpoints(t *testing.T) {
	repo := newMockUserRepository()
	router, _ := setupRouter(repo)

	employerUUID := uuid.New()
	employerClaims := &auth.UserClaims{
		UserID:        employerUUID.String(),
		Email:         "founder@startup.io",
		EmailVerified: true,
		Roles:         []string{"employer"},
	}

	studentUUID := uuid.New()
	studentClaims := &auth.UserClaims{
		UserID:        studentUUID.String(),
		Email:         "student@cal.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}

	t.Run("GET /profile/employer by Student returns 403 FORBIDDEN (Affirmative Authorization)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/profile/employer", nil)
		req = req.WithContext(auth.WithUserContext(req.Context(), studentClaims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d. Body: %s", rr.Code, rr.Body.String())
		}
		var env responseEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&env)
		if env.Error == nil || env.Error.Code != "FORBIDDEN" {
			t.Errorf("expected FORBIDDEN code, got: %+v", env.Error)
		}
	})

	t.Run("GET /profile/employer by Employer before creation returns 404", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/profile/employer", nil)
		req = req.WithContext(auth.WithUserContext(req.Context(), employerClaims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rr.Code)
		}
	})

	t.Run("PUT /profile/employer by Student returns 403 FORBIDDEN (Affirmative Authorization)", func(t *testing.T) {
		reqBody, _ := json.Marshal(user.UpdateEmployerProfileRequest{CompanyOrOrg: "Unauthorized Corp"})
		req := httptest.NewRequest("PUT", "/profile/employer", bytes.NewReader(reqBody))
		req = req.WithContext(auth.WithUserContext(req.Context(), studentClaims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
		var env responseEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&env)
		if env.Error == nil || env.Error.Code != "FORBIDDEN" {
			t.Errorf("expected FORBIDDEN code, got: %+v", env.Error)
		}
	})

	t.Run("PUT /profile/employer validates length and returns 400 INVALID_INPUT", func(t *testing.T) {
		reqBody, _ := json.Marshal(user.UpdateEmployerProfileRequest{
			CompanyOrOrg: strings.Repeat("M", 201),
		})
		req := httptest.NewRequest("PUT", "/profile/employer", bytes.NewReader(reqBody))
		req = req.WithContext(auth.WithUserContext(req.Context(), employerClaims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
		var env responseEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&env)
		if env.Error == nil || env.Error.Code != "INVALID_INPUT" {
			t.Errorf("expected INVALID_INPUT, got: %+v", env.Error)
		}
	})

	t.Run("PUT /profile/employer by Employer succeeds (200 OK)", func(t *testing.T) {
		reqBody, _ := json.Marshal(user.UpdateEmployerProfileRequest{
			CompanyOrOrg: "Lynk Technologies",
			ContactName:  "Alex Founder",
			Description:  "Building campus freelance infrastructure",
			Website:      "https://lynk.edu",
		})
		req := httptest.NewRequest("PUT", "/profile/employer", bytes.NewReader(reqBody))
		req = req.WithContext(auth.WithUserContext(req.Context(), employerClaims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d. Body: %s", rr.Code, rr.Body.String())
		}
		var env responseEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&env)
		var ep user.EmployerProfile
		_ = json.Unmarshal(env.Data, &ep)
		if ep.CompanyOrOrg != "Lynk Technologies" || ep.Website != "https://lynk.edu" {
			t.Errorf("unexpected employer profile: %+v", ep)
		}
	})

	t.Run("GET /profile/employer after update returns 200 OK", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/profile/employer", nil)
		req = req.WithContext(auth.WithUserContext(req.Context(), employerClaims))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		var env responseEnvelope
		_ = json.NewDecoder(rr.Body).Decode(&env)
		var ep user.EmployerProfile
		_ = json.Unmarshal(env.Data, &ep)
		if ep.ContactName != "Alex Founder" {
			t.Errorf("expected ContactName 'Alex Founder', got '%s'", ep.ContactName)
		}
	})
}

func TestSubrouters(t *testing.T) {
	repo := newMockUserRepository()
	svc := user.NewService(repo)
	h := user.NewHandler(svc, repo)

	authR := h.AuthRoutes(func(next http.Handler) http.Handler { return next })
	profR := h.ProfileRoutes(func(next http.Handler) http.Handler { return next })

	claims := &auth.UserClaims{
		UserID:        uuid.New().String(),
		Email:         "subrouter@test.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}

	// Test auth subrouter
	reqAuth := httptest.NewRequest("GET", "/me", nil)
	reqAuth = reqAuth.WithContext(auth.WithUserContext(reqAuth.Context(), claims))
	rrAuth := httptest.NewRecorder()
	authR.ServeHTTP(rrAuth, reqAuth)
	if rrAuth.Code != http.StatusOK {
		t.Errorf("expected 200 on /me via AuthRoutes, got %d", rrAuth.Code)
	}

	// Test profile subrouter
	reqProf := httptest.NewRequest("GET", "/student", nil)
	reqProf = reqProf.WithContext(auth.WithUserContext(reqProf.Context(), claims))
	rrProf := httptest.NewRecorder()
	profR.ServeHTTP(rrProf, reqProf)
	if rrProf.Code != http.StatusOK {
		t.Errorf("expected 200 on /student via ProfileRoutes, got %d", rrProf.Code)
	}
}
