package review

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/httputil"
	"github.com/lynk/backend/internal/httpx"
)

// Handler handles HTTP requests for reviews and rating aggregates.
type Handler struct {
	service *Service
}

// NewHandler creates a new review HTTP handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Routes constructs a chi.Router mounting all contract review and public user review endpoints.
func (h *Handler) Routes(authMiddleware func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()

	// Public routes
	r.Get("/contracts/{id}/reviews", h.GetContractReviews)
	r.Get("/api/v1/contracts/{id}/reviews", h.GetContractReviews)

	r.Get("/users/{id}/reviews", h.GetUserReviews)
	r.Get("/api/v1/users/{id}/reviews", h.GetUserReviews)

	authMiddleware = httpx.DefaultAuthMiddleware(authMiddleware)

	// Protected routes (contract participants)
	r.Group(func(pr chi.Router) {
		pr.Use(authMiddleware)
		pr.Post("/contracts/{id}/reviews", h.CreateReview)
		pr.Post("/api/v1/contracts/{id}/reviews", h.CreateReview)
	})

	return r
}

// CreateReview handles POST /api/v1/contracts/{id}/reviews (Participant submits rating & review).
func (h *Handler) CreateReview(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil || claims == nil {
		httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials", nil)
		return
	}

	contractID, err := extractContractID(r)
	if err != nil || contractID == uuid.Nil {
		httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_INPUT", "Invalid contract ID in URL", err)
		return
	}

	var req CreateReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			httputil.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Request body cannot be empty", err)
			return
		}
		httputil.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON request body", err)
		return
	}

	review, err := h.service.CreateReview(r.Context(), claims, contractID, req)
	if err != nil {
		if errors.Is(err, auth.ErrEmailNotVerified) {
			httputil.WriteError(w, r, http.StatusForbidden, "EMAIL_NOT_VERIFIED", auth.CampusVerificationPendingMsg, nil)
			return
		}
		if errors.Is(err, ErrContractNotFound) {
			httputil.WriteError(w, r, http.StatusNotFound, "CONTRACT_NOT_FOUND", "Contract not found", nil)
			return
		}
		if errors.Is(err, ErrContractNotCompleted) {
			httputil.WriteError(w, r, http.StatusBadRequest, "CONTRACT_NOT_COMPLETED", err.Error(), nil)
			return
		}
		if errors.Is(err, ErrNotParticipant) {
			httputil.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Only contract participants may submit reviews", nil)
			return
		}
		if errors.Is(err, ErrForbidden) {
			httputil.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		if errors.Is(err, ErrDuplicateReview) {
			httputil.WriteError(w, r, http.StatusConflict, "DUPLICATE_REVIEW", "Review already submitted for this contract", nil)
			return
		}
		if errors.Is(err, ErrInvalidRating) {
			httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_RATING", err.Error(), err)
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error(), err)
			return
		}
		httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to submit review", err)
		return
	}

	httputil.WriteSuccess(w, http.StatusCreated, review)
}

// GetContractReviews handles GET /api/v1/contracts/{id}/reviews (List reviews on a contract).
func (h *Handler) GetContractReviews(w http.ResponseWriter, r *http.Request) {
	contractID, err := extractContractID(r)
	if err != nil || contractID == uuid.Nil {
		httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_INPUT", "Invalid contract ID in URL", err)
		return
	}

	reviews, err := h.service.GetReviewsByContractID(r.Context(), contractID)
	if err != nil {
		if errors.Is(err, ErrContractNotFound) {
			httputil.WriteError(w, r, http.StatusNotFound, "CONTRACT_NOT_FOUND", "Contract not found", nil)
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error(), err)
			return
		}
		httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve contract reviews", err)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, reviews)
}

// GetUserReviews handles GET /api/v1/users/{id}/reviews (Public: user review summary + received reviews).
func (h *Handler) GetUserReviews(w http.ResponseWriter, r *http.Request) {
	userID, err := extractUserID(r)
	if err != nil || strings.TrimSpace(userID) == "" {
		httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_INPUT", "Invalid user ID in URL", err)
		return
	}

	summary, err := h.service.GetUserReviewsWithSummary(r.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			httputil.WriteError(w, r, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error(), err)
			return
		}
		httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve user review summary", err)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, summary)
}

func extractContractID(r *http.Request) (uuid.UUID, error) {
	if val := chi.URLParam(r, "id"); val != "" {
		if id, err := uuid.Parse(val); err == nil {
			return id, nil
		}
	}
	if val := chi.URLParam(r, "contractID"); val != "" {
		if id, err := uuid.Parse(val); err == nil {
			return id, nil
		}
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for i, p := range parts {
		if (p == "contracts" || p == "contract") && i+1 < len(parts) && parts[i+1] != "reviews" {
			return uuid.Parse(parts[i+1])
		}
	}

	return uuid.Nil, errors.New("invalid contract id in url")
}

func extractUserID(r *http.Request) (string, error) {
	if val := strings.TrimSpace(chi.URLParam(r, "id")); val != "" {
		return val, nil
	}
	if val := strings.TrimSpace(chi.URLParam(r, "userID")); val != "" {
		return val, nil
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for i, p := range parts {
		if (p == "users" || p == "user") && i+1 < len(parts) && parts[i+1] != "reviews" {
			val := strings.TrimSpace(parts[i+1])
			if val != "" {
				return val, nil
			}
		}
	}

	return "", errors.New("invalid user id in url")
}
