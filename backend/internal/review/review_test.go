package review

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/contract"
	"github.com/lynk/backend/internal/middleware"
)

// mockReviewRepo implements ReviewRepository in-memory for testing.
type mockReviewRepo struct {
	mu          sync.Mutex
	reviews     map[uuid.UUID]*Review
	knownUsers  map[uuid.UUID]bool
	forceErr    error
}

func newMockReviewRepo() *mockReviewRepo {
	return &mockReviewRepo{
		reviews:    make(map[uuid.UUID]*Review),
		knownUsers: make(map[uuid.UUID]bool),
	}
}

func (m *mockReviewRepo) CreateReview(ctx context.Context, review *Review) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.forceErr != nil {
		return m.forceErr
	}

	// Check unique constraint uq_contract_reviewer
	for _, r := range m.reviews {
		if r.ContractID == review.ContractID && r.ReviewerID == review.ReviewerID {
			return ErrDuplicateReview
		}
	}

	if review.ID == uuid.Nil {
		review.ID = uuid.New()
	}
	review.CreatedAt = time.Now().UTC()

	cp := *review
	m.reviews[review.ID] = &cp
	return nil
}

func (m *mockReviewRepo) GetReviewsByContractID(ctx context.Context, contractID uuid.UUID) ([]*Review, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.forceErr != nil {
		return nil, m.forceErr
	}

	list := make([]*Review, 0)
	for _, r := range m.reviews {
		if r.ContractID == contractID {
			cp := *r
			list = append(list, &cp)
		}
	}
	return list, nil
}

func (m *mockReviewRepo) GetUserReviewsWithSummary(ctx context.Context, userID uuid.UUID) (*UserReviewSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.forceErr != nil {
		return nil, m.forceErr
	}

	if !m.knownUsers[userID] {
		return nil, ErrUserNotFound
	}

	list := make([]*Review, 0)
	totalRating := 0
	for _, r := range m.reviews {
		if r.RevieweeID == userID {
			cp := *r
			list = append(list, &cp)
			totalRating += r.Rating
		}
	}

	count := len(list)
	avg := 0.0
	if count > 0 {
		avg = float64(totalRating) / float64(count)
		avg = math.Round(avg*100) / 100
	}

	return &UserReviewSummary{
		UserID:        userID,
		AverageRating: avg,
		ReviewCount:   count,
		Reviews:       list,
	}, nil
}

func (m *mockReviewRepo) HasUserReviewedContract(ctx context.Context, contractID, reviewerID uuid.UUID) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.forceErr != nil {
		return false, m.forceErr
	}

	for _, r := range m.reviews {
		if r.ContractID == contractID && r.ReviewerID == reviewerID {
			return true, nil
		}
	}
	return false, nil
}

// mockContractReader implements ContractReader for testing.
type mockContractReader struct {
	mu        sync.Mutex
	contracts map[uuid.UUID]*contract.ContractWithDetails
}

func newMockContractReader() *mockContractReader {
	return &mockContractReader{
		contracts: make(map[uuid.UUID]*contract.ContractWithDetails),
	}
}

func (m *mockContractReader) GetContractByID(ctx context.Context, id uuid.UUID) (*contract.ContractWithDetails, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.contracts[id]
	if !ok {
		return nil, contract.ErrContractNotFound
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
		t.Fatalf("failed to unmarshal JSON envelope: %v, raw: %s", err, string(body))
	}
	return res
}

// --- 1. Unit Tests: Model Validation ---

