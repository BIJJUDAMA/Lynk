package application

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

// Handler handles HTTP requests for job applications and institutional email verification.
type Handler struct {
	service *Service
	repo    ApplicationRepository
}

// NewHandler creates a new application handler.
func NewHandler(service *Service, repo ApplicationRepository) *Handler {
	if repo == nil && service != nil {
		repo = service.repo
	}
	return &Handler{service: service, repo: repo}
}

// Routes constructs a chi.Router mounting all application routes.
// Literal paths (/mine) are registered before parameterized paths (/{id}) to prevent route shadowing.
func (h *Handler) Routes(authMiddleware func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()

	routeSetup := func(rt chi.Router) {
		// Job applications
		rt.Post("/jobs/{id}/applications", h.ApplyToJob)
		rt.Get("/jobs/{id}/applications", h.ListJobApplications)
		rt.Post("/api/v1/jobs/{id}/applications", h.ApplyToJob)
		rt.Get("/api/v1/jobs/{id}/applications", h.ListJobApplications)

		// Member submitted applications (MUST precede /{id})
		rt.Get("/mine", h.GetMyApplications)
		rt.Get("/applications/mine", h.GetMyApplications)
		rt.Get("/api/v1/applications/mine", h.GetMyApplications)

		// Application detail and status mutation
		rt.Get("/{id}", h.GetApplicationByID)
		rt.Get("/applications/{id}", h.GetApplicationByID)
		rt.Get("/api/v1/applications/{id}", h.GetApplicationByID)

		rt.Patch("/{id}/status", h.UpdateApplicationStatus)
		rt.Patch("/applications/{id}/status", h.UpdateApplicationStatus)
		rt.Patch("/api/v1/applications/{id}/status", h.UpdateApplicationStatus)
	}

	if authMiddleware != nil {
		r.Group(func(pr chi.Router) {
			pr.Use(authMiddleware)
			routeSetup(pr)
		})
	} else {
		routeSetup(r)
	}

	return r
}

// JobApplicationRoutes provides a subrouter for mounting under /api/v1/jobs/{id}/applications.
func (h *Handler) JobApplicationRoutes(authMiddleware func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	if authMiddleware != nil {
		r.Use(authMiddleware)
	}
	r.Post("/", h.ApplyToJob)
	r.Get("/", h.ListJobApplications)
	return r
}

// ApplicationRoutes provides a subrouter for mounting under /api/v1/applications.
func (h *Handler) ApplicationRoutes(authMiddleware func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	if authMiddleware != nil {
		r.Use(authMiddleware)
	}
	r.Get("/mine", h.GetMyApplications)
	r.Get("/{id}", h.GetApplicationByID)
	r.Patch("/{id}/status", h.UpdateApplicationStatus)
	return r
}

// ApplyToJob handles POST /api/v1/jobs/{id}/applications (Member applies with cover letter).
func (h *Handler) ApplyToJob(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
		return
	}

	// Institutional Email Gate: Reject unverified applicants with 403 Forbidden
	if !claims.EmailVerified {
		writeJSONError(w, http.StatusForbidden, "EMAIL_NOT_VERIFIED", "University email must be verified before applying to jobs")
		return
	}

	jobID, err := extractJobID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Invalid job UUID")
		return
	}

	var req ApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Request body cannot be empty")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON request body")
		return
	}

	app, err := h.service.ApplyToJob(r.Context(), claims, jobID, req)
	if err != nil {
		if errors.Is(err, ErrEmailNotVerified) {
			writeJSONError(w, http.StatusForbidden, "EMAIL_NOT_VERIFIED", "University email must be verified before applying to jobs")
			return
		}
		if errors.Is(err, ErrForbidden) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		if errors.Is(err, ErrJobNotFound) {
			writeJSONError(w, http.StatusNotFound, "JOB_NOT_FOUND", "Job not found")
			return
		}
		if errors.Is(err, ErrJobNotOpen) {
			writeJSONError(w, http.StatusBadRequest, "JOB_NOT_OPEN", "Job is not open for applications")
			return
		}
		if errors.Is(err, ErrDuplicateApplication) {
			writeJSONError(w, http.StatusConflict, "APPLICATION_ALREADY_EXISTS", "You have already applied to this job")
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to submit application")
		return
	}

	writeJSONSuccess(w, http.StatusCreated, app)
}

