package contract

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/middleware"
)

// mockContractRepo implements ContractRepository in memory for unit and integration testing.
type mockContractRepo struct {
	mu        sync.Mutex
	contracts map[uuid.UUID]*ContractWithDetails
}

func newMockContractRepo() *mockContractRepo {
	return &mockContractRepo{
		contracts: make(map[uuid.UUID]*ContractWithDetails),
	}
}

func (m *mockContractRepo) CreateContract(ctx context.Context, contract *Contract) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if contract.ID == uuid.Nil {
		contract.ID = uuid.New()
	}
	if contract.Status == "" {
		contract.Status = StatusDraft
	}
	now := time.Now().UTC()
	if contract.Status == StatusActive && contract.StartedAt == nil {
		contract.StartedAt = &now
	}
	if contract.Status == StatusCompleted && contract.CompletedAt == nil {
		contract.CompletedAt = &now
	}
	contract.CreatedAt = now
	contract.UpdatedAt = now

	m.contracts[contract.ID] = &ContractWithDetails{
		Contract: *contract,
		Job: &JobSummary{
			ID:          contract.JobID,
			EmployerID:  contract.EmployerID,
			Title:       "Sample Job",
			Description: "Job description",
			Budget:      contract.AgreedBudget,
			PayType:     "fixed",
			Department:  "Computer Science",
			Status:      "in_progress",
		},
		Employer: &EmployerSummary{
			ID:           contract.EmployerID,
			Email:        "employer@example.com",
			CompanyOrOrg: "Test Org",
			ContactName:  "Test Employer",
		},
		Student: &StudentSummary{
			ID:             contract.StudentID,
			Email:          "student@university.edu",
			FirstName:      "John",
			LastName:       "Doe",
			Department:     "Computer Science",
			GraduationYear: 2026,
		},
	}
	return nil
}

func (m *mockContractRepo) GetContractByID(ctx context.Context, id uuid.UUID) (*ContractWithDetails, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.contracts[id]
	if !ok {
		return nil, nil
	}
	cp := *c
	return &cp, nil
}

func (m *mockContractRepo) ListContractsByUserID(ctx context.Context, userID uuid.UUID) ([]*ContractWithDetails, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	list := make([]*ContractWithDetails, 0)
	for _, c := range m.contracts {
		if c.StudentID == userID || c.EmployerID == userID {
			cp := *c
			list = append(list, &cp)
		}
	}
	return list, nil
}

func (m *mockContractRepo) UpdateContractStatus(ctx context.Context, id uuid.UUID, targetStatus string) (*ContractWithDetails, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.contracts[id]
	if !ok {
		return nil, ErrContractNotFound
	}

	// Concurrency guard matching SQL: WHERE status NOT IN ('completed', 'cancelled')
	if c.Status == StatusCompleted || c.Status == StatusCancelled {
		return nil, ErrTerminalStatus
	}

	now := time.Now().UTC()
	c.Status = targetStatus
	c.UpdatedAt = now

	if targetStatus == StatusActive && c.StartedAt == nil {
		c.StartedAt = &now
	}
	if targetStatus == StatusCompleted {
		c.CompletedAt = &now
	}

	cp := *c
	return &cp, nil
}


// mockValidator implements auth.TokenValidator for integration testing.
type mockValidator struct {
	claimsMap map[string]*auth.UserClaims
}

func (m *mockValidator) ValidateToken(ctx context.Context, tokenStr string) (*auth.UserClaims, error) {
	if claims, ok := m.claimsMap[tokenStr]; ok {
		return claims, nil
	}
	return nil, errors.New("invalid or expired token")
}

func withAuth(r *http.Request, claims *auth.UserClaims) *http.Request {
	ctx := auth.WithUserContext(r.Context(), claims)
	return r.WithContext(ctx)
}

type jsonEnvelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func parseEnvelope(t *testing.T, body []byte) jsonEnvelope {
	t.Helper()
	var res jsonEnvelope
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("failed to unmarshal JSON response: %v, raw: %s", err, string(body))
	}
	return res
}