func TestReview_ModelValidation(t *testing.T) {
	tests := []struct {
		name      string
		rating    int
		comment   string
		expectErr bool
	}{
		{"rating_0_invalid", 0, "Too low", true},
		{"rating_minus_1_invalid", -1, "Negative", true},
		{"rating_1_valid_min", 1, "Poor", false},
		{"rating_2_valid", 2, "Fair", false},
		{"rating_3_valid", 3, "Average", false},
		{"rating_4_valid", 4, "Good", false},
		{"rating_5_valid_max", 5, "Excellent", false},
		{"rating_6_invalid", 6, "Too high", true},
		{"rating_100_invalid", 100, "Way too high", true},
		{"empty_comment_valid", 5, "", false},
		{"whitespace_comment_valid", 4, "   ", false},
		{"comment_within_5000_chars", 5, strings.Repeat("A", 5000), false},
		{"comment_exceeds_5000_chars", 5, strings.Repeat("B", 5001), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCreateReview(CreateReviewRequest{
				Rating:  tc.rating,
				Comment: tc.comment,
			})
			if tc.expectErr && err == nil {
				t.Fatalf("expected error for rating=%d comment_len=%d, got nil", tc.rating, len(tc.comment))
			}
			if !tc.expectErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// --- 2. Unit Tests: Completed Contract Check ---

func TestReview_CompletedContractCheck(t *testing.T) {
	studentID := uuid.New()
	employerID := uuid.New()

	contractStatuses := []struct {
		status        string
		expectCode    int
		expectErrCode string
	}{
		{contract.StatusDraft, http.StatusBadRequest, "CONTRACT_NOT_COMPLETED"},
		{contract.StatusActive, http.StatusBadRequest, "CONTRACT_NOT_COMPLETED"},
		{contract.StatusCancelled, http.StatusBadRequest, "CONTRACT_NOT_COMPLETED"},
		{contract.StatusCompleted, http.StatusCreated, ""},
	}

	for _, cs := range contractStatuses {
		t.Run("status_"+cs.status, func(t *testing.T) {
			contractID := uuid.New()

			contractReader := newMockContractReader()
			contractReader.contracts[contractID] = &contract.ContractWithDetails{
				Contract: contract.Contract{
					ID:         contractID,
					StudentID:  studentID,
					EmployerID: employerID,
					Status:     cs.status,
				},
			}

			repo := newMockReviewRepo()
			service := NewService(repo, contractReader)
			handler := NewHandler(service)

			body, _ := json.Marshal(CreateReviewRequest{
				Rating:  5,
				Comment: "Completed check review",
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+contractID.String()+"/reviews", bytes.NewReader(body))
			req = withAuth(req, &auth.UserClaims{
				UserID: studentID.String(),
				Email:  "student@university.edu",
				Roles:  []string{"student"},
			})

			w := httptest.NewRecorder()
			handler.Routes(nil).ServeHTTP(w, req)

			if w.Code != cs.expectCode {
				t.Fatalf("status %s: expected HTTP %d, got %d. Body: %s", cs.status, cs.expectCode, w.Code, w.Body.String())
			}

			env := parseEnvelope(t, w.Body.Bytes())
			if cs.expectErrCode != "" {
				if env.Success {
					t.Fatalf("expected success=false for status %s", cs.status)
				}
				if env.Error == nil || env.Error.Code != cs.expectErrCode {
					t.Fatalf("expected error code %s, got %+v", cs.expectErrCode, env.Error)
				}
			} else {
				if !env.Success {
					t.Fatalf("expected success=true for status %s, got %+v", cs.status, env.Error)
				}
			}
		})
	}
}

// --- 3. Unit Tests: Participant Authorization & Reviewee Assignment ---

func TestReview_ParticipantAuthorization(t *testing.T) {
	studentID := uuid.New()
	employerID := uuid.New()
	unrelatedID := uuid.New()
	adminID := uuid.New()

	contractID := uuid.New()
	contractReader := newMockContractReader()
	contractReader.contracts[contractID] = &contract.ContractWithDetails{
		Contract: contract.Contract{
			ID:         contractID,
			StudentID:  studentID,
			EmployerID: employerID,
			Status:     contract.StatusCompleted,
		},
	}

	t.Run("student_reviews_employer", func(t *testing.T) {
		repo := newMockReviewRepo()
		service := NewService(repo, contractReader)
		handler := NewHandler(service)

		body, _ := json.Marshal(CreateReviewRequest{Rating: 5, Comment: "Great employer!"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+contractID.String()+"/reviews", bytes.NewReader(body))
		req = withAuth(req, &auth.UserClaims{
			UserID: studentID.String(),
			Email:  "student@university.edu",
			Roles:  []string{"student"},
		})
		w := httptest.NewRecorder()
		handler.Routes(nil).ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
		}

		env := parseEnvelope(t, w.Body.Bytes())
		var created Review
		if err := json.Unmarshal(env.Data, &created); err != nil {
			t.Fatalf("failed to unmarshal created review: %v", err)
		}

		if created.ReviewerID != studentID {
			t.Errorf("expected ReviewerID=%s, got %s", studentID, created.ReviewerID)
		}
		if created.RevieweeID != employerID {
			t.Errorf("expected RevieweeID=%s (employer), got %s", employerID, created.RevieweeID)
		}
	})

	t.Run("employer_reviews_student", func(t *testing.T) {
		repo := newMockReviewRepo()
		service := NewService(repo, contractReader)
		handler := NewHandler(service)

		body, _ := json.Marshal(CreateReviewRequest{Rating: 4, Comment: "Strong student performance!"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+contractID.String()+"/reviews", bytes.NewReader(body))
		req = withAuth(req, &auth.UserClaims{
			UserID: employerID.String(),
			Email:  "employer@company.com",
			Roles:  []string{"employer"},
		})
		w := httptest.NewRecorder()
		handler.Routes(nil).ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
		}

		env := parseEnvelope(t, w.Body.Bytes())
		var created Review
		if err := json.Unmarshal(env.Data, &created); err != nil {
			t.Fatalf("failed to unmarshal created review: %v", err)
		}

		if created.ReviewerID != employerID {
			t.Errorf("expected ReviewerID=%s, got %s", employerID, created.ReviewerID)
		}
		if created.RevieweeID != studentID {
			t.Errorf("expected RevieweeID=%s (student), got %s", studentID, created.RevieweeID)
		}
	})

	t.Run("unrelated_user_rejected", func(t *testing.T) {
		repo := newMockReviewRepo()
		service := NewService(repo, contractReader)
		handler := NewHandler(service)

		body, _ := json.Marshal(CreateReviewRequest{Rating: 5, Comment: "I was not involved"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+contractID.String()+"/reviews", bytes.NewReader(body))
		req = withAuth(req, &auth.UserClaims{
			UserID: unrelatedID.String(),
			Email:  "outsider@example.com",
			Roles:  []string{"student"},
		})
		w := httptest.NewRecorder()
		handler.Routes(nil).ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d: %s", w.Code, w.Body.String())
		}
		env := parseEnvelope(t, w.Body.Bytes())
		if env.Error == nil || env.Error.Code != "FORBIDDEN" {
			t.Fatalf("expected error code FORBIDDEN, got %+v", env.Error)
		}
	})

	t.Run("admin_non_participant_rejected", func(t *testing.T) {
		repo := newMockReviewRepo()
		service := NewService(repo, contractReader)
		handler := NewHandler(service)

		body, _ := json.Marshal(CreateReviewRequest{Rating: 5, Comment: "Admin attempting to review"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+contractID.String()+"/reviews", bytes.NewReader(body))
		req = withAuth(req, &auth.UserClaims{
			UserID: adminID.String(),
			Email:  "admin@platform.com",
			Roles:  []string{"admin"},
		})
		w := httptest.NewRecorder()
		handler.Routes(nil).ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for non-participant admin, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("unauthenticated_rejected", func(t *testing.T) {
		repo := newMockReviewRepo()
		service := NewService(repo, contractReader)
		handler := NewHandler(service)

		body, _ := json.Marshal(CreateReviewRequest{Rating: 5, Comment: "No auth"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+contractID.String()+"/reviews", bytes.NewReader(body))
		// No auth context
		w := httptest.NewRecorder()
		handler.Routes(nil).ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// --- 4. Unit Tests: Duplicate Review Prevention (409 Conflict) ---

func TestReview_DuplicateSubmission(t *testing.T) {
	studentID := uuid.New()
	employerID := uuid.New()
	contractID := uuid.New()

	contractReader := newMockContractReader()
	contractReader.contracts[contractID] = &contract.ContractWithDetails{
		Contract: contract.Contract{
			ID:         contractID,
			StudentID:  studentID,
			EmployerID: employerID,
			Status:     contract.StatusCompleted,
		},
	}

	repo := newMockReviewRepo()
	service := NewService(repo, contractReader)
	handler := NewHandler(service)

	// 1. Student submits first review -> 201 Created
	body1, _ := json.Marshal(CreateReviewRequest{Rating: 5, Comment: "First review from student"})
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+contractID.String()+"/reviews", bytes.NewReader(body1))
	req1 = withAuth(req1, &auth.UserClaims{
		UserID: studentID.String(),
		Email:  "student@university.edu",
		Roles:  []string{"student"},
	})
	w1 := httptest.NewRecorder()
	handler.Routes(nil).ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("first review expected 201 Created, got %d: %s", w1.Code, w1.Body.String())
	}

	// 2. Student attempts second review on same contract -> 409 Conflict
	body2, _ := json.Marshal(CreateReviewRequest{Rating: 4, Comment: "Second attempt by student"})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+contractID.String()+"/reviews", bytes.NewReader(body2))
	req2 = withAuth(req2, &auth.UserClaims{
		UserID: studentID.String(),
		Email:  "student@university.edu",
		Roles:  []string{"student"},
	})
	w2 := httptest.NewRecorder()
	handler.Routes(nil).ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for duplicate review, got %d: %s", w2.Code, w2.Body.String())
	}
	env2 := parseEnvelope(t, w2.Body.Bytes())
	if env2.Error == nil || env2.Error.Code != "DUPLICATE_REVIEW" {
		t.Fatalf("expected DUPLICATE_REVIEW code, got %+v", env2.Error)
	}

	// 3. Employer submits their first review on the same contract -> 201 Created
	body3, _ := json.Marshal(CreateReviewRequest{Rating: 4, Comment: "First review from employer"})
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+contractID.String()+"/reviews", bytes.NewReader(body3))
	req3 = withAuth(req3, &auth.UserClaims{
		UserID: employerID.String(),
		Email:  "employer@company.com",
		Roles:  []string{"employer"},
	})
	w3 := httptest.NewRecorder()
	handler.Routes(nil).ServeHTTP(w3, req3)

	if w3.Code != http.StatusCreated {
		t.Fatalf("employer first review expected 201 Created, got %d: %s", w3.Code, w3.Body.String())
	}

	// 4. Employer attempts second review on same contract -> 409 Conflict
	body4, _ := json.Marshal(CreateReviewRequest{Rating: 5, Comment: "Second attempt by employer"})
	req4 := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+contractID.String()+"/reviews", bytes.NewReader(body4))
	req4 = withAuth(req4, &auth.UserClaims{
		UserID: employerID.String(),
		Email:  "employer@company.com",
		Roles:  []string{"employer"},
	})
	w4 := httptest.NewRecorder()
	handler.Routes(nil).ServeHTTP(w4, req4)

	if w4.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for duplicate review from employer, got %d: %s", w4.Code, w4.Body.String())
	}
	env4 := parseEnvelope(t, w4.Body.Bytes())
	if env4.Error == nil || env4.Error.Code != "DUPLICATE_REVIEW" {
		t.Fatalf("expected DUPLICATE_REVIEW code, got %+v", env4.Error)
	}
}

// --- 5. Unit Tests: Contract Reviews Listing ---

func TestReview_ListContractReviews(t *testing.T) {
	studentID := uuid.New()
	employerID := uuid.New()
	contractID := uuid.New()

	contractReader := newMockContractReader()
	contractReader.contracts[contractID] = &contract.ContractWithDetails{
		Contract: contract.Contract{
			ID:         contractID,
			StudentID:  studentID,
			EmployerID: employerID,
			Status:     contract.StatusCompleted,
		},
	}

	repo := newMockReviewRepo()
	service := NewService(repo, contractReader)
	handler := NewHandler(service)

	t.Run("non_existent_contract", func(t *testing.T) {
		missingID := uuid.New()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+missingID.String()+"/reviews", nil)
		w := httptest.NewRecorder()
		handler.Routes(nil).ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d: %s", w.Code, w.Body.String())
		}
		env := parseEnvelope(t, w.Body.Bytes())
		if env.Error == nil || env.Error.Code != "CONTRACT_NOT_FOUND" {
			t.Fatalf("expected error code CONTRACT_NOT_FOUND, got %+v", env.Error)
		}
	})

	t.Run("empty_contract_reviews", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+contractID.String()+"/reviews", nil)
		w := httptest.NewRecorder()
		handler.Routes(nil).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}
		env := parseEnvelope(t, w.Body.Bytes())
		if !env.Success {
			t.Fatalf("expected success=true, got %+v", env.Error)
		}
		var list []Review
		if err := json.Unmarshal(env.Data, &list); err != nil {
			t.Fatalf("failed to unmarshal list: %v", err)
		}
		if len(list) != 0 {
			t.Fatalf("expected 0 reviews, got %d", len(list))
		}
	})

	t.Run("contract_with_reviews", func(t *testing.T) {
		_ = repo.CreateReview(context.Background(), &Review{
			ContractID: contractID,
			ReviewerID: studentID,
			RevieweeID: employerID,
			Rating:     5,
			Comment:    "Student's rating",
		})
		_ = repo.CreateReview(context.Background(), &Review{
			ContractID: contractID,
			ReviewerID: employerID,
			RevieweeID: studentID,
			Rating:     4,
			Comment:    "Employer's rating",
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+contractID.String()+"/reviews", nil)
		w := httptest.NewRecorder()
		handler.Routes(nil).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}
		env := parseEnvelope(t, w.Body.Bytes())
		var list []Review
		if err := json.Unmarshal(env.Data, &list); err != nil {
			t.Fatalf("failed to unmarshal list: %v", err)
		}
		if len(list) != 2 {
			t.Fatalf("expected 2 reviews, got %d", len(list))
		}
	})
}

