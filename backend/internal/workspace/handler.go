package workspace

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/httputil"
	"github.com/lynk/backend/internal/httpx"
	"github.com/lynk/backend/internal/project"
)

// Handler serves HTTP endpoints for project workspace sections, resources, and activity.
type Handler struct {
	service *Service
}

// NewHandler constructs a new workspace handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Routes configures a Chi router mounting all workspace routes.
func (h *Handler) Routes(authMiddleware func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	authMiddleware = httpx.DefaultAuthMiddleware(authMiddleware)

	// Public / viewer routes (may carry optional auth token for private projects)
	r.Get("/projects/{id}/sections", h.GetSections)
	r.Get("/{id}/sections", h.GetSections)

	r.Get("/projects/{id}/resources", h.ListResources)
	r.Get("/{id}/resources", h.ListResources)

	r.Get("/projects/{id}/activity", h.ListActivity)
	r.Get("/{id}/activity", h.ListActivity)

	// Protected workspace management routes
	r.Group(func(pr chi.Router) {
		pr.Use(authMiddleware)

		pr.Put("/projects/{id}/sections", h.UpdateSections)
		pr.Put("/{id}/sections", h.UpdateSections)

		pr.Post("/projects/{id}/resources", h.AddResource)
		pr.Post("/{id}/resources", h.AddResource)

		pr.Delete("/projects/{id}/resources/{resourceId}", h.DeleteResource)
		pr.Delete("/{id}/resources/{resourceId}", h.DeleteResource)
	})

	return r
}

// parseUserID extracts the optional authenticated user ID from request context.
func parseUserID(r *http.Request) string {
	claims, err := auth.GetUserContext(r.Context())
	if err == nil && claims != nil {
		return claims.UserID
	}
	return ""
}

// GetSections handles GET /api/v1/projects/{id}/sections.
func (h *Handler) GetSections(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_PROJECT_ID", "Invalid project ID UUID")
		return
	}

	userID := parseUserID(r)
	sections, err := h.service.GetSections(r.Context(), projectID, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrUnauthorizedWorkspaceAction):
			httputil.WriteForbidden(w, r, "FORBIDDEN", err.Error())
		case errors.Is(err, project.ErrProjectNotFound):
			httputil.WriteNotFound(w, r, "PROJECT_NOT_FOUND", err.Error())
		default:
			httputil.WriteInternalServerError(w, r, err)
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, sections)
}

// UpdateSections handles PUT /api/v1/projects/{id}/sections.
func (h *Handler) UpdateSections(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_PROJECT_ID", "Invalid project ID UUID")
		return
	}

	claims, err := auth.GetUserContext(r.Context())
	if err != nil || claims.UserID == "" {
		httputil.WriteUnauthorized(w, r, "UNAUTHORIZED", "Authentication required")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_REQUEST_BODY", "Failed to read request body")
		return
	}

	var sections []ProjectSection
	var wrapper UpsertSectionsInput
	if err := json.Unmarshal(body, &wrapper); err == nil && len(wrapper.Sections) > 0 {
		sections = wrapper.Sections
	} else if err := json.Unmarshal(body, &sections); err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_REQUEST_BODY", "Failed to parse sections array or object")
		return
	}

	if err := h.service.UpdateSections(r.Context(), projectID, claims.UserID, sections); err != nil {
		switch {
		case errors.Is(err, ErrUnauthorizedWorkspaceAction):
			httputil.WriteForbidden(w, r, "FORBIDDEN", err.Error())
		case errors.Is(err, project.ErrProjectNotFound):
			httputil.WriteNotFound(w, r, "PROJECT_NOT_FOUND", err.Error())
		case errors.Is(err, ErrInvalidSectionType),
			errors.Is(err, ErrInvalidPosition),
			errors.Is(err, ErrDuplicateSectionType):
			httputil.WriteBadRequest(w, r, "BAD_REQUEST", err.Error())
		default:
			httputil.WriteInternalServerError(w, r, err)
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, map[string]string{"message": "sections updated successfully"})
}

