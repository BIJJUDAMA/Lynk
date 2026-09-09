package review

import (
	"bytes"
	"context"
	"encoding/json"
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
)

type mockReviewRepo struct {
	mu         sync.Mutex
	reviews    map[uuid.UUID]*Review
	knownUsers map[string]bool
	forceErr   error
}

func newMockReviewRepo() *mockReviewRepo {
	return &mockReviewRepo{
		reviews:    make(map[uuid.UUID]*Review),
		knownUsers: make(map[string]bool),
	}
}

func (m *mockReviewRepo) CreateReview(ctx context.Context, review *Review) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.forceErr != nil {
		return m.forceErr
	}

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

func (m *mockReviewRepo) GetUserReviewsWithSummary(ctx context.Context, userID string) (*UserReviewSummary, error) {
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

func (m *mockReviewRepo) HasUserReviewedContract(ctx context.Context, contractID uuid.UUID, reviewerID string) (bool, error) {
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

func TestReview_CompletedContractCheck(t *testing.T) {
	freelancerID := uuid.New().String()
	clientID := uuid.New().String()

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
					ID:           contractID,
					FreelancerID: freelancerID,
					ClientID:     clientID,
					Status:       cs.status,
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
				UserID: freelancerID,
				Email:  "student@university.edu",
				Roles:  []string{"member"},
			})

			w := httptest.NewRecorder()
			handler.Routes(nil).ServeHTTP(w, req)

			if w.Code != cs.expectCode {
				t.Fatalf("status %s: expected HTTP %d, got %d. Body: %s", cs.status, cs.expectCode, w.Code, w.Body.String())
			}
		})
	}
}

