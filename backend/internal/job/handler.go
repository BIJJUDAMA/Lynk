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
		if val, err := uuid.Parse(createdByStr); err == nil {
			filter.CreatedBy = &val
		}
	}

	jobs, err := h.service.ListJobs(r.Context(), filter)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve jobs")
		return
	}

	writeJSONSuccess(w, http.StatusOK, jobs)
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
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Invalid job UUID")
		return
	}

	job, err := h.service.GetJobByID(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			writeJSONError(w, http.StatusNotFound, "JOB_NOT_FOUND", "Job not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve job")
		return
	}

	writeJSONSuccess(w, http.StatusOK, job)
}

// CreateJob handles POST /api/v1/jobs (Any verified campus member)
func (h *Handler) CreateJob(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
		return
	}

	// Verification check: campus email must be verified to post opportunities
	if !claims.EmailVerified {
		writeJSONError(w, http.StatusForbidden, "EMAIL_NOT_VERIFIED", "Campus verification required to post opportunities")
		return
	}

	userUUID, err := uuid.Parse(claims.UserID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user UUID")
		return
	}

	var req CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			writeJSONError(w, http.StatusBadRequest, "EMPTY_BODY", "Request body cannot be empty")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse request body")
		return
	}

	job, err := h.service.CreateJob(r.Context(), userUUID, req)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			writeJSONError(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create job")
		return
	}

	writeJSONSuccess(w, http.StatusCreated, job)
}

// GetMyJobs handles GET /api/v1/jobs/mine
func (h *Handler) GetMyJobs(w http.ResponseWriter, r *http.Request) {
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

	jobs, err := h.service.GetMyJobs(r.Context(), userUUID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve your jobs")
		return
	}

	writeJSONSuccess(w, http.StatusOK, jobs)
}

// UpdateJob handles PUT /api/v1/jobs/{id}
func (h *Handler) UpdateJob(w http.ResponseWriter, r *http.Request) {
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

	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		idStr = parts[len(parts)-1]
	}
	jobID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Invalid job UUID")
		return
	}

	var req UpdateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse request body")
		return
	}

	job, err := h.service.UpdateJob(r.Context(), userUUID, jobID, req)
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			writeJSONError(w, http.StatusNotFound, "JOB_NOT_FOUND", "Job not found")
			return
		}
		if errors.Is(err, ErrForbidden) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "Only the job creator can edit this opportunity")
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			writeJSONError(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update job")
		return
	}

	writeJSONSuccess(w, http.StatusOK, job)
}

// DeleteJob handles DELETE /api/v1/jobs/{id}
func (h *Handler) DeleteJob(w http.ResponseWriter, r *http.Request) {
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

	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		idStr = parts[len(parts)-1]
	}
	jobID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Invalid job UUID")
		return
	}

	err = h.service.DeleteJob(r.Context(), userUUID, jobID)
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			writeJSONError(w, http.StatusNotFound, "JOB_NOT_FOUND", "Job not found")
			return
		}
		if errors.Is(err, ErrForbidden) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "Only the job creator can delete this opportunity")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete job")
		return
	}

	w.WriteHeader(http.StatusNoContent)
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