// ListResources handles GET /api/v1/projects/{id}/resources.
func (h *Handler) ListResources(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_PROJECT_ID", "Invalid project ID UUID")
		return
	}

	userID := parseUserID(r)
	resources, err := h.service.ListResources(r.Context(), projectID, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrUnauthorizedWorkspaceAction):
			httputil.WriteForbidden(w, r, "FORBIDDEN", err.Error())
		case errors.Is(err, project.ErrProjectNotFound):
			httputil.WriteNotFound(w, r, "PROJECT_NOT_FOUND", err.Error())
		default:
			httputil.WriteInternalServerError(w, r, err)
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, resources)
}

// AddResource handles POST /api/v1/projects/{id}/resources.
func (h *Handler) AddResource(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_PROJECT_ID", "Invalid project ID UUID")
		return
	}

	claims, err := auth.GetUserContext(r.Context())
	if err != nil || claims.UserID == "" {
		httputil.WriteUnauthorized(w, r, "UNAUTHORIZED", "Authentication required")
		return
	}

	var input CreateResourceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_REQUEST_BODY", "Failed to parse JSON request body")
		return
	}

	res, err := h.service.AddResource(r.Context(), projectID, claims.UserID, input)
	if err != nil {
		switch {
		case errors.Is(err, ErrUnauthorizedWorkspaceAction):
			httputil.WriteForbidden(w, r, "FORBIDDEN", err.Error())
		case errors.Is(err, project.ErrProjectNotFound):
			httputil.WriteNotFound(w, r, "PROJECT_NOT_FOUND", err.Error())
		case errors.Is(err, ErrInvalidResourceURL), errors.Is(err, ErrInvalidResourceInput):
			httputil.WriteBadRequest(w, r, "BAD_REQUEST", err.Error())
		default:
			httputil.WriteInternalServerError(w, r, err)
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusCreated, res)
}

// DeleteResource handles DELETE /api/v1/projects/{id}/resources/{resourceId}.
func (h *Handler) DeleteResource(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_PROJECT_ID", "Invalid project ID UUID")
		return
	}

	resourceID, err := uuid.Parse(chi.URLParam(r, "resourceId"))
	if err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_RESOURCE_ID", "Invalid resource ID UUID")
		return
	}

	claims, err := auth.GetUserContext(r.Context())
	if err != nil || claims.UserID == "" {
		httputil.WriteUnauthorized(w, r, "UNAUTHORIZED", "Authentication required")
		return
	}

	if err := h.service.DeleteResource(r.Context(), projectID, resourceID, claims.UserID); err != nil {
		switch {
		case errors.Is(err, ErrUnauthorizedWorkspaceAction):
			httputil.WriteForbidden(w, r, "FORBIDDEN", err.Error())
		case errors.Is(err, ErrResourceNotFound):
			httputil.WriteNotFound(w, r, "RESOURCE_NOT_FOUND", err.Error())
		default:
			httputil.WriteInternalServerError(w, r, err)
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, map[string]string{"message": "resource deleted successfully"})
}

// ListActivity handles GET /api/v1/projects/{id}/activity.
func (h *Handler) ListActivity(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_PROJECT_ID", "Invalid project ID UUID")
		return
	}

	limit := 50
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		if parsedLimit, err := strconv.Atoi(rawLimit); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	userID := parseUserID(r)
	activities, err := h.service.ListActivity(r.Context(), projectID, userID, limit)
	if err != nil {
		switch {
		case errors.Is(err, ErrUnauthorizedWorkspaceAction):
			httputil.WriteForbidden(w, r, "FORBIDDEN", err.Error())
		case errors.Is(err, project.ErrProjectNotFound):
			httputil.WriteNotFound(w, r, "PROJECT_NOT_FOUND", err.Error())
		default:
			httputil.WriteInternalServerError(w, r, err)
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, activities)
}
