package user

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/storage"
)

// Handler handles user authentication sync and unified profile management endpoints.
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

	// Unified profile & resume streaming
	r.Get("/profile/me", h.GetMyProfile)
	r.Put("/profile/me", h.UpdateMyProfile)
	r.Get("/profile/{id}", h.GetProfileByID)

	if h.s3Client != nil {
		r.Post("/profile/resume", h.UploadResume(h.s3Client))
		r.Get("/profile/resume", h.GetMyResumeURL(h.s3Client))
		r.Get("/profile/{id}/resume", h.GetMemberResumeURL(h.s3Client))
	}

	// Backwards compatibility aliases
	r.Get("/profile/student", h.GetMyProfile)
	r.Put("/profile/student", h.UpdateMyProfile)
	r.Get("/profile/student/{id}", h.GetProfileByID)
	r.Get("/profile/employer", h.GetMyProfile)
	r.Put("/profile/employer", h.UpdateMyProfile)
	if h.s3Client != nil {
		r.Post("/profile/student/resume", h.UploadResume(h.s3Client))
		r.Get("/profile/student/resume", h.GetMyResumeURL(h.s3Client))
		r.Get("/profile/student/{id}/resume", h.GetMemberResumeURL(h.s3Client))
	}

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
	r.Get("/me", h.GetMyProfile)
	r.Put("/me", h.UpdateMyProfile)
	r.Get("/{id}", h.GetProfileByID)

	if h.s3Client != nil {
		r.Post("/resume", h.UploadResume(h.s3Client))
		r.Get("/resume", h.GetMyResumeURL(h.s3Client))
		r.Get("/{id}/resume", h.GetMemberResumeURL(h.s3Client))
	}

	// Backwards compatibility aliases
	r.Get("/student", h.GetMyProfile)
	r.Put("/student", h.UpdateMyProfile)
	r.Get("/student/{id}", h.GetProfileByID)
	r.Get("/employer", h.GetMyProfile)
	r.Put("/employer", h.UpdateMyProfile)
	if h.s3Client != nil {
		r.Post("/student/resume", h.UploadResume(h.s3Client))
		r.Get("/student/resume", h.GetMyResumeURL(h.s3Client))
		r.Get("/student/{id}/resume", h.GetMemberResumeURL(h.s3Client))
	}

	return r
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
			writeJSONError(w, http.StatusBadRequest, "INVALID_ROLE", "Role must be member or admin")
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

// GetMyProfile handles GET /api/v1/profile/me
func (h *Handler) GetMyProfile(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
		return
	}

	profile, err := h.service.GetProfile(r.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, ErrProfileNotFound) {
			writeJSONError(w, http.StatusNotFound, "PROFILE_NOT_FOUND", "Profile not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get profile")
		return
	}

	writeJSONSuccess(w, http.StatusOK, profile)
}

// UpdateMyProfile handles PUT /api/v1/profile/me
func (h *Handler) UpdateMyProfile(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
		return
	}

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON body")
		return
	}

	profile, err := h.service.UpdateProfile(r.Context(), claims.UserID, req)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update profile")
		return
	}

	writeJSONSuccess(w, http.StatusOK, profile)
}

// GetProfileByID handles GET /api/v1/profile/{id}
func (h *Handler) GetProfileByID(w http.ResponseWriter, r *http.Request) {
	_, err := auth.GetUserContext(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
		return
	}

	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid profile ID")
		return
	}

	profile, err := h.service.GetProfileByID(r.Context(), idStr)
	if err != nil {
		if errors.Is(err, ErrProfileNotFound) {
			writeJSONError(w, http.StatusNotFound, "PROFILE_NOT_FOUND", "Profile not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve profile")
		return
	}

	writeJSONSuccess(w, http.StatusOK, profile)
}

// Backwards compatibility handler aliases
func (h *Handler) GetMyStudentProfile(w http.ResponseWriter, r *http.Request) {
	h.GetMyProfile(w, r)
}

func (h *Handler) UpdateMyStudentProfile(w http.ResponseWriter, r *http.Request) {
	h.UpdateMyProfile(w, r)
}

func (h *Handler) GetStudentProfileByID(w http.ResponseWriter, r *http.Request) {
	h.GetProfileByID(w, r)
}

func (h *Handler) GetMyEmployerProfile(w http.ResponseWriter, r *http.Request) {
	h.GetMyProfile(w, r)
}

func (h *Handler) UpdateMyEmployerProfile(w http.ResponseWriter, r *http.Request) {
	h.UpdateMyProfile(w, r)
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
