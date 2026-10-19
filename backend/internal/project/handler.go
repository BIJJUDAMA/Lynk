package project

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/httputil"
	"github.com/lynk/backend/internal/httpx"
)

// Handler exposes HTTP REST endpoints for the project domain.
type Handler struct {
	service *Service
	repo    Repository
}

// NewHandler creates a new project handler instance.
func NewHandler(service *Service, repo Repository) *Handler {
	if repo == nil && service != nil {
		repo = service.repo
	}
	return &Handler{service: service, repo: repo}
}

// Routes constructs a chi.Router mounting all project endpoints.
func (h *Handler) Routes(authMiddleware func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()

	authMiddleware = httpx.DefaultAuthMiddleware(authMiddleware)

	// Public collection routes
	r.Get("/", h.ListProjects)
	r.Get("/projects", h.ListProjects)

	// Protected mutations
	r.Group(func(pr chi.Router) {
		pr.Use(authMiddleware)

		pr.Post("/", h.CreateProject)
		pr.Post("/projects", h.CreateProject)

		pr.Patch("/{id}", h.UpdateProject)
		pr.Put("/{id}", h.UpdateProject)
		pr.Patch("/projects/{id}", h.UpdateProject)
		pr.Put("/projects/{id}", h.UpdateProject)

		pr.Patch("/{id}/visibility", h.UpdateVisibility)
		pr.Put("/{id}/visibility", h.UpdateVisibility)
		pr.Patch("/projects/{id}/visibility", h.UpdateVisibility)
		pr.Put("/projects/{id}/visibility", h.UpdateVisibility)
	})

	// Parameterized single resource retrieval
	r.Get("/{idOrSlug}", h.GetProject)
	r.Get("/projects/{idOrSlug}", h.GetProject)

	return r
}

// ListProjects handles GET /api/v1/projects.
func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := ListFilter{
		Category:          strings.TrimSpace(q.Get("category")),
		Visibility:        Visibility(strings.ToLower(strings.TrimSpace(q.Get("visibility")))),
		RecruitmentStatus: RecruitmentStatus(strings.ToLower(strings.TrimSpace(q.Get("recruitment_status")))),
		LifecycleStatus:   LifecycleStatus(strings.ToLower(strings.TrimSpace(q.Get("lifecycle_status")))),
		SearchQuery:       strings.TrimSpace(q.Get("search")),
		OwnerMemberID:     strings.TrimSpace(q.Get("owner_id")),
		Cursor:            strings.TrimSpace(q.Get("cursor")),
	}
	if filter.SearchQuery == "" {
		filter.SearchQuery = strings.TrimSpace(q.Get("q"))
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

	var viewerID *string
	claims, err := auth.GetUserContext(r.Context())
	if err == nil && claims != nil && claims.UserID != "" {
		viewerID = &claims.UserID
	}

	projects, _, err := h.service.ListProjects(r.Context(), filter, viewerID)
	if err != nil {
		httputil.WriteInternalServerError(w, r, err)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, projects)
}

// GetProject handles GET /api/v1/projects/{idOrSlug}.
func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
	idOrSlug := chi.URLParam(r, "idOrSlug")
	if idOrSlug == "" {
		idOrSlug = chi.URLParam(r, "id")
		if idOrSlug == "" {
			idOrSlug = chi.URLParam(r, "slug")
		}
	}
	if idOrSlug == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		idOrSlug = parts[len(parts)-1]
	}

	var viewerID *string
	claims, err := auth.GetUserContext(r.Context())
	if err == nil && claims != nil && claims.UserID != "" {
		viewerID = &claims.UserID
	}

	if id, err := uuid.Parse(idOrSlug); err == nil {
		p, err := h.service.GetProjectByID(r.Context(), id, viewerID)
		if err == nil {
			httputil.WriteSuccess(w, http.StatusOK, p)
			return
		}
		if !errors.Is(err, ErrProjectNotFound) {
			h.handleServiceError(w, r, err, viewerID)
			return
		}
	}

	p, err := h.service.GetProjectBySlug(r.Context(), idOrSlug, viewerID)
	if err != nil {
		h.handleServiceError(w, r, err, viewerID)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, p)
}

