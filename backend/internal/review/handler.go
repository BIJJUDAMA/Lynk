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

	// Protected routes (contract participants)
	if authMiddleware != nil {
		r.Group(func(pr chi.Router) {
			pr.Use(authMiddleware)
			pr.Post("/contracts/{id}/reviews", h.CreateReview)
			pr.Post("/api/v1/contracts/{id}/reviews", h.CreateReview)
		})
	} else {
		r.Post("/contracts/{id}/reviews", h.CreateReview)
		r.Post("/api/v1/contracts/{id}/reviews", h.CreateReview)
	}

	return r
}

// CreateReview handles POST /api/v1/contracts/{id}/reviews (Participant submits rating & review).
func (h *Handler) CreateReview(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil || claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
		return
	}

	contractID, err := extractContractID(r)
	if err != nil || contractID == uuid.Nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid contract ID in URL")
		return
	}

	var req CreateReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Request body cannot be empty")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON request body")
		return
	}

	review, err := h.service.CreateReview(r.Context(), claims, contractID, req)
	if err != nil {
		if errors.Is(err, ErrContractNotFound) {
			writeJSONError(w, http.StatusNotFound, "CONTRACT_NOT_FOUND", "Contract not found")
			return
		}
		if errors.Is(err, ErrContractNotCompleted) {
			writeJSONError(w, http.StatusBadRequest, "CONTRACT_NOT_COMPLETED", err.Error())
			return
		}
		if errors.Is(err, ErrNotParticipant) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "Only contract participants may submit reviews")
			return
		}
		if errors.Is(err, ErrForbidden) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		if errors.Is(err, ErrDuplicateReview) {
			writeJSONError(w, http.StatusConflict, "DUPLICATE_REVIEW", "Review already submitted for this contract")
			return
		}
		if errors.Is(err, ErrInvalidRating) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_RATING", err.Error())
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to submit review")
		return
	}

	writeJSONSuccess(w, http.StatusCreated, review)
}

// GetContractReviews handles GET /api/v1/contracts/{id}/reviews (List reviews on a contract).
func (h *Handler) GetContractReviews(w http.ResponseWriter, r *http.Request) {
	contractID, err := extractContractID(r)
	if err != nil || contractID == uuid.Nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid contract ID in URL")
		return
	}

	reviews, err := h.service.GetReviewsByContractID(r.Context(), contractID)
	if err != nil {
		if errors.Is(err, ErrContractNotFound) {
			writeJSONError(w, http.StatusNotFound, "CONTRACT_NOT_FOUND", "Contract not found")
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve contract reviews")
		return
	}

	writeJSONSuccess(w, http.StatusOK, reviews)
}

// GetUserReviews handles GET /api/v1/users/{id}/reviews (Public: user review summary + received reviews).
func (h *Handler) GetUserReviews(w http.ResponseWriter, r *http.Request) {
	userID, err := extractUserID(r)
	if err != nil || userID == uuid.Nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid user ID in URL")
		return
	}

	summary, err := h.service.GetUserReviewsWithSummary(r.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			writeJSONError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve user review summary")
		return
	}

	writeJSONSuccess(w, http.StatusOK, summary)
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

func extractUserID(r *http.Request) (uuid.UUID, error) {
	if val := chi.URLParam(r, "id"); val != "" {
		if id, err := uuid.Parse(val); err == nil {
			return id, nil
		}
	}
	if val := chi.URLParam(r, "userID"); val != "" {
		if id, err := uuid.Parse(val); err == nil {
			return id, nil
		}
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for i, p := range parts {
		if (p == "users" || p == "user") && i+1 < len(parts) && parts[i+1] != "reviews" {
			return uuid.Parse(parts[i+1])
		}
	}

	return uuid.Nil, errors.New("invalid user id in url")
}

func writeJSONSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    data,
		"error":   nil,
	})
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"data":    nil,
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