// --- 6. Unit Tests: User Aggregated Review Summary ---

func TestReview_GetUserReviewsWithSummary(t *testing.T) {
	repo := newMockReviewRepo()
	service := NewService(repo, nil)
	handler := NewHandler(service)

	targetUserID := uuid.New()
	repo.knownUsers[targetUserID] = true

	t.Run("non_existent_user_returns_404", func(t *testing.T) {
		unknownID := uuid.New()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+unknownID.String()+"/reviews", nil)
		w := httptest.NewRecorder()
		handler.Routes(nil).ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d: %s", w.Code, w.Body.String())
		}
		env := parseEnvelope(t, w.Body.Bytes())
		if env.Error == nil || env.Error.Code != "USER_NOT_FOUND" {
			t.Fatalf("expected USER_NOT_FOUND error code, got %+v", env.Error)
		}
	})

	t.Run("user_with_zero_reviews", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+targetUserID.String()+"/reviews", nil)
		w := httptest.NewRecorder()
		handler.Routes(nil).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}
		env := parseEnvelope(t, w.Body.Bytes())
		var summary UserReviewSummary
		if err := json.Unmarshal(env.Data, &summary); err != nil {
			t.Fatalf("failed to unmarshal summary: %v", err)
		}

		if summary.UserID != targetUserID {
			t.Errorf("expected UserID=%s, got %s", targetUserID, summary.UserID)
		}
		if summary.ReviewCount != 0 {
			t.Errorf("expected ReviewCount=0, got %d", summary.ReviewCount)
		}
		if summary.AverageRating != 0.0 {
			t.Errorf("expected AverageRating=0.0, got %f", summary.AverageRating)
		}
		if summary.Reviews == nil || len(summary.Reviews) != 0 {
			t.Errorf("expected empty non-nil reviews slice, got %+v", summary.Reviews)
		}
	})

	t.Run("user_with_aggregated_reviews", func(t *testing.T) {
		cID1 := uuid.New()
		cID2 := uuid.New()
		cID3 := uuid.New()

		// Add 3 reviews: ratings 5, 4, 4 -> average = 4.33
		_ = repo.CreateReview(context.Background(), &Review{
			ContractID: cID1,
			ReviewerID: uuid.New(),
			RevieweeID: targetUserID,
			Rating:     5,
			Comment:    "Outstanding",
		})
		_ = repo.CreateReview(context.Background(), &Review{
			ContractID: cID2,
			ReviewerID: uuid.New(),
			RevieweeID: targetUserID,
			Rating:     4,
			Comment:    "Very good",
		})
		_ = repo.CreateReview(context.Background(), &Review{
			ContractID: cID3,
			ReviewerID: uuid.New(),
			RevieweeID: targetUserID,
			Rating:     4,
			Comment:    "Consistent quality",
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+targetUserID.String()+"/reviews", nil)
		w := httptest.NewRecorder()
		handler.Routes(nil).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}
		env := parseEnvelope(t, w.Body.Bytes())
		var summary UserReviewSummary
		if err := json.Unmarshal(env.Data, &summary); err != nil {
			t.Fatalf("failed to unmarshal summary: %v", err)
		}

		if summary.ReviewCount != 3 {
			t.Fatalf("expected ReviewCount=3, got %d", summary.ReviewCount)
		}
		if summary.AverageRating != 4.33 {
			t.Fatalf("expected AverageRating=4.33, got %f", summary.AverageRating)
		}
		if len(summary.Reviews) != 3 {
			t.Fatalf("expected 3 review items, got %d", len(summary.Reviews))
		}
	})
}

