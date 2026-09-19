package job

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

// Handler handles job-related HTTP requests.
type Handler struct {
	service *Service
	repo    JobRepository
}

// NewHandler creates a new job handler.
func NewHandler(service *Service, repo JobRepository) *Handler {
	if repo == nil && service != nil {
		repo = service.repo
	}
	return &Handler{service: service, repo: repo}
}

// Routes constructs a chi.Router mounting all job routes.
func (h *Handler) Routes(authMiddleware func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()

	// Public routes
	r.Get("/", h.ListJobs)
	r.Get("/jobs", h.ListJobs)

	authMiddleware = httpx.DefaultAuthMiddleware(authMiddleware)

	// Protected routes (any verified campus member)
	r.Group(func(pr chi.Router) {
		pr.Use(authMiddleware)

		pr.Post("/", h.CreateJob)
		pr.Post("/jobs", h.CreateJob)

		pr.Get("/mine", h.GetMyJobs)
		pr.Get("/jobs/mine", h.GetMyJobs)

		pr.Put("/{id}", h.UpdateJob)
		pr.Put("/jobs/{id}", h.UpdateJob)

		pr.Delete("/{id}", h.DeleteJob)
		pr.Delete("/jobs/{id}", h.DeleteJob)
	})

	// Public single job detail route
	r.Get("/{id}", h.GetJobByID)
	r.Get("/jobs/{id}", h.GetJobByID)

	return r
}

// ListJobs handles GET /api/v1/jobs
func (h *Handler) ListJobs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := JobFilter{
		Search:     strings.TrimSpace(q.Get("search")),
		Department: strings.TrimSpace(q.Get("department")),
		Status:     strings.ToLower(strings.TrimSpace(q.Get("status"))),
	}
	single, many := ParseSkillQuery(q["skill"])
	filter.Skill = single
	filter.Skills = many

	if limitStr := strings.TrimSpace(q.Get("limit")); limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = val
		}
	}
	if offsetStr := strings.TrimSpace(q.Get("offset")); offsetStr != "" {
		if val, err := strconv.Atoi(offsetStr); err == nil {
			filter.Offset = val
		}
	}
	if createdByStr := strings.TrimSpace(q.Get("created_by")); createdByStr != "" {
		filter.CreatedBy = &createdByStr
	}

	jobs, err := h.service.ListJobs(r.Context(), filter)
	if err != nil {
		httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve jobs", err)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, jobs)
}

// GetJobByID handles GET /api/v1/jobs/{id}
func (h *Handler) GetJobByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		idStr = parts[len(parts)-1]
	}

	jobID, err := uuid.Parse(idStr)
	if err != nil {
		httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "Invalid job UUID", err)
		return
	}

	job, err := h.service.GetJobByID(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			httputil.WriteError(w, r, http.StatusNotFound, "JOB_NOT_FOUND", "Job not found", nil)
			return
		}
		httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve job", err)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, job)
}

// CreateJob handles POST /api/v1/jobs (Any verified campus member)
func (h *Handler) CreateJob(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials", nil)
		return
	}

	if err := auth.CheckEmailVerified(claims); err != nil {
		if errors.Is(err, auth.ErrEmailNotVerified) {
			httputil.WriteError(w, r, http.StatusForbidden, "EMAIL_NOT_VERIFIED", auth.CampusVerificationPendingMsg, nil)
			return
		}
		httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials", nil)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "Failed to read request body", err)
		return
	}
	if len(body) == 0 {
		httputil.WriteError(w, r, http.StatusBadRequest, "EMPTY_BODY", "Request body cannot be empty", io.EOF)
		return
	}
	var req CreateJobRequest
	if err := json.Unmarshal(body, &req); err != nil {
		httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "Failed to parse request body", err)
		return
	}

	job, err := h.service.CreateJob(r.Context(), claims, req)
	if err != nil {
		if errors.Is(err, auth.ErrEmailNotVerified) {
			httputil.WriteError(w, r, http.StatusForbidden, "EMAIL_NOT_VERIFIED", auth.CampusVerificationPendingMsg, nil)
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			httputil.WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", err.Error(), err)
			return
		}
		httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create job", err)
		return
	}

	httputil.WriteSuccess(w, http.StatusCreated, job)
}

// GetMyJobs handles GET /api/v1/jobs/mine
func (h *Handler) GetMyJobs(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials", nil)
		return
	}

	if strings.TrimSpace(claims.UserID) == "" {
		httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_USER_ID", "User ID is required", nil)
		return
	}

	jobs, err := h.service.GetMyJobs(r.Context(), claims.UserID)
	if err != nil {
		httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve your jobs", err)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, jobs)
}

// UpdateJob handles PUT /api/v1/jobs/{id}
func (h *Handler) UpdateJob(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials", nil)
		return
	}

	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		idStr = parts[len(parts)-1]
	}
	jobID, err := uuid.Parse(idStr)
	if err != nil {
		httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "Invalid job UUID", err)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "Failed to read request body", err)
		return
	}
	var req UpdateJobRequest
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "Failed to parse request body", err)
			return
		}
	}

	job, err := h.service.UpdateJob(r.Context(), claims, jobID, req)
	if err != nil {
		if errors.Is(err, auth.ErrEmailNotVerified) {
			httputil.WriteError(w, r, http.StatusForbidden, "EMAIL_NOT_VERIFIED", auth.CampusVerificationPendingMsg, nil)
			return
		}
		if errors.Is(err, ErrJobNotFound) {
			httputil.WriteError(w, r, http.StatusNotFound, "JOB_NOT_FOUND", "Job not found", nil)
			return
		}
		if errors.Is(err, ErrForbidden) {
			httputil.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Only the job creator can edit this opportunity", nil)
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			httputil.WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", err.Error(), err)
			return
		}
		httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update job", err)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, job)
}

// DeleteJob handles DELETE /api/v1/jobs/{id}
func (h *Handler) DeleteJob(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials", nil)
		return
	}

	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		idStr = parts[len(parts)-1]
	}
	jobID, err := uuid.Parse(idStr)
	if err != nil {
		httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "Invalid job UUID", err)
		return
	}

	err = h.service.DeleteJob(r.Context(), claims, jobID)
	if err != nil {
		if errors.Is(err, auth.ErrEmailNotVerified) {
			httputil.WriteError(w, r, http.StatusForbidden, "EMAIL_NOT_VERIFIED", auth.CampusVerificationPendingMsg, nil)
			return
		}
		if errors.Is(err, ErrJobNotFound) {
			httputil.WriteError(w, r, http.StatusNotFound, "JOB_NOT_FOUND", "Job not found", nil)
			return
		}
		if errors.Is(err, ErrForbidden) {
			httputil.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Only the job creator can delete this opportunity", nil)
			return
		}
		if errors.Is(err, ErrJobHasDependents) {
			httputil.WriteError(w, r, http.StatusConflict, "JOB_HAS_DEPENDENTS", err.Error(), err)
			return
		}
		httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete job", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