// --- Unit Tests: State Machine Transition Logic ---

func TestContract_ValidateTransition(t *testing.T) {
	tests := []struct {
		name        string
		current     string
		target      string
		expectErr   bool
		expectedErr error
	}{
		// Valid transitions from draft
		{"draft to active", StatusDraft, StatusActive, false, nil},
		{"draft to cancelled", StatusDraft, StatusCancelled, false, nil},
		// Invalid transitions from draft
		{"draft to completed disallowed", StatusDraft, StatusCompleted, true, ErrInvalidTransition},
		{"draft to draft disallowed", StatusDraft, StatusDraft, true, ErrInvalidTransition},

		// Valid transitions from active
		{"active to completed", StatusActive, StatusCompleted, false, nil},
		{"active to cancelled", StatusActive, StatusCancelled, false, nil},
		// Invalid transitions from active
		{"active to draft disallowed", StatusActive, StatusDraft, true, ErrInvalidTransition},
		{"active to active disallowed", StatusActive, StatusActive, true, ErrInvalidTransition},

		// Terminal states cannot transition to anything
		{"completed to active disallowed", StatusCompleted, StatusActive, true, ErrTerminalStatus},
		{"completed to cancelled disallowed", StatusCompleted, StatusCancelled, true, ErrTerminalStatus},
		{"completed to draft disallowed", StatusCompleted, StatusDraft, true, ErrTerminalStatus},
		{"completed to completed disallowed", StatusCompleted, StatusCompleted, true, ErrInvalidTransition},

		{"cancelled to active disallowed", StatusCancelled, StatusActive, true, ErrTerminalStatus},
		{"cancelled to completed disallowed", StatusCancelled, StatusCompleted, true, ErrTerminalStatus},
		{"cancelled to draft disallowed", StatusCancelled, StatusDraft, true, ErrTerminalStatus},
		{"cancelled to cancelled disallowed", StatusCancelled, StatusCancelled, true, ErrInvalidTransition},

		// Invalid status strings
		{"empty target status", StatusActive, "", true, ErrInvalidInput},
		{"unknown target status", StatusActive, "pending", true, ErrInvalidStatus},
		{"unknown current status", "pending", StatusActive, true, ErrInvalidStatus},
		{"case insensitive valid", "DRAFT", "ACTIVE", false, nil},
		{"trimmed whitespace valid", " active ", " completed ", false, nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTransition(tc.current, tc.target)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error for transition %s -> %s, got nil", tc.current, tc.target)
				}
				if tc.expectedErr != nil && !errors.Is(err, tc.expectedErr) {
					t.Fatalf("expected error wrapping %v, got %v", tc.expectedErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected valid transition for %s -> %s, got error: %v", tc.current, tc.target, err)
				}
			}
		})
	}
}

// --- Unit Tests: Participant Authorization ---