// --- 7. Unit Tests: Invalid Requests & Payloads ---

func TestReview_InvalidRequests(t *testing.T) {
	studentID := uuid.New()
	employerID := uuid.New()
	contractID := uuid.New()

	contractReader := newMockContractReader()
	contractReader.contracts[contractID] = &contract.ContractWithDetails{
		Contract: contract.Contract{
			ID:         contractID,
			StudentID:  studentID,
			EmployerID: employerID,
			Status:     contract.StatusCompleted,
		},
	}

	repo := newMockReviewRepo()
	service := NewService(repo, contractReader)
	handler := NewHandler(service)

	t.Run("empty_body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+contractID.String()+"/reviews", bytes.NewReader([]byte{}))
		req = withAuth(req, &auth.UserClaims{
			UserID: studentID.String(),
			Email:  "student@university.edu",
			Roles:  []string{"student"},
		})
		w := httptest.NewRecorder()
		handler.Routes(nil).ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", w.Code)
		}
		env := parseEnvelope(t, w.Body.Bytes())
		if env.Error == nil || env.Error.Code != "BAD_REQUEST" {
			t.Fatalf("expected BAD_REQUEST error code, got %+v", env.Error)
		}
	})

	t.Run("malformed_json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+contractID.String()+"/reviews", strings.NewReader("{rating: 5"))
		req = withAuth(req, &auth.UserClaims{
			UserID: studentID.String(),
			Email:  "student@university.edu",
			Roles:  []string{"student"},
		})
		w := httptest.NewRecorder()
		handler.Routes(nil).ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", w.Code)
		}
		env := parseEnvelope(t, w.Body.Bytes())
		if env.Error == nil || env.Error.Code != "BAD_REQUEST" {
			t.Fatalf("expected BAD_REQUEST error code, got %+v", env.Error)
		}
	})

	t.Run("rating_out_of_bounds", func(t *testing.T) {
		body, _ := json.Marshal(CreateReviewRequest{Rating: 6, Comment: "Out of bounds"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+contractID.String()+"/reviews", bytes.NewReader(body))
		req = withAuth(req, &auth.UserClaims{
			UserID: studentID.String(),
			Email:  "student@university.edu",
			Roles:  []string{"student"},
		})
		w := httptest.NewRecorder()
		handler.Routes(nil).ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", w.Code)
		}
		env := parseEnvelope(t, w.Body.Bytes())
		if env.Error == nil || env.Error.Code != "INVALID_RATING" {
			t.Fatalf("expected INVALID_RATING error code, got %+v", env.Error)
		}
	})

	t.Run("invalid_contract_uuid", func(t *testing.T) {
		body, _ := json.Marshal(CreateReviewRequest{Rating: 5, Comment: "Valid review"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/invalid-uuid/reviews", bytes.NewReader(body))
		req = withAuth(req, &auth.UserClaims{
			UserID: studentID.String(),
			Email:  "student@university.edu",
			Roles:  []string{"student"},
		})
		w := httptest.NewRecorder()
		handler.Routes(nil).ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("invalid_user_uuid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/not-a-uuid/reviews", nil)
		w := httptest.NewRecorder()
		handler.Routes(nil).ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("contract_not_found_on_post", func(t *testing.T) {
		missingContractID := uuid.New()
		body, _ := json.Marshal(CreateReviewRequest{Rating: 5, Comment: "Contract doesn't exist"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+missingContractID.String()+"/reviews", bytes.NewReader(body))
		req = withAuth(req, &auth.UserClaims{
			UserID: studentID.String(),
			Email:  "student@university.edu",
			Roles:  []string{"student"},
		})
		w := httptest.NewRecorder()
		handler.Routes(nil).ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", w.Code)
		}
		env := parseEnvelope(t, w.Body.Bytes())
		if env.Error == nil || env.Error.Code != "CONTRACT_NOT_FOUND" {
			t.Fatalf("expected CONTRACT_NOT_FOUND error code, got %+v", env.Error)
		}
	})
}

// --- 8. Integration Tests: Live Auth Middleware Integration ---

func TestReview_LiveAuthMiddlewareIntegration(t *testing.T) {
	studentID := uuid.New()
	employerID := uuid.New()
	contractID := uuid.New()

	contractReader := newMockContractReader()
	contractReader.contracts[contractID] = &contract.ContractWithDetails{
		Contract: contract.Contract{
			ID:         contractID,
			StudentID:  studentID,
			EmployerID: employerID,
			Status:     contract.StatusCompleted,
		},
	}

	repo := newMockReviewRepo()
	service := NewService(repo, contractReader)
	handler := NewHandler(service)

	validToken := "valid-token-for-student"
	claims := &auth.UserClaims{
		UserID:        studentID.String(),
		Email:         "student@university.edu",
		EmailVerified: true,
		Roles:         []string{"student"},
	}

	validator := &mockValidator{
		claimsMap: map[string]*auth.UserClaims{
			validToken: claims,
		},
	}
	authMw := middleware.AuthMiddleware(validator)
	router := handler.Routes(authMw)

	t.Run("request_with_valid_bearer_token", func(t *testing.T) {
		body, _ := json.Marshal(CreateReviewRequest{
			Rating:  5,
			Comment: "Verified auth review submission",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+contractID.String()+"/reviews", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+validToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
		}
		env := parseEnvelope(t, w.Body.Bytes())
		if !env.Success {
			t.Fatalf("expected success=true, got %+v", env.Error)
		}
	})

	t.Run("request_without_auth_header", func(t *testing.T) {
		contractID2 := uuid.New()
		contractReader.contracts[contractID2] = &contract.ContractWithDetails{
			Contract: contract.Contract{
				ID:         contractID2,
				StudentID:  studentID,
				EmployerID: employerID,
				Status:     contract.StatusCompleted,
			},
		}

		body, _ := json.Marshal(CreateReviewRequest{Rating: 5, Comment: "No token"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+contractID2.String()+"/reviews", bytes.NewReader(body))

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("request_with_invalid_token", func(t *testing.T) {
		contractID3 := uuid.New()
		contractReader.contracts[contractID3] = &contract.ContractWithDetails{
			Contract: contract.Contract{
				ID:         contractID3,
				StudentID:  studentID,
				EmployerID: employerID,
				Status:     contract.StatusCompleted,
			},
		}

		body, _ := json.Marshal(CreateReviewRequest{Rating: 5, Comment: "Bad token"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+contractID3.String()+"/reviews", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer invalid-garbage-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d: %s", w.Code, w.Body.String())
		}
	})
}
