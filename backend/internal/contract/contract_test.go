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
	"github.com/lynk/backend/internal/httpx"
)

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

	applyCreateDefaults(contract)
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
			CreatedBy:   contract.ClientID,
			Title:       "Sample Job",
			Description: "Job description",
			BudgetCents: contract.AgreedBudgetCents,
			PayType:     "fixed",
			Department:  "Computer Science",
			Status:      "in_progress",
		},
		Client: &MemberSummary{
			ID:             contract.ClientID,
			Email:          "client@example.com",
			FirstName:      "Alice",
			LastName:       "Client",
			Department:     "Computer Science",
			GraduationYear: 2025,
		},
		Freelancer: &MemberSummary{
			ID:             contract.FreelancerID,
			Email:          "freelancer@university.edu",
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

func (m *mockContractRepo) ListContractsByUserID(ctx context.Context, userID string, limit, offset int) ([]*ContractWithDetails, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	list := make([]*ContractWithDetails, 0)
	for _, c := range m.contracts {
		if c.FreelancerID == userID || c.ClientID == userID {
			cp := *c
			list = append(list, &cp)
		}
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(list) {
		return []*ContractWithDetails{}, nil
	}
	end := offset + limit
	if end > len(list) {
		end = len(list)
	}
	return list[offset:end], nil
}

func (m *mockContractRepo) UpdateContractStatus(ctx context.Context, id uuid.UUID, targetStatus string) (*ContractWithDetails, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.contracts[id]
	if !ok {
		return nil, ErrContractNotFound
	}

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
		if c.Job != nil {
			c.Job.Status = "closed"
		} else {
			c.Job = &JobSummary{
				ID:     c.JobID,
				Status: "closed",
			}
		}
	}
	if targetStatus == StatusCancelled {
		if c.Job != nil {
			c.Job.Status = "open"
		} else {
			c.Job = &JobSummary{
				ID:     c.JobID,
				Status: "open",
			}
		}
	}

	cp := *c
	return &cp, nil
}

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

func TestCreateContract_DefaultsToActiveNotDraft(t *testing.T) {
	c := &Contract{
		JobID:             uuid.New(),
		ApplicationID:     uuid.New(),
		ClientID:          "c",
		FreelancerID:      "f",
		AgreedBudgetCents: 100,
	}
	if c.Status != "" {
		t.Fatal("fixture status must be empty to test default")
	}
	applyCreateDefaults(c)
	if c.Status != StatusActive {
		t.Fatalf("expected default active, got %s", c.Status)
	}
}

func TestContract_ValidateTransition(t *testing.T) {
	tests := []struct {
		name        string
		current     string
		target      string
		expectErr   bool
		expectedErr error
	}{
		{"draft to active", StatusDraft, StatusActive, false, nil},
		{"draft to cancelled", StatusDraft, StatusCancelled, false, nil},
		{"draft to completed disallowed", StatusDraft, StatusCompleted, true, ErrInvalidTransition},
		{"draft to draft disallowed", StatusDraft, StatusDraft, true, ErrInvalidTransition},

		{"active to completed", StatusActive, StatusCompleted, false, nil},
		{"active to cancelled", StatusActive, StatusCancelled, false, nil},
		{"active to draft disallowed", StatusActive, StatusDraft, true, ErrInvalidTransition},
		{"active to active disallowed", StatusActive, StatusActive, true, ErrInvalidTransition},

		{"completed to active disallowed", StatusCompleted, StatusActive, true, ErrTerminalStatus},
		{"completed to cancelled disallowed", StatusCompleted, StatusCancelled, true, ErrTerminalStatus},
		{"completed to draft disallowed", StatusCompleted, StatusDraft, true, ErrTerminalStatus},
		{"completed to completed disallowed", StatusCompleted, StatusCompleted, true, ErrInvalidTransition},

		{"cancelled to active disallowed", StatusCancelled, StatusActive, true, ErrTerminalStatus},
		{"cancelled to completed disallowed", StatusCancelled, StatusCompleted, true, ErrTerminalStatus},
		{"cancelled to draft disallowed", StatusCancelled, StatusDraft, true, ErrTerminalStatus},
		{"cancelled to cancelled disallowed", StatusCancelled, StatusCancelled, true, ErrInvalidTransition},

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

func TestContract_ParticipantAuthorization(t *testing.T) {
	repo := newMockContractRepo()
	service := NewService(repo)
	handler := NewHandler(service)
	router := handler.Routes(httpx.RequireAuthFromContext())

	freelancerID := uuid.New().String()
	clientID := uuid.New().String()
	unrelatedID := uuid.New().String()
	contractID := uuid.New()

	testContract := &Contract{
		ID:            contractID,
		JobID:         uuid.New(),
		ApplicationID: uuid.New(),
		ClientID:      clientID,
		FreelancerID:  freelancerID,
		AgreedBudgetCents:  50000,
		Status:        StatusActive,
	}
	_ = repo.CreateContract(context.Background(), testContract)

	freelancerClaims := &auth.UserClaims{
		UserID:        freelancerID,
		Email:         "student@uni.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	clientClaims := &auth.UserClaims{
		UserID:        clientID,
		Email:         "employer@corp.com",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	adminClaims := &auth.UserClaims{
		UserID:        uuid.New().String(),
		Email:         "admin@lynk.edu",
		EmailVerified: true,
		Roles:         []string{"admin"},
	}

	unrelatedClaims := &auth.UserClaims{
		UserID:        unrelatedID,
		Email:         "stranger@other.com",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	t.Run("freelancer participant can view contract (200 OK)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/contracts/%s", contractID), nil)
		req = withAuth(req, freelancerClaims)
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

	t.Run("client participant can view contract (200 OK)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/contracts/%s", contractID), nil)
		req = withAuth(req, clientClaims)
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
}

func TestContract_ListContracts(t *testing.T) {
	repo := newMockContractRepo()
	service := NewService(repo)
	handler := NewHandler(service)
	router := handler.Routes(httpx.RequireAuthFromContext())

	freelancerID := uuid.New().String()
	clientID := uuid.New().String()

	c1 := &Contract{
		ID:            uuid.New(),
		JobID:         uuid.New(),
		ApplicationID: uuid.New(),
		ClientID:      clientID,
		FreelancerID:  freelancerID,
		AgreedBudgetCents:  30000,
		Status:        StatusActive,
	}
	c2 := &Contract{
		ID:            uuid.New(),
		JobID:         uuid.New(),
		ApplicationID: uuid.New(),
		ClientID:      clientID,
		FreelancerID:  freelancerID,
		AgreedBudgetCents:  75000,
		Status:        StatusCompleted,
	}
	cOther := &Contract{
		ID:            uuid.New(),
		JobID:         uuid.New(),
		ApplicationID: uuid.New(),
		ClientID:      uuid.New().String(),
		FreelancerID:  uuid.New().String(),
		AgreedBudgetCents:  100000,
		Status:        StatusActive,
	}

	_ = repo.CreateContract(context.Background(), c1)
	_ = repo.CreateContract(context.Background(), c2)
	_ = repo.CreateContract(context.Background(), cOther)

	t.Run("member lists contracts where they are participant", func(t *testing.T) {
		claims := &auth.UserClaims{
			UserID:        freelancerID,
			Email:         "student@uni.edu",
			EmailVerified: true,
			Roles:         []string{"member"},
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
			t.Fatalf("expected 2 contracts, got %d", len(contracts))
		}
	})

	t.Run("unauthenticated call rejected (401)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
}

func TestContract_StateTransitions(t *testing.T) {
	repo := newMockContractRepo()
	service := NewService(repo)
	handler := NewHandler(service)
	router := handler.Routes(httpx.RequireAuthFromContext())

	freelancerID := uuid.New().String()
	clientID := uuid.New().String()

	freelancerClaims := &auth.UserClaims{
		UserID:        freelancerID,
		Email:         "student@uni.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	clientClaims := &auth.UserClaims{
		UserID:        clientID,
		Email:         "employer@corp.com",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	t.Run("draft to active transition succeeds and sets started_at", func(t *testing.T) {
		cID := uuid.New()
		jobID := uuid.New()
		now := time.Now().UTC()
		repo.contracts[cID] = &ContractWithDetails{
			Contract: Contract{
				ID:                cID,
				JobID:             jobID,
				ApplicationID:     uuid.New(),
				ClientID:          clientID,
				FreelancerID:      freelancerID,
				AgreedBudgetCents: 40000,
				Status:            StatusDraft,
				CreatedAt:         now,
				UpdatedAt:         now,
			},
			Job: &JobSummary{ID: jobID, CreatedBy: clientID, Status: "in_progress"},
		}

		payload, _ := json.Marshal(UpdateContractStatusRequest{Status: StatusActive})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/contracts/%s/status", cID), bytes.NewReader(payload))
		req = withAuth(req, clientClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var updated ContractWithDetails
		env := parseEnvelope(t, rec.Body.Bytes())
		_ = json.Unmarshal(env.Data, &updated)
		if updated.Status != StatusActive {
			t.Fatalf("expected status 'active', got '%s'", updated.Status)
		}
	})

	t.Run("active to completed transition succeeds and sets completed_at", func(t *testing.T) {
		cID := uuid.New()
		started := time.Now().Add(-2 * time.Hour)
		c := &Contract{
			ID:            cID,
			JobID:         uuid.New(),
			ApplicationID: uuid.New(),
			ClientID:      clientID,
			FreelancerID:  freelancerID,
			AgreedBudgetCents:  80000,
			Status:        StatusActive,
			StartedAt:     &started,
		}
		_ = repo.CreateContract(context.Background(), c)

		payload, _ := json.Marshal(UpdateContractStatusRequest{Status: StatusCompleted})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/contracts/%s/status", cID), bytes.NewReader(payload))
		req = withAuth(req, clientClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var updated ContractWithDetails
		env := parseEnvelope(t, rec.Body.Bytes())
		_ = json.Unmarshal(env.Data, &updated)
		if updated.Status != StatusCompleted {
			t.Fatalf("expected status 'completed', got '%s'", updated.Status)
		}
		if updated.Job == nil || updated.Job.Status != "closed" {
			t.Fatalf("expected job status 'closed', got %+v", updated.Job)
		}
	})

	t.Run("freelancer cannot mark active contract completed via HTTP (403 Forbidden)", func(t *testing.T) {
		cID := uuid.New()
		started := time.Now().Add(-2 * time.Hour)
		c := &Contract{
			ID:            cID,
			JobID:         uuid.New(),
			ApplicationID: uuid.New(),
			ClientID:      clientID,
			FreelancerID:  freelancerID,
			AgreedBudgetCents:  80000,
			Status:        StatusActive,
			StartedAt:     &started,
		}
		_ = repo.CreateContract(context.Background(), c)

		payload, _ := json.Marshal(UpdateContractStatusRequest{Status: StatusCompleted})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/contracts/%s/status", cID), bytes.NewReader(payload))
		req = withAuth(req, freelancerClaims)
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

	t.Run("client can cancel active contract via HTTP and reopen job (200 OK)", func(t *testing.T) {
		cID := uuid.New()
		c := &Contract{
			ID:            cID,
			JobID:         uuid.New(),
			ApplicationID: uuid.New(),
			ClientID:      clientID,
			FreelancerID:  freelancerID,
			AgreedBudgetCents:  65000,
			Status:        StatusActive,
		}
		_ = repo.CreateContract(context.Background(), c)

		payload, _ := json.Marshal(UpdateContractStatusRequest{Status: StatusCancelled})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/contracts/%s/status", cID), bytes.NewReader(payload))
		req = withAuth(req, clientClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var updated ContractWithDetails
		env := parseEnvelope(t, rec.Body.Bytes())
		_ = json.Unmarshal(env.Data, &updated)
		if updated.Status != StatusCancelled {
			t.Fatalf("expected status 'cancelled', got '%s'", updated.Status)
		}
		if updated.Job == nil || updated.Job.Status != "open" {
			t.Fatalf("expected job status 'open', got %+v", updated.Job)
		}
	})

	t.Run("freelancer can cancel active contract via HTTP and reopen job (200 OK)", func(t *testing.T) {
		cID := uuid.New()
		c := &Contract{
			ID:            cID,
			JobID:         uuid.New(),
			ApplicationID: uuid.New(),
			ClientID:      clientID,
			FreelancerID:  freelancerID,
			AgreedBudgetCents:  65000,
			Status:        StatusActive,
		}
		_ = repo.CreateContract(context.Background(), c)

		payload, _ := json.Marshal(UpdateContractStatusRequest{Status: StatusCancelled})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/contracts/%s/status", cID), bytes.NewReader(payload))
		req = withAuth(req, freelancerClaims)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var updated ContractWithDetails
		env := parseEnvelope(t, rec.Body.Bytes())
		_ = json.Unmarshal(env.Data, &updated)
		if updated.Status != StatusCancelled {
			t.Fatalf("expected status 'cancelled', got '%s'", updated.Status)
		}
		if updated.Job == nil || updated.Job.Status != "open" {
			t.Fatalf("expected job status 'open', got %+v", updated.Job)
		}
	})

	t.Run("unrelated user cannot cancel active contract via HTTP (403 Forbidden)", func(t *testing.T) {
		cID := uuid.New()
		c := &Contract{
			ID:            cID,
			JobID:         uuid.New(),
			ApplicationID: uuid.New(),
			ClientID:      clientID,
			FreelancerID:  freelancerID,
			AgreedBudgetCents:  65000,
			Status:        StatusActive,
		}
		_ = repo.CreateContract(context.Background(), c)

		unrelatedClaims := &auth.UserClaims{
			UserID:        uuid.New().String(),
			Email:         "stranger@other.edu",
			EmailVerified: true,
			Roles:         []string{"member"},
		}

		payload, _ := json.Marshal(UpdateContractStatusRequest{Status: StatusCancelled})
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/contracts/%s/status", cID), bytes.NewReader(payload))
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
}

func TestService_FreelancerCannotMarkContractCompleted(t *testing.T) {
	clientID := uuid.New().String()
	freelancerID := uuid.New().String()
	contractID := uuid.New()

	mockRepo := newMockContractRepo()
	mockRepo.contracts[contractID] = &ContractWithDetails{
		Contract: Contract{
			ID:           contractID,
			ClientID:     clientID,
			FreelancerID: freelancerID,
			Status:       StatusActive,
		},
	}
	svc := NewService(mockRepo)

	// Freelancer attempts to mark as completed -> MUST return ErrForbidden
	freelancerClaims := &auth.UserClaims{UserID: freelancerID, EmailVerified: true, Roles: []string{"member"}}
	_, err := svc.UpdateContractStatus(context.Background(), freelancerClaims, contractID, UpdateContractStatusRequest{Status: StatusCompleted})
	if err == nil {
		t.Fatalf("expected error when freelancer marks contract completed, got nil")
	}
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got: %v", err)
	}

	// Client marks as completed -> succeeds
	clientClaims := &auth.UserClaims{UserID: clientID, EmailVerified: true, Roles: []string{"member"}}
	updated, err := svc.UpdateContractStatus(context.Background(), clientClaims, contractID, UpdateContractStatusRequest{Status: StatusCompleted})
	if err != nil {
		t.Fatalf("expected client to successfully mark completed, got: %v", err)
	}
	if updated.Status != StatusCompleted {
		t.Fatalf("expected status %s, got %s", StatusCompleted, updated.Status)
	}
	if updated.Job == nil || updated.Job.Status != "closed" {
		t.Fatalf("expected job status 'closed' upon contract completion, got %+v", updated.Job)
	}

	// Admin marks as completed on a new active contract -> succeeds
	contract2ID := uuid.New()
	mockRepo.contracts[contract2ID] = &ContractWithDetails{
		Contract: Contract{
			ID:           contract2ID,
			ClientID:     clientID,
			FreelancerID: freelancerID,
			Status:       StatusActive,
		},
	}
	adminID := uuid.New().String()
	adminClaims := &auth.UserClaims{UserID: adminID, EmailVerified: true, Roles: []string{"admin"}}
	adminUpdated, err := svc.UpdateContractStatus(context.Background(), adminClaims, contract2ID, UpdateContractStatusRequest{Status: StatusCompleted})
	if err != nil {
		t.Fatalf("expected admin to successfully mark completed, got: %v", err)
	}
	if adminUpdated.Status != StatusCompleted {
		t.Fatalf("expected status %s, got %s", StatusCompleted, adminUpdated.Status)
	}
	if adminUpdated.Job == nil || adminUpdated.Job.Status != "closed" {
		t.Fatalf("expected job status 'closed' upon contract completion, got %+v", adminUpdated.Job)
	}
}

func TestContract_CompleteContractClosesJob(t *testing.T) {
	repo := newMockContractRepo()
	service := NewService(repo)

	clientID := uuid.New().String()
	freelancerID := uuid.New().String()
	contractID := uuid.New()
	jobID := uuid.New()

	c := &Contract{
		ID:            contractID,
		JobID:         jobID,
		ApplicationID: uuid.New(),
		ClientID:      clientID,
		FreelancerID:  freelancerID,
		AgreedBudgetCents:  50000,
		Status:        StatusActive,
	}
	if err := repo.CreateContract(context.Background(), c); err != nil {
		t.Fatalf("failed to create contract: %v", err)
	}

	// Verify initial job status is in_progress
	initial, err := repo.GetContractByID(context.Background(), contractID)
	if err != nil || initial == nil {
		t.Fatalf("failed to retrieve contract: %v", err)
	}
	if initial.Job == nil || initial.Job.Status != "in_progress" {
		t.Fatalf("expected initial job status to be 'in_progress', got %+v", initial.Job)
	}

	clientClaims := &auth.UserClaims{
		UserID:        clientID,
		Email:         "client@uni.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	updated, err := service.UpdateContractStatus(context.Background(), clientClaims, contractID, UpdateContractStatusRequest{Status: StatusCompleted})
	if err != nil {
		t.Fatalf("expected status update to succeed, got: %v", err)
	}

	if updated.Status != StatusCompleted {
		t.Fatalf("expected contract status %s, got %s", StatusCompleted, updated.Status)
	}
	if updated.Job == nil || updated.Job.Status != "closed" {
		t.Fatalf("expected job status 'closed' upon contract completion, got %+v", updated.Job)
	}
}

func TestContract_CancelContractReopensJob(t *testing.T) {
	repo := newMockContractRepo()
	service := NewService(repo)

	clientID := uuid.New().String()
	freelancerID := uuid.New().String()
	unrelatedID := uuid.New().String()

	clientClaims := &auth.UserClaims{
		UserID:        clientID,
		Email:         "client@uni.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}
	freelancerClaims := &auth.UserClaims{
		UserID:        freelancerID,
		Email:         "freelancer@uni.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}
	unrelatedClaims := &auth.UserClaims{
		UserID:        unrelatedID,
		Email:         "stranger@uni.edu",
		EmailVerified: true,
		Roles:         []string{"member"},
	}

	t.Run("client cancels active contract and reopens job", func(t *testing.T) {
		contractID := uuid.New()
		jobID := uuid.New()
		c := &Contract{
			ID:            contractID,
			JobID:         jobID,
			ApplicationID: uuid.New(),
			ClientID:      clientID,
			FreelancerID:  freelancerID,
			AgreedBudgetCents:  50000,
			Status:        StatusActive,
		}
		if err := repo.CreateContract(context.Background(), c); err != nil {
			t.Fatalf("failed to create contract: %v", err)
		}

		updated, err := service.UpdateContractStatus(context.Background(), clientClaims, contractID, UpdateContractStatusRequest{Status: StatusCancelled})
		if err != nil {
			t.Fatalf("expected cancel to succeed, got: %v", err)
		}
		if updated.Status != StatusCancelled {
			t.Fatalf("expected status 'cancelled', got '%s'", updated.Status)
		}
		if updated.Job == nil || updated.Job.Status != "open" {
			t.Fatalf("expected job status 'open', got %+v", updated.Job)
		}
	})

	t.Run("freelancer cancels active contract and reopens job", func(t *testing.T) {
		contractID := uuid.New()
		jobID := uuid.New()
		c := &Contract{
			ID:            contractID,
			JobID:         jobID,
			ApplicationID: uuid.New(),
			ClientID:      clientID,
			FreelancerID:  freelancerID,
			AgreedBudgetCents:  75000,
			Status:        StatusActive,
		}
		if err := repo.CreateContract(context.Background(), c); err != nil {
			t.Fatalf("failed to create contract: %v", err)
		}

		updated, err := service.UpdateContractStatus(context.Background(), freelancerClaims, contractID, UpdateContractStatusRequest{Status: StatusCancelled})
		if err != nil {
			t.Fatalf("expected cancel to succeed, got: %v", err)
		}
		if updated.Status != StatusCancelled {
			t.Fatalf("expected status 'cancelled', got '%s'", updated.Status)
		}
		if updated.Job == nil || updated.Job.Status != "open" {
			t.Fatalf("expected job status 'open', got %+v", updated.Job)
		}
	})

	t.Run("client cancels legacy draft contract and reopens job", func(t *testing.T) {
		contractID := uuid.New()
		jobID := uuid.New()
		now := time.Now().UTC()
		repo.contracts[contractID] = &ContractWithDetails{
			Contract: Contract{
				ID:                contractID,
				JobID:             jobID,
				ApplicationID:     uuid.New(),
				ClientID:          clientID,
				FreelancerID:      freelancerID,
				AgreedBudgetCents: 30000,
				Status:            StatusDraft,
				CreatedAt:         now,
				UpdatedAt:         now,
			},
			Job: &JobSummary{ID: jobID, CreatedBy: clientID, Status: "in_progress"},
		}

		updated, err := service.UpdateContractStatus(context.Background(), clientClaims, contractID, UpdateContractStatusRequest{Status: StatusCancelled})
		if err != nil {
			t.Fatalf("expected cancel to succeed, got: %v", err)
		}
		if updated.Status != StatusCancelled {
			t.Fatalf("expected status 'cancelled', got '%s'", updated.Status)
		}
		if updated.Job == nil || updated.Job.Status != "open" {
			t.Fatalf("expected job status 'open', got %+v", updated.Job)
		}
	})

	t.Run("unrelated user cannot cancel contract", func(t *testing.T) {
		contractID := uuid.New()
		c := &Contract{
			ID:            contractID,
			JobID:         uuid.New(),
			ApplicationID: uuid.New(),
			ClientID:      clientID,
			FreelancerID:  freelancerID,
			AgreedBudgetCents:  50000,
			Status:        StatusActive,
		}
		if err := repo.CreateContract(context.Background(), c); err != nil {
			t.Fatalf("failed to create contract: %v", err)
		}

		_, err := service.UpdateContractStatus(context.Background(), unrelatedClaims, contractID, UpdateContractStatusRequest{Status: StatusCancelled})
		if err == nil {
			t.Fatalf("expected error for unrelated user, got nil")
		}
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got: %v", err)
		}
	})

	t.Run("cannot cancel completed contract", func(t *testing.T) {
		contractID := uuid.New()
		c := &Contract{
			ID:            contractID,
			JobID:         uuid.New(),
			ApplicationID: uuid.New(),
			ClientID:      clientID,
			FreelancerID:  freelancerID,
			AgreedBudgetCents:  50000,
			Status:        StatusCompleted,
		}
		if err := repo.CreateContract(context.Background(), c); err != nil {
			t.Fatalf("failed to create contract: %v", err)
		}

		_, err := service.UpdateContractStatus(context.Background(), clientClaims, contractID, UpdateContractStatusRequest{Status: StatusCancelled})
		if err == nil {
			t.Fatalf("expected error cancelling completed contract, got nil")
		}
		if !errors.Is(err, ErrTerminalStatus) {
			t.Fatalf("expected ErrTerminalStatus, got: %v", err)
		}
	})

	t.Run("cannot cancel already cancelled contract", func(t *testing.T) {
		contractID := uuid.New()
		c := &Contract{
			ID:            contractID,
			JobID:         uuid.New(),
			ApplicationID: uuid.New(),
			ClientID:      clientID,
			FreelancerID:  freelancerID,
			AgreedBudgetCents:  50000,
			Status:        StatusCancelled,
		}
		if err := repo.CreateContract(context.Background(), c); err != nil {
			t.Fatalf("failed to create contract: %v", err)
		}

		_, err := service.UpdateContractStatus(context.Background(), clientClaims, contractID, UpdateContractStatusRequest{Status: StatusCancelled})
		if err == nil {
			t.Fatalf("expected error cancelling cancelled contract, got nil")
		}
		if !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("expected ErrInvalidTransition, got: %v", err)
		}
	})
}
