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

	// Protected routes (any verified campus member)
	if authMiddleware != nil {
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
	} else {
		r.Post("/", h.CreateJob)
		r.Post("/jobs", h.CreateJob)

		r.Get("/mine", h.GetMyJobs)
		r.Get("/jobs/mine", h.GetMyJobs)

		r.Put("/{id}", h.UpdateJob)
		r.Put("/jobs/{id}", h.UpdateJob)

		r.Delete("/{id}", h.DeleteJob)
		r.Delete("/jobs/{id}", h.DeleteJob)
	}

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
		Skill:      strings.TrimSpace(q.Get("skill")),
		PayType:    strings.ToLower(strings.TrimSpace(q.Get("pay_type"))),
		Status:     strings.ToLower(strings.TrimSpace(q.Get("status"))),
	}

	if minStr := strings.TrimSpace(q.Get("min_budget")); minStr != "" {
		if val, err := strconv.ParseFloat(minStr, 64); err == nil {
			filter.MinBudget = &val
		}
	}
	if maxStr := strings.TrimSpace(q.Get("max_budget")); maxStr != "" {
		if val, err := strconv.ParseFloat(maxStr, 64); err == nil {
			filter.MaxBudget = &val
		}
	}
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

	// Verification check: campus email must be verified to post opportunities
	if !claims.EmailVerified {
		httputil.WriteError(w, r, http.StatusForbidden, "EMAIL_NOT_VERIFIED", "Campus verification required to post opportunities", nil)
		return
	}

	if strings.TrimSpace(claims.UserID) == "" {
		httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_USER_ID", "User ID is required", nil)
		return
	}

	var req CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			httputil.WriteError(w, r, http.StatusBadRequest, "EMPTY_BODY", "Request body cannot be empty", err)
			return
		}
		httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "Failed to parse request body", err)
		return
	}

	job, err := h.service.CreateJob(r.Context(), claims.UserID, req)
	if err != nil {
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

	if strings.TrimSpace(claims.UserID) == "" {
		httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_USER_ID", "User ID is required", nil)
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

	var req UpdateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "Failed to parse request body", err)
		return
	}

	job, err := h.service.UpdateJob(r.Context(), claims.UserID, jobID, req)
	if err != nil {
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

	if strings.TrimSpace(claims.UserID) == "" {
		httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_USER_ID", "User ID is required", nil)
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

	err = h.service.DeleteJob(r.Context(), claims.UserID, jobID)
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			httputil.WriteError(w, r, http.StatusNotFound, "JOB_NOT_FOUND", "Job not found", nil)
			return
		}
		if errors.Is(err, ErrForbidden) {
			httputil.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Only the job creator can delete this opportunity", nil)
			return
		}
		httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete job", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
