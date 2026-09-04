package user

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/storage"
)

// Handler handles user authentication sync and profile management endpoints.
type Handler struct {
	service  *Service
	repo     UserRepository
	s3Client storage.Client
}

// NewHandler creates a new user Handler with the provided service, repository, and optional storage client.
func NewHandler(service *Service, repo UserRepository, s3Client ...storage.Client) *Handler {
	if repo == nil && service != nil {
		repo = service.repo
	}
	var s3 storage.Client
	if len(s3Client) > 0 {
		s3 = s3Client[0]
	}
	return &Handler{service: service, repo: repo, s3Client: s3}
}

// WithStorage sets or overrides the storage client on the Handler.
func (h *Handler) WithStorage(s3Client storage.Client) *Handler {
	h.s3Client = s3Client
	return h
}

// Routes constructs a chi.Router mounting all auth, profile, and resume routes protected by authMiddleware.
func (h *Handler) Routes(authMiddleware func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	if authMiddleware != nil {
		r.Use(authMiddleware)
	}

	// Auth sync & me
	r.Post("/auth/sync", h.SyncUser)
	r.Get("/auth/me", h.GetMe)

	// Student profiles & resume streaming
	r.Get("/profile/student", h.GetMyStudentProfile)
	r.Put("/profile/student", h.UpdateMyStudentProfile)
	if h.s3Client != nil {
		r.Post("/profile/student/resume", h.UploadResume(h.s3Client))
		r.Get("/profile/student/resume", h.GetMyResumeURL(h.s3Client))
		r.Get("/profile/student/{id}/resume", h.GetStudentResumeURL(h.s3Client))
	}
	r.Get("/profile/student/{id}", h.GetStudentProfileByID)

	// Employer profiles
	r.Get("/profile/employer", h.GetMyEmployerProfile)
	r.Put("/profile/employer", h.UpdateMyEmployerProfile)

	return r
}

// AuthRoutes provides subrouter for mounting at /auth prefix.
func (h *Handler) AuthRoutes(authMiddleware func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	if authMiddleware != nil {
		r.Use(authMiddleware)
	}
	r.Post("/sync", h.SyncUser)
	r.Get("/me", h.GetMe)
	return r
}

// ProfileRoutes provides subrouter for mounting at /profile prefix.
func (h *Handler) ProfileRoutes(authMiddleware func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	if authMiddleware != nil {
		r.Use(authMiddleware)
	}
	r.Get("/student", h.GetMyStudentProfile)
	r.Put("/student", h.UpdateMyStudentProfile)
	if h.s3Client != nil {
		r.Post("/student/resume", h.UploadResume(h.s3Client))
		r.Get("/student/resume", h.GetMyResumeURL(h.s3Client))
		r.Get("/student/{id}/resume", h.GetStudentResumeURL(h.s3Client))
	}
	r.Get("/student/{id}", h.GetStudentProfileByID)
	r.Get("/employer", h.GetMyEmployerProfile)
	r.Put("/employer", h.UpdateMyEmployerProfile)
	return r
}

// isStudent checks affirmative student authorization via JWT claims or database user record.
func (h *Handler) isStudent(ctx context.Context, claims *auth.UserClaims, userUUID uuid.UUID) bool {
	if claims.HasRole("student") {
		return true
	}
	u, err := h.repo.GetUserByID(ctx, userUUID)
	return err == nil && u != nil && u.Role == "student"
}

// isEmployer checks affirmative employer authorization via JWT claims or database user record.
func (h *Handler) isEmployer(ctx context.Context, claims *auth.UserClaims, userUUID uuid.UUID) bool {
	if claims.HasRole("employer") {
		return true
	}
	u, err := h.repo.GetUserByID(ctx, userUUID)
	return err == nil && u != nil && u.Role == "employer"
}

// SyncUser handles POST /api/v1/auth/sync
func (h *Handler) SyncUser(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
		return
	}

	var req SyncUserRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body")
			return
		}
	}

	user, err := h.service.SyncUser(r.Context(), claims, req)
	if err != nil {
		if errors.Is(err, ErrInvalidRole) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ROLE", "Role must be student or employer")
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user UUID")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to sync user")
		return
	}

	writeJSONSuccess(w, http.StatusOK, user)
}

// GetMe handles GET /api/v1/auth/me
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
		return
	}

	summary, err := h.service.GetMe(r.Context(), claims)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user UUID")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get user profile")
		return
	}

	writeJSONSuccess(w, http.StatusOK, summary)
}

// GetMyStudentProfile handles GET /api/v1/profile/student
func (h *Handler) GetMyStudentProfile(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
		return
	}

	userUUID, err := uuid.Parse(claims.UserID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user UUID")
		return
	}

	if !h.isStudent(r.Context(), claims, userUUID) {
		writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "Student role required")
		return
	}

	profile, err := h.service.GetStudentProfile(r.Context(), userUUID)
	if err != nil {
		if errors.Is(err, ErrProfileNotFound) {
			writeJSONError(w, http.StatusNotFound, "PROFILE_NOT_FOUND", "Student profile not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get student profile")
		return
	}

	writeJSONSuccess(w, http.StatusOK, profile)
}

// UpdateMyStudentProfile handles PUT /api/v1/profile/student
func (h *Handler) UpdateMyStudentProfile(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
		return
	}

	userUUID, err := uuid.Parse(claims.UserID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user UUID")
		return
	}

	if !h.isStudent(r.Context(), claims, userUUID) {
		writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "Student role required")
		return
	}

	var req UpdateStudentProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body")
		return
	}

	profile, err := h.service.UpdateStudentProfile(r.Context(), userUUID, req)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update student profile")
		return
	}

	writeJSONSuccess(w, http.StatusOK, profile)
}

// GetStudentProfileByID handles GET /api/v1/profile/student/{id}
func (h *Handler) GetStudentProfileByID(w http.ResponseWriter, r *http.Request) {
	_, err := auth.GetUserContext(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid student ID")
		return
	}

	profile, err := h.service.GetStudentProfileByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrProfileNotFound) {
			writeJSONError(w, http.StatusNotFound, "PROFILE_NOT_FOUND", "Student profile not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve student profile")
		return
	}

	writeJSONSuccess(w, http.StatusOK, profile)
}

// GetMyEmployerProfile handles GET /api/v1/profile/employer
func (h *Handler) GetMyEmployerProfile(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
		return
	}

	userUUID, err := uuid.Parse(claims.UserID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user UUID")
		return
	}

	if !h.isEmployer(r.Context(), claims, userUUID) {
		writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "Employer role required")
		return
	}

	profile, err := h.service.GetEmployerProfile(r.Context(), userUUID)
	if err != nil {
		if errors.Is(err, ErrProfileNotFound) {
			writeJSONError(w, http.StatusNotFound, "PROFILE_NOT_FOUND", "Employer profile not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get employer profile")
		return
	}

	writeJSONSuccess(w, http.StatusOK, profile)
}

// UpdateMyEmployerProfile handles PUT /api/v1/profile/employer
func (h *Handler) UpdateMyEmployerProfile(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
		return
	}

	userUUID, err := uuid.Parse(claims.UserID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user UUID")
		return
	}

	if !h.isEmployer(r.Context(), claims, userUUID) {
		writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "Employer role required")
		return
	}

	var req UpdateEmployerProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body")
		return
	}

	profile, err := h.service.UpdateEmployerProfile(r.Context(), userUUID, req)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update employer profile")
		return
	}

	writeJSONSuccess(w, http.StatusOK, profile)
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