// CreateProject handles POST /api/v1/projects.
func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil || claims == nil || claims.UserID == "" {
		httputil.WriteUnauthorized(w, r, "UNAUTHORIZED", "Missing credentials")
		return
	}

	if err := auth.CheckEmailVerified(claims); err != nil {
		if errors.Is(err, auth.ErrEmailNotVerified) {
			httputil.WriteForbidden(w, r, "EMAIL_NOT_VERIFIED", auth.CampusVerificationPendingMsg)
			return
		}
		httputil.WriteUnauthorized(w, r, "UNAUTHORIZED", "Missing credentials")
		return
	}

	var input CreateProjectInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_JSON", "Malformed request body")
		return
	}

	created, err := h.service.CreateProject(r.Context(), claims.UserID, input)
	if err != nil {
		h.handleServiceError(w, r, err, &claims.UserID)
		return
	}

	httputil.WriteSuccess(w, http.StatusCreated, created)
}

// UpdateProject handles PATCH /api/v1/projects/{id}.
func (h *Handler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil || claims == nil || claims.UserID == "" {
		httputil.WriteUnauthorized(w, r, "UNAUTHORIZED", "Missing credentials")
		return
	}

	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		idStr = parts[len(parts)-1]
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_PROJECT_ID", "Invalid project UUID")
		return
	}

	var input UpdateProjectInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_JSON", "Malformed request body")
		return
	}

	updated, err := h.service.UpdateProject(r.Context(), id, claims.UserID, input)
	if err != nil {
		h.handleServiceError(w, r, err, &claims.UserID)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, updated)
}

// UpdateVisibility handles PATCH /api/v1/projects/{id}/visibility.
func (h *Handler) UpdateVisibility(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil || claims == nil || claims.UserID == "" {
		httputil.WriteUnauthorized(w, r, "UNAUTHORIZED", "Missing credentials")
		return
	}

	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 2 {
			idStr = parts[len(parts)-2]
		}
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_PROJECT_ID", "Invalid project UUID")
		return
	}

	var input UpdateVisibilityInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_JSON", "Malformed request body")
		return
	}

	updated, err := h.service.UpdateVisibility(r.Context(), id, claims.UserID, input.Visibility)
	if err != nil {
		h.handleServiceError(w, r, err, &claims.UserID)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, updated)
}

func (h *Handler) handleServiceError(w http.ResponseWriter, r *http.Request, err error, viewerID *string) {
	if errors.Is(err, ErrProjectNotFound) {
		httputil.WriteNotFound(w, r, "PROJECT_NOT_FOUND", "Project not found")
		return
	}
	if errors.Is(err, ErrUnauthorized) {
		if viewerID == nil {
			httputil.WriteUnauthorized(w, r, "UNAUTHORIZED", "Authentication required to access this project")
		} else {
			httputil.WriteForbidden(w, r, "FORBIDDEN", "You do not have permission to access this project")
		}
		return
	}
	if errors.Is(err, ErrForbidden) {
		httputil.WriteForbidden(w, r, "FORBIDDEN", err.Error())
		return
	}
	if errors.Is(err, ErrDuplicateSlug) {
		httputil.WriteConflict(w, r, "DUPLICATE_SLUG", err.Error())
		return
	}
	if errors.Is(err, ErrInvalidTitle) ||
		errors.Is(err, ErrInvalidDescription) ||
		errors.Is(err, ErrInvalidVisibility) ||
		errors.Is(err, ErrInvalidRecruitment) ||
		errors.Is(err, ErrInvalidLifecycle) ||
		errors.Is(err, ErrInvalidSlug) {
		httputil.WriteBadRequest(w, r, "VALIDATION_ERROR", err.Error())
		return
	}
	httputil.WriteInternalServerError(w, r, err)
}
