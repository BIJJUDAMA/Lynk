package application

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/httputil"
	"github.com/lynk/backend/internal/httpx"
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
		rt.Get("/applied", h.GetMyApplicationForJob)
		rt.Get("/applications/mine", h.GetMyApplications)
		rt.Get("/applications/applied", h.GetMyApplicationForJob)
		rt.Get("/api/v1/applications/mine", h.GetMyApplications)
		rt.Get("/api/v1/applications/applied", h.GetMyApplicationForJob)

		// Application detail and status mutation
		rt.Get("/{id}", h.GetApplicationByID)
		rt.Get("/applications/{id}", h.GetApplicationByID)
		rt.Get("/api/v1/applications/{id}", h.GetApplicationByID)

		rt.Patch("/{id}/status", h.UpdateApplicationStatus)
		rt.Patch("/applications/{id}/status", h.UpdateApplicationStatus)
		rt.Patch("/api/v1/applications/{id}/status", h.UpdateApplicationStatus)
	}

	authMiddleware = httpx.DefaultAuthMiddleware(authMiddleware)
	r.Group(func(pr chi.Router) {
		pr.Use(authMiddleware)
		routeSetup(pr)
	})

	return r
}

// JobApplicationRoutes provides a subrouter for mounting under /api/v1/jobs/{id}/applications.
func (h *Handler) JobApplicationRoutes(authMiddleware func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Use(httpx.DefaultAuthMiddleware(authMiddleware))
	r.Post("/", h.ApplyToJob)
	r.Get("/", h.ListJobApplications)
	return r
}

// ApplicationRoutes provides a subrouter for mounting under /api/v1/applications.
func (h *Handler) ApplicationRoutes(authMiddleware func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Use(httpx.DefaultAuthMiddleware(authMiddleware))
	r.Get("/mine", h.GetMyApplications)
	r.Get("/applied", h.GetMyApplicationForJob)
	r.Get("/{id}", h.GetApplicationByID)
	r.Patch("/{id}/status", h.UpdateApplicationStatus)
	return r
}

// ApplyToJob handles POST /api/v1/jobs/{id}/applications (Member applies with cover letter).
func (h *Handler) ApplyToJob(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials", nil)
		return
	}

	// Institutional Email Gate: Reject unverified applicants with 403 Forbidden
	if !claims.EmailVerified {
		httputil.WriteError(w, r, http.StatusForbidden, "EMAIL_NOT_VERIFIED", auth.CampusVerificationPendingMsg, nil)
		return
	}

	jobID, err := extractJobID(r)
	if err != nil {
		httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "Invalid job UUID", err)
		return
	}

	var req ApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			httputil.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Request body cannot be empty", err)
			return
		}
		httputil.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON request body", err)
		return
	}

	app, err := h.service.ApplyToJob(r.Context(), claims, jobID, req)
	if err != nil {
		if errors.Is(err, ErrEmailNotVerified) {
			httputil.WriteError(w, r, http.StatusForbidden, "EMAIL_NOT_VERIFIED", auth.CampusVerificationPendingMsg, nil)
			return
		}
		if errors.Is(err, ErrForbidden) {
			httputil.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		if errors.Is(err, ErrJobNotFound) {
			httputil.WriteError(w, r, http.StatusNotFound, "JOB_NOT_FOUND", "Job not found", nil)
			return
		}
		if errors.Is(err, ErrJobNotOpen) {
			httputil.WriteError(w, r, http.StatusBadRequest, "JOB_NOT_OPEN", "Job is not open for applications", nil)
			return
		}
		if errors.Is(err, ErrDuplicateApplication) {
			httputil.WriteError(w, r, http.StatusConflict, "APPLICATION_ALREADY_EXISTS", "You have already applied to this job", nil)
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error(), err)
			return
		}
		httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to submit application", err)
		return
	}

	httputil.WriteSuccess(w, http.StatusCreated, app)
}

// ListJobApplications handles GET /api/v1/jobs/{id}/applications (Job Creator views applications).
func (h *Handler) ListJobApplications(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials", nil)
		return
	}

	jobID, err := extractJobID(r)
	if err != nil {
		httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "Invalid job UUID", err)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	apps, err := h.service.ListJobApplications(r.Context(), claims, jobID, limit, offset)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			httputil.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		if errors.Is(err, ErrJobNotFound) {
			httputil.WriteError(w, r, http.StatusNotFound, "JOB_NOT_FOUND", "Job not found", nil)
			return
		}
		httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve job applications", err)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, apps)
}

// GetMyApplications handles GET /api/v1/applications/mine (Member views own applications).
func (h *Handler) GetMyApplications(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials", nil)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	apps, err := h.service.ListMyApplications(r.Context(), claims, limit, offset)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			httputil.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve applications", err)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, apps)
}

// GetMyApplicationForJob handles GET /api/v1/applications/applied?job_id=<uuid>.
func (h *Handler) GetMyApplicationForJob(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing session", err)
		return
	}

	jobID, err := uuid.Parse(strings.TrimSpace(r.URL.Query().Get("job_id")))
	if err != nil {
		httputil.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "job_id query param must be a UUID", err)
		return
	}

	app, err := h.service.GetMyApplicationForJob(r.Context(), claims.UserID, jobID)
	if err != nil {
		httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to lookup application", err)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, app)
}

// GetApplicationByID handles GET /api/v1/applications/{id} (Applicant or Job Creator).
func (h *Handler) GetApplicationByID(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials", nil)
		return
	}

	appID, err := extractApplicationID(r)
	if err != nil {
		httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "Invalid application UUID", err)
		return
	}

	app, err := h.service.GetApplicationByID(r.Context(), claims, appID)
	if err != nil {
		if errors.Is(err, ErrApplicationNotFound) {
			httputil.WriteError(w, r, http.StatusNotFound, "APPLICATION_NOT_FOUND", "Application not found", nil)
			return
		}
		if errors.Is(err, ErrForbidden) {
			httputil.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve application", err)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, app)
}

// UpdateApplicationStatus handles PATCH /api/v1/applications/{id}/status (Job Creator accepts/rejects).
func (h *Handler) UpdateApplicationStatus(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials", nil)
		return
	}

	appID, err := extractApplicationID(r)
	if err != nil {
		httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "Invalid application UUID", err)
		return
	}

	var req UpdateApplicationStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			httputil.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Request body cannot be empty", err)
			return
		}
		httputil.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON request body", err)
		return
	}

	app, _, err := h.service.UpdateApplicationStatus(r.Context(), claims, appID, req)
	if err != nil {
		if errors.Is(err, ErrApplicationNotFound) {
			httputil.WriteError(w, r, http.StatusNotFound, "APPLICATION_NOT_FOUND", "Application not found", nil)
			return
		}
		if errors.Is(err, ErrForbidden) {
			httputil.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		if errors.Is(err, ErrApplicationNotPending) {
			httputil.WriteError(w, r, http.StatusBadRequest, "APPLICATION_NOT_PENDING", "Application is not in pending status", nil)
			return
		}
		if errors.Is(err, ErrJobNotOpen) {
			httputil.WriteError(w, r, http.StatusBadRequest, "JOB_NOT_OPEN", "Job is no longer open", nil)
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error(), err)
			return
		}
		httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update application status", err)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, app)
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