func TestContract_ParticipantAuthorization(t *testing.T) {
	repo := newMockContractRepo()
	service := NewService(repo)
	handler := NewHandler(service)
	router := handler.Routes(nil)

	studentID := uuid.New()
	employerID := uuid.New()
	unrelatedID := uuid.New()
	contractID := uuid.New()

	testContract := &Contract{
		ID:            contractID,
		JobID:         uuid.New(),
		ApplicationID: uuid.New(),
		EmployerID:    employerID,
		StudentID:     studentID,
		AgreedBudget:  500.0,
		Status:        StatusActive,
	}
	_ = repo.CreateContract(context.Background(), testContract)

	studentClaims := &auth.UserClaims{
		UserID:        studentID.String(),
		Email:         "student@uni.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}

	employerClaims := &auth.UserClaims{
		UserID:        employerID.String(),
		Email:         "employer@corp.com",
		EmailVerified: true,
		Roles:         []string{"employer"},
	}

	adminClaims := &auth.UserClaims{
		UserID:        uuid.New().String(),
		Email:         "admin@lynk.edu",
		EmailVerified: true,
		Roles:         []string{"admin"},
	}

	unrelatedClaims := &auth.UserClaims{
		UserID:        unrelatedID.String(),
		Email:         "stranger@other.com",
		EmailVerified: true,
		Roles:         []string{"student"},
	}

	t.Run("student participant can view contract (200 OK)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/contracts/%s", contractID), nil)
		req = withAuth(req, studentClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
		}
		env := parseEnvelope(t, rec.Body.Bytes())
		if !env.Success {
			t.Fatalf("expected success=true, got error: %+v", env.Error)
		}
	})

	t.Run("employer participant can view contract (200 OK)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/contracts/%s", contractID), nil)
		req = withAuth(req, employerClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		env := parseEnvelope(t, rec.Body.Bytes())
		if !env.Success {
			t.Fatalf("expected success=true")
		}
	})

	t.Run("admin can view any contract (200 OK)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/contracts/%s", contractID), nil)
		req = withAuth(req, adminClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		env := parseEnvelope(t, rec.Body.Bytes())
		if !env.Success {
			t.Fatalf("expected success=true")
		}
	})

	t.Run("unrelated user cannot view contract (403 Forbidden)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/contracts/%s", contractID), nil)
		req = withAuth(req, unrelatedClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d", rec.Code)
		}
		env := parseEnvelope(t, rec.Body.Bytes())
		if env.Success {
			t.Fatalf("expected success=false")
		}
		if env.Error == nil || env.Error.Code != "FORBIDDEN" {
			t.Fatalf("expected code FORBIDDEN, got %+v", env.Error)
		}
	})

	t.Run("unauthenticated request to view contract returns 401 Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/contracts/%s", contractID), nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
		}
		env := parseEnvelope(t, rec.Body.Bytes())
		if env.Error == nil || env.Error.Code != "UNAUTHORIZED" {
			t.Fatalf("expected code UNAUTHORIZED, got %+v", env.Error)
		}
	})

	t.Run("non-existent contract returns 404 CONTRACT_NOT_FOUND", func(t *testing.T) {
		nonExistentID := uuid.New()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/contracts/%s", nonExistentID), nil)
		req = withAuth(req, studentClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rec.Code)
		}
		env := parseEnvelope(t, rec.Body.Bytes())
		if env.Error == nil || env.Error.Code != "CONTRACT_NOT_FOUND" {
			t.Fatalf("expected code CONTRACT_NOT_FOUND, got %+v", env.Error)
		}
	})

	t.Run("invalid contract UUID returns 400 INVALID_ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/invalid-uuid", nil)
		req = withAuth(req, studentClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
		env := parseEnvelope(t, rec.Body.Bytes())
		if env.Error == nil || env.Error.Code != "INVALID_ID" {
			t.Fatalf("expected code INVALID_ID, got %+v", env.Error)
		}
	})
}

// --- Unit Tests: List Contracts ---