func TestReview_ParticipantAuthorization(t *testing.T) {
	freelancerID := uuid.New().String()
	clientID := uuid.New().String()
	unrelatedID := uuid.New().String()

	contractID := uuid.New()
	contractReader := newMockContractReader()
	contractReader.contracts[contractID] = &contract.ContractWithDetails{
		Contract: contract.Contract{
			ID:           contractID,
			FreelancerID: freelancerID,
			ClientID:     clientID,
			Status:       contract.StatusCompleted,
		},
	}

	t.Run("freelancer_reviews_client", func(t *testing.T) {
		repo := newMockReviewRepo()
		service := NewService(repo, contractReader)
		handler := NewHandler(service)

		body, _ := json.Marshal(CreateReviewRequest{Rating: 5, Comment: "Great client!"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+contractID.String()+"/reviews", bytes.NewReader(body))
		req = withAuth(req, &auth.UserClaims{
			UserID: freelancerID,
			Email:  "student@university.edu",
			Roles:  []string{"member"},
		})
		w := httptest.NewRecorder()
		handler.Routes(nil).ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
		}

		env := parseEnvelope(t, w.Body.Bytes())
		var created Review
		_ = json.Unmarshal(env.Data, &created)

		if created.ReviewerID != freelancerID {
			t.Errorf("expected ReviewerID=%s, got %s", freelancerID, created.ReviewerID)
		}
		if created.RevieweeID != clientID {
			t.Errorf("expected RevieweeID=%s (client), got %s", clientID, created.RevieweeID)
		}
	})

	t.Run("client_reviews_freelancer", func(t *testing.T) {
		repo := newMockReviewRepo()
		service := NewService(repo, contractReader)
		handler := NewHandler(service)

		body, _ := json.Marshal(CreateReviewRequest{Rating: 4, Comment: "Strong freelancer performance!"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+contractID.String()+"/reviews", bytes.NewReader(body))
		req = withAuth(req, &auth.UserClaims{
			UserID: clientID,
			Email:  "client@company.com",
			Roles:  []string{"member"},
		})
		w := httptest.NewRecorder()
		handler.Routes(nil).ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
		}

		env := parseEnvelope(t, w.Body.Bytes())
		var created Review
		_ = json.Unmarshal(env.Data, &created)

		if created.ReviewerID != clientID {
			t.Errorf("expected ReviewerID=%s, got %s", clientID, created.ReviewerID)
		}
		if created.RevieweeID != freelancerID {
			t.Errorf("expected RevieweeID=%s (freelancer), got %s", freelancerID, created.RevieweeID)
		}
	})

	t.Run("unrelated_user_rejected", func(t *testing.T) {
		repo := newMockReviewRepo()
		service := NewService(repo, contractReader)
		handler := NewHandler(service)

		body, _ := json.Marshal(CreateReviewRequest{Rating: 5, Comment: "I was not involved"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+contractID.String()+"/reviews", bytes.NewReader(body))
		req = withAuth(req, &auth.UserClaims{
			UserID: unrelatedID,
			Email:  "outsider@example.com",
			Roles:  []string{"member"},
		})
		w := httptest.NewRecorder()
		handler.Routes(nil).ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestReview_DuplicateSubmission(t *testing.T) {
	freelancerID := uuid.New().String()
	clientID := uuid.New().String()
	contractID := uuid.New()

	contractReader := newMockContractReader()
	contractReader.contracts[contractID] = &contract.ContractWithDetails{
		Contract: contract.Contract{
			ID:           contractID,
			FreelancerID: freelancerID,
			ClientID:     clientID,
			Status:       contract.StatusCompleted,
		},
	}

	repo := newMockReviewRepo()
	service := NewService(repo, contractReader)
	handler := NewHandler(service)

	body1, _ := json.Marshal(CreateReviewRequest{Rating: 5, Comment: "First review from freelancer"})
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+contractID.String()+"/reviews", bytes.NewReader(body1))
	req1 = withAuth(req1, &auth.UserClaims{
		UserID: freelancerID,
		Email:  "student@university.edu",
		Roles:  []string{"member"},
	})
	w1 := httptest.NewRecorder()
	handler.Routes(nil).ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("first review expected 201 Created, got %d", w1.Code)
	}

	// Attempt duplicate
	body2, _ := json.Marshal(CreateReviewRequest{Rating: 4, Comment: "Second attempt"})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+contractID.String()+"/reviews", bytes.NewReader(body2))
	req2 = withAuth(req2, &auth.UserClaims{
		UserID: freelancerID,
		Email:  "student@university.edu",
		Roles:  []string{"member"},
	})
	w2 := httptest.NewRecorder()
	handler.Routes(nil).ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for duplicate review, got %d", w2.Code)
	}
}

func TestReview_GetUserReviewsWithSummary(t *testing.T) {
	repo := newMockReviewRepo()
	service := NewService(repo, nil)
	handler := NewHandler(service)

	targetUserID := uuid.New().String()
	repo.knownUsers[targetUserID] = true

	cID1 := uuid.New()
	cID2 := uuid.New()

	_ = repo.CreateReview(context.Background(), &Review{
		ContractID: cID1,
		ReviewerID: uuid.New().String(),
		RevieweeID: targetUserID,
		Rating:     5,
		Comment:    "Outstanding",
	})
	_ = repo.CreateReview(context.Background(), &Review{
		ContractID: cID2,
		ReviewerID: uuid.New().String(),
		RevieweeID: targetUserID,
		Rating:     4,
		Comment:    "Very good",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+targetUserID+"/reviews", nil)
	w := httptest.NewRecorder()
	handler.Routes(nil).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}
	env := parseEnvelope(t, w.Body.Bytes())
	var summary UserReviewSummary
	_ = json.Unmarshal(env.Data, &summary)

	if summary.ReviewCount != 2 {
		t.Fatalf("expected ReviewCount=2, got %d", summary.ReviewCount)
	}
	if summary.AverageRating != 4.5 {
		t.Fatalf("expected AverageRating=4.5, got %f", summary.AverageRating)
	}
}