// ListJobApplications handles GET /api/v1/jobs/{id}/applications (Job Creator views applications).
func (h *Handler) ListJobApplications(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
		return
	}

	jobID, err := extractJobID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Invalid job UUID")
		return
	}

	apps, err := h.service.ListJobApplications(r.Context(), claims, jobID)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		if errors.Is(err, ErrJobNotFound) {
			writeJSONError(w, http.StatusNotFound, "JOB_NOT_FOUND", "Job not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve job applications")
		return
	}

	writeJSONSuccess(w, http.StatusOK, apps)
}

// GetMyApplications handles GET /api/v1/applications/mine (Member views own applications).
func (h *Handler) GetMyApplications(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
		return
	}

	apps, err := h.service.ListMyApplications(r.Context(), claims)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve applications")
		return
	}

	writeJSONSuccess(w, http.StatusOK, apps)
}

// GetApplicationByID handles GET /api/v1/applications/{id} (Applicant or Job Creator).
func (h *Handler) GetApplicationByID(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
		return
	}

	appID, err := extractApplicationID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Invalid application UUID")
		return
	}

	app, err := h.service.GetApplicationByID(r.Context(), claims, appID)
	if err != nil {
		if errors.Is(err, ErrApplicationNotFound) {
			writeJSONError(w, http.StatusNotFound, "APPLICATION_NOT_FOUND", "Application not found")
			return
		}
		if errors.Is(err, ErrForbidden) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve application")
		return
	}

	writeJSONSuccess(w, http.StatusOK, app)
}

// UpdateApplicationStatus handles PATCH /api/v1/applications/{id}/status (Job Creator accepts/rejects).
func (h *Handler) UpdateApplicationStatus(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
		return
	}

	appID, err := extractApplicationID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Invalid application UUID")
		return
	}

	var req UpdateApplicationStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Request body cannot be empty")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON request body")
		return
	}

	app, _, err := h.service.UpdateApplicationStatus(r.Context(), claims, appID, req)
	if err != nil {
		if errors.Is(err, ErrApplicationNotFound) {
			writeJSONError(w, http.StatusNotFound, "APPLICATION_NOT_FOUND", "Application not found")
			return
		}
		if errors.Is(err, ErrForbidden) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		if errors.Is(err, ErrApplicationNotPending) {
			writeJSONError(w, http.StatusBadRequest, "APPLICATION_NOT_PENDING", "Application is not in pending status")
			return
		}
		if errors.Is(err, ErrJobNotOpen) {
			writeJSONError(w, http.StatusBadRequest, "JOB_NOT_OPEN", "Job is no longer open")
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update application status")
		return
	}

	writeJSONSuccess(w, http.StatusOK, app)
}

func extractJobID(r *http.Request) (uuid.UUID, error) {
	if val := chi.URLParam(r, "id"); val != "" {
		if id, err := uuid.Parse(val); err == nil {
			return id, nil
		}
	}
	if val := chi.URLParam(r, "jobID"); val != "" {
		if id, err := uuid.Parse(val); err == nil {
			return id, nil
		}
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for i, p := range parts {
		if p == "jobs" && i+1 < len(parts) {
			return uuid.Parse(parts[i+1])
		}
	}
	return uuid.Nil, errors.New("invalid job id in url")
}

func extractApplicationID(r *http.Request) (uuid.UUID, error) {
	if val := chi.URLParam(r, "id"); val != "" {
		if id, err := uuid.Parse(val); err == nil {
			return id, nil
		}
	}
	if val := chi.URLParam(r, "appID"); val != "" {
		if id, err := uuid.Parse(val); err == nil {
			return id, nil
		}
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for i, p := range parts {
		if (p == "applications" || p == "application") && i+1 < len(parts) && parts[i+1] != "mine" {
			return uuid.Parse(parts[i+1])
		}
	}
	return uuid.Nil, errors.New("invalid application id in url")
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