func TestContract_ListContracts(t *testing.T) {
	repo := newMockContractRepo()
	service := NewService(repo)
	handler := NewHandler(service)
	router := handler.Routes(nil)

	studentID := uuid.New()
	employerID := uuid.New()

	c1 := &Contract{
		ID:            uuid.New(),
		JobID:         uuid.New(),
		ApplicationID: uuid.New(),
		EmployerID:    employerID,
		StudentID:     studentID,
		AgreedBudget:  300.0,
		Status:        StatusActive,
	}
	c2 := &Contract{
		ID:            uuid.New(),
		JobID:         uuid.New(),
		ApplicationID: uuid.New(),
		EmployerID:    employerID,
		StudentID:     studentID,
		AgreedBudget:  750.0,
		Status:        StatusCompleted,
	}
	cOther := &Contract{
		ID:            uuid.New(),
		JobID:         uuid.New(),
		ApplicationID: uuid.New(),
		EmployerID:    uuid.New(),
		StudentID:     uuid.New(),
		AgreedBudget:  1000.0,
		Status:        StatusActive,
	}

	_ = repo.CreateContract(context.Background(), c1)
	_ = repo.CreateContract(context.Background(), c2)
	_ = repo.CreateContract(context.Background(), cOther)

	t.Run("student lists only contracts where they are participant", func(t *testing.T) {
		claims := &auth.UserClaims{
			UserID:        studentID.String(),
			Email:         "student@uni.edu",
			EmailVerified: true,
			Roles:         []string{"student"},
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil)
		req = withAuth(req, claims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		env := parseEnvelope(t, rec.Body.Bytes())
		if !env.Success {
			t.Fatalf("expected success=true")
		}

		var contracts []*ContractWithDetails
		if err := json.Unmarshal(env.Data, &contracts); err != nil {
			t.Fatalf("failed to decode contracts: %v", err)
		}
		if len(contracts) != 2 {
			t.Fatalf("expected 2 contracts for student, got %d", len(contracts))
		}
	})

	t.Run("employer lists only contracts where they are employer", func(t *testing.T) {
		claims := &auth.UserClaims{
			UserID:        employerID.String(),
			Email:         "employer@corp.com",
			EmailVerified: true,
			Roles:         []string{"employer"},
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil)
		req = withAuth(req, claims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		env := parseEnvelope(t, rec.Body.Bytes())
		var contracts []*ContractWithDetails
		_ = json.Unmarshal(env.Data, &contracts)
		if len(contracts) != 2 {
			t.Fatalf("expected 2 contracts for employer, got %d", len(contracts))
		}
	})

	t.Run("user with no contracts receives empty JSON list", func(t *testing.T) {
		nobodyClaims := &auth.UserClaims{
			UserID:        uuid.New().String(),
			Email:         "nobody@uni.edu",
			EmailVerified: true,
			Roles:         []string{"student"},
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil)
		req = withAuth(req, nobodyClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		env := parseEnvelope(t, rec.Body.Bytes())
		var contracts []*ContractWithDetails
		_ = json.Unmarshal(env.Data, &contracts)
		if contracts == nil || len(contracts) != 0 {
			t.Fatalf("expected empty non-nil slice, got %v", contracts)
		}
	})

	t.Run("unauthenticated list request returns 401 Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
		}
	})
}

// --- Unit Tests: Status Transitions & State Machine Execution ---

func TestContract_StatusTransitions(t *testing.T) {
	repo := newMockContractRepo()
	service := NewService(repo)
	handler := NewHandler(service)
	router := handler.Routes(nil)

	studentID := uuid.New()
	employerID := uuid.New()
	otherID := uuid.New()

	studentClaims := &auth.UserClaims{
		UserID:        studentID.String(),
		Email:         "student@uni.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}

	employerClaims := &auth.UserClaims{
		UserID:        employerID.String(),
		Email:         "employer@corp.com",
		EmailVerified: true,
		Roles:         []string{"employer"},
	}

	adminClaims := &auth.UserClaims{
		UserID:        uuid.New().String(),
		Email:         "admin@lynk.edu",
		EmailVerified: true,
		Roles:         []string{"admin"},
	}

	unrelatedClaims := &auth.UserClaims{
		UserID:        otherID.String(),
		Email:         "stranger@other.com",
		EmailVerified: true,
		Roles:         []string{"student"},
	}

	t.Run("draft to active transition succeeds and sets started_at", func(t *testing.T) {
		cID := uuid.New()
		c := &Contract{
			ID:            cID,
			JobID:         uuid.New(),
			ApplicationID: uuid.New(),
			EmployerID:    employerID,
			StudentID:     studentID,
			AgreedBudget:  400.0,
			Status:        StatusDraft,
		}
		_ = repo.CreateContract(context.Background(), c)

		payload, _ := json.Marshal(UpdateContractStatusRequest{Status: StatusActive})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/contracts/%s/status", cID), bytes.NewReader(payload))
		req = withAuth(req, employerClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		env := parseEnvelope(t, rec.Body.Bytes())
		if !env.Success {
			t.Fatalf("expected success=true")
		}

		var updated ContractWithDetails
		if err := json.Unmarshal(env.Data, &updated); err != nil {
			t.Fatalf("failed to decode updated contract: %v", err)
		}
		if updated.Status != StatusActive {
			t.Fatalf("expected status 'active', got '%s'", updated.Status)
		}
		if updated.StartedAt == nil {
			t.Fatalf("expected started_at timestamp to be set")
		}
	})

	t.Run("active to completed transition succeeds and sets completed_at = NOW()", func(t *testing.T) {
		cID := uuid.New()
		started := time.Now().Add(-2 * time.Hour)
		c := &Contract{
			ID:            cID,
			JobID:         uuid.New(),
			ApplicationID: uuid.New(),
			EmployerID:    employerID,
			StudentID:     studentID,
			AgreedBudget:  800.0,
			Status:        StatusActive,
			StartedAt:     &started,
		}
		_ = repo.CreateContract(context.Background(), c)

		payload, _ := json.Marshal(UpdateContractStatusRequest{Status: StatusCompleted})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/contracts/%s/status", cID), bytes.NewReader(payload))
		req = withAuth(req, studentClaims) // Student participant can complete
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		env := parseEnvelope(t, rec.Body.Bytes())
		var updated ContractWithDetails
		_ = json.Unmarshal(env.Data, &updated)

		if updated.Status != StatusCompleted {
			t.Fatalf("expected status 'completed', got '%s'", updated.Status)
		}
		if updated.CompletedAt == nil {
			t.Fatalf("expected completed_at timestamp to be set")
		}
	})

	t.Run("active to cancelled transition succeeds", func(t *testing.T) {
		cID := uuid.New()
		started := time.Now().Add(-1 * time.Hour)
		c := &Contract{
			ID:            cID,
			JobID:         uuid.New(),
			ApplicationID: uuid.New(),
			EmployerID:    employerID,
			StudentID:     studentID,
			AgreedBudget:  500.0,
			Status:        StatusActive,
			StartedAt:     &started,
		}
		_ = repo.CreateContract(context.Background(), c)

		payload, _ := json.Marshal(UpdateContractStatusRequest{Status: StatusCancelled})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/contracts/%s/status", cID), bytes.NewReader(payload))
		req = withAuth(req, employerClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		env := parseEnvelope(t, rec.Body.Bytes())
		var updated ContractWithDetails
		_ = json.Unmarshal(env.Data, &updated)
		if updated.Status != StatusCancelled {
			t.Fatalf("expected status 'cancelled', got '%s'", updated.Status)
		}
	})

	t.Run("draft to cancelled transition succeeds", func(t *testing.T) {
		cID := uuid.New()
		c := &Contract{
			ID:            cID,
			JobID:         uuid.New(),
			ApplicationID: uuid.New(),
			EmployerID:    employerID,
			StudentID:     studentID,
			AgreedBudget:  250.0,
			Status:        StatusDraft,
		}
		_ = repo.CreateContract(context.Background(), c)

		payload, _ := json.Marshal(UpdateContractStatusRequest{Status: StatusCancelled})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/contracts/%s/status", cID), bytes.NewReader(payload))
		req = withAuth(req, studentClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		env := parseEnvelope(t, rec.Body.Bytes())
		var updated ContractWithDetails
		_ = json.Unmarshal(env.Data, &updated)
		if updated.Status != StatusCancelled {
			t.Fatalf("expected status 'cancelled', got '%s'", updated.Status)
		}
	})

	t.Run("terminal status: completed cannot transition to active (400 INVALID_TRANSITION)", func(t *testing.T) {
		cID := uuid.New()
		c := &Contract{
			ID:            cID,
			JobID:         uuid.New(),
			ApplicationID: uuid.New(),
			EmployerID:    employerID,
			StudentID:     studentID,
			AgreedBudget:  500.0,
			Status:        StatusCompleted,
		}
		_ = repo.CreateContract(context.Background(), c)

		payload, _ := json.Marshal(UpdateContractStatusRequest{Status: StatusActive})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/contracts/%s/status", cID), bytes.NewReader(payload))
		req = withAuth(req, employerClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		env := parseEnvelope(t, rec.Body.Bytes())
		if env.Error == nil || env.Error.Code != "INVALID_TRANSITION" {
			t.Fatalf("expected code INVALID_TRANSITION, got %+v", env.Error)
		}
	})

	t.Run("terminal status: cancelled cannot transition to completed (400 INVALID_TRANSITION)", func(t *testing.T) {
		cID := uuid.New()
		c := &Contract{
			ID:            cID,
			JobID:         uuid.New(),
			ApplicationID: uuid.New(),
			EmployerID:    employerID,
			StudentID:     studentID,
			AgreedBudget:  500.0,
			Status:        StatusCancelled,
		}
		_ = repo.CreateContract(context.Background(), c)

		payload, _ := json.Marshal(UpdateContractStatusRequest{Status: StatusCompleted})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/contracts/%s/status", cID), bytes.NewReader(payload))
		req = withAuth(req, studentClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}

		env := parseEnvelope(t, rec.Body.Bytes())
		if env.Error == nil || env.Error.Code != "INVALID_TRANSITION" {
			t.Fatalf("expected code INVALID_TRANSITION, got %+v", env.Error)
		}
	})

	t.Run("draft cannot transition directly to completed (400 INVALID_TRANSITION)", func(t *testing.T) {
		cID := uuid.New()
		c := &Contract{
			ID:            cID,
			JobID:         uuid.New(),
			ApplicationID: uuid.New(),
			EmployerID:    employerID,
			StudentID:     studentID,
			AgreedBudget:  500.0,
			Status:        StatusDraft,
		}
		_ = repo.CreateContract(context.Background(), c)

		payload, _ := json.Marshal(UpdateContractStatusRequest{Status: StatusCompleted})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/contracts/%s/status", cID), bytes.NewReader(payload))
		req = withAuth(req, employerClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}

		env := parseEnvelope(t, rec.Body.Bytes())
		if env.Error == nil || env.Error.Code != "INVALID_TRANSITION" {
			t.Fatalf("expected code INVALID_TRANSITION, got %+v", env.Error)
		}
	})

	t.Run("transitioning to same status returns 400 INVALID_TRANSITION", func(t *testing.T) {
		cID := uuid.New()
		c := &Contract{
			ID:            cID,
			JobID:         uuid.New(),
			ApplicationID: uuid.New(),
			EmployerID:    employerID,
			StudentID:     studentID,
			AgreedBudget:  500.0,
			Status:        StatusActive,
		}
		_ = repo.CreateContract(context.Background(), c)

		payload, _ := json.Marshal(UpdateContractStatusRequest{Status: StatusActive})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/contracts/%s/status", cID), bytes.NewReader(payload))
		req = withAuth(req, employerClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}

		env := parseEnvelope(t, rec.Body.Bytes())
		if env.Error == nil || env.Error.Code != "INVALID_TRANSITION" {
			t.Fatalf("expected code INVALID_TRANSITION, got %+v", env.Error)
		}
	})

	t.Run("transitioning to invalid status name returns 400 INVALID_STATUS", func(t *testing.T) {
		cID := uuid.New()
		c := &Contract{
			ID:            cID,
			JobID:         uuid.New(),
			ApplicationID: uuid.New(),
			EmployerID:    employerID,
			StudentID:     studentID,
			AgreedBudget:  500.0,
			Status:        StatusActive,
		}
		_ = repo.CreateContract(context.Background(), c)

		payload, _ := json.Marshal(UpdateContractStatusRequest{Status: "pending"})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/contracts/%s/status", cID), bytes.NewReader(payload))
		req = withAuth(req, employerClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}

		env := parseEnvelope(t, rec.Body.Bytes())
		if env.Error == nil || env.Error.Code != "INVALID_STATUS" {
			t.Fatalf("expected code INVALID_STATUS, got %+v", env.Error)
		}
	})

	t.Run("non-participant cannot update contract status (403 Forbidden)", func(t *testing.T) {
		cID := uuid.New()
		c := &Contract{
			ID:            cID,
			JobID:         uuid.New(),
			ApplicationID: uuid.New(),
			EmployerID:    employerID,
			StudentID:     studentID,
			AgreedBudget:  500.0,
			Status:        StatusActive,
		}
		_ = repo.CreateContract(context.Background(), c)

		payload, _ := json.Marshal(UpdateContractStatusRequest{Status: StatusCompleted})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/contracts/%s/status", cID), bytes.NewReader(payload))
		req = withAuth(req, unrelatedClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d", rec.Code)
		}

		env := parseEnvelope(t, rec.Body.Bytes())
		if env.Error == nil || env.Error.Code != "FORBIDDEN" {
			t.Fatalf("expected code FORBIDDEN, got %+v", env.Error)
		}
	})

	t.Run("admin can update contract status even if not participant", func(t *testing.T) {
		cID := uuid.New()
		c := &Contract{
			ID:            cID,
			JobID:         uuid.New(),
			ApplicationID: uuid.New(),
			EmployerID:    employerID,
			StudentID:     studentID,
			AgreedBudget:  500.0,
			Status:        StatusActive,
		}
		_ = repo.CreateContract(context.Background(), c)

		payload, _ := json.Marshal(UpdateContractStatusRequest{Status: StatusCancelled})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/contracts/%s/status", cID), bytes.NewReader(payload))
		req = withAuth(req, adminClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		env := parseEnvelope(t, rec.Body.Bytes())
		var updated ContractWithDetails
		_ = json.Unmarshal(env.Data, &updated)
		if updated.Status != StatusCancelled {
			t.Fatalf("expected status 'cancelled', got '%s'", updated.Status)
		}
	})

	t.Run("updating non-existent contract returns 404 CONTRACT_NOT_FOUND", func(t *testing.T) {
		nonExistentID := uuid.New()
		payload, _ := json.Marshal(UpdateContractStatusRequest{Status: StatusCancelled})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/contracts/%s/status", nonExistentID), bytes.NewReader(payload))
		req = withAuth(req, employerClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rec.Code)
		}

		env := parseEnvelope(t, rec.Body.Bytes())
		if env.Error == nil || env.Error.Code != "CONTRACT_NOT_FOUND" {
			t.Fatalf("expected code CONTRACT_NOT_FOUND, got %+v", env.Error)
		}
	})

	t.Run("empty request body returns 400 BAD_REQUEST", func(t *testing.T) {
		cID := uuid.New()
		c := &Contract{
			ID:            cID,
			JobID:         uuid.New(),
			ApplicationID: uuid.New(),
			EmployerID:    employerID,
			StudentID:     studentID,
			AgreedBudget:  500.0,
			Status:        StatusActive,
		}
		_ = repo.CreateContract(context.Background(), c)

		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/contracts/%s/status", cID), bytes.NewReader([]byte("")))
		req = withAuth(req, employerClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
		env := parseEnvelope(t, rec.Body.Bytes())
		if env.Error == nil || env.Error.Code != "BAD_REQUEST" {
			t.Fatalf("expected code BAD_REQUEST, got %+v", env.Error)
		}
	})

	t.Run("empty status string returns 400 INVALID_INPUT", func(t *testing.T) {
		cID := uuid.New()
		c := &Contract{
			ID:            cID,
			JobID:         uuid.New(),
			ApplicationID: uuid.New(),
			EmployerID:    employerID,
			StudentID:     studentID,
			AgreedBudget:  500.0,
			Status:        StatusActive,
		}
		_ = repo.CreateContract(context.Background(), c)

		payload, _ := json.Marshal(UpdateContractStatusRequest{Status: "   "})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/contracts/%s/status", cID), bytes.NewReader(payload))
		req = withAuth(req, employerClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
		env := parseEnvelope(t, rec.Body.Bytes())
		if env.Error == nil || env.Error.Code != "INVALID_INPUT" {
			t.Fatalf("expected code INVALID_INPUT, got %+v", env.Error)
		}
	})
}

// --- Integration Tests: Auth Middleware with Bearer Tokens ---

func TestContract_AuthMiddlewareIntegration(t *testing.T) {
	repo := newMockContractRepo()
	service := NewService(repo)
	handler := NewHandler(service)

	studentID := uuid.New()
	employerID := uuid.New()
	contractID := uuid.New()

	c := &Contract{
		ID:            contractID,
		JobID:         uuid.New(),
		ApplicationID: uuid.New(),
		EmployerID:    employerID,
		StudentID:     studentID,
		AgreedBudget:  650.0,
		Status:        StatusActive,
	}
	_ = repo.CreateContract(context.Background(), c)

	validToken := "valid-jwt-token"
	validator := &mockValidator{
		claimsMap: map[string]*auth.UserClaims{
			validToken: {
				UserID:        studentID.String(),
				Email:         "student@uni.edu",
				EmailVerified: true,
				Roles:         []string{"student"},
			},
		},
	}

	authMw := middleware.AuthMiddleware(validator)
	router := handler.Routes(authMw)

	t.Run("request with valid bearer token succeeds (200 OK)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/contracts/%s", contractID), nil)
		req.Header.Set("Authorization", "Bearer "+validToken)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
		}
		env := parseEnvelope(t, rec.Body.Bytes())
		if !env.Success {
			t.Fatalf("expected success=true")
		}
	})

	t.Run("request with invalid bearer token returns 401 Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/contracts/%s", contractID), nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
		}
	})

	t.Run("request without Authorization header returns 401 Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/contracts/%s", contractID), nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
		}
	})
}

// --- Unit Tests: Repository Concurrency Guard ---

func TestContract_Repository_TerminalStateGuard(t *testing.T) {
	repo := newMockContractRepo()

	completedID := uuid.New()
	_ = repo.CreateContract(context.Background(), &Contract{
		ID:            completedID,
		JobID:         uuid.New(),
		ApplicationID: uuid.New(),
		EmployerID:    uuid.New(),
		StudentID:     uuid.New(),
		AgreedBudget:  500.0,
		Status:        StatusCompleted,
	})

	cancelledID := uuid.New()
	_ = repo.CreateContract(context.Background(), &Contract{
		ID:            cancelledID,
		JobID:         uuid.New(),
		ApplicationID: uuid.New(),
		EmployerID:    uuid.New(),
		StudentID:     uuid.New(),
		AgreedBudget:  500.0,
		Status:        StatusCancelled,
	})

	t.Run("concurrency guard prevents mutating completed contract at repository level", func(t *testing.T) {
		_, err := repo.UpdateContractStatus(context.Background(), completedID, StatusActive)
		if !errors.Is(err, ErrTerminalStatus) {
			t.Fatalf("expected ErrTerminalStatus, got %v", err)
		}
	})

	t.Run("concurrency guard prevents mutating cancelled contract at repository level", func(t *testing.T) {
		_, err := repo.UpdateContractStatus(context.Background(), cancelledID, StatusCompleted)
		if !errors.Is(err, ErrTerminalStatus) {
			t.Fatalf("expected ErrTerminalStatus, got %v", err)
		}
	})

	t.Run("updating non-existent contract returns ErrContractNotFound", func(t *testing.T) {
		_, err := repo.UpdateContractStatus(context.Background(), uuid.New(), StatusCompleted)
		if !errors.Is(err, ErrContractNotFound) {
			t.Fatalf("expected ErrContractNotFound, got %v", err)
		}
	})
}

