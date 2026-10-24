package recruitment

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lynk/backend/internal/auth"
	"github.com/lynk/backend/internal/httputil"
	"github.com/lynk/backend/internal/httpx"
	"github.com/lynk/backend/internal/project"
)

// Handler serves HTTP REST endpoints for the recruitment and application lifecycle.
type Handler struct {
	service *Service
}

// NewHandler constructs a new recruitment HTTP handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Service returns the underlying recruitment Service instance.
func (h *Handler) Service() *Service {
	return h.service
}

// HandleServiceError formats and returns appropriate HTTP responses for recruitment errors.
func (h *Handler) HandleServiceError(w http.ResponseWriter, r *http.Request, err error) {
	h.handleServiceError(w, r, err)
}

// Routes configures a Chi router mounting all recruitment and application lifecycle routes.
func (h *Handler) Routes(authMiddleware func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	authMiddleware = httpx.DefaultAuthMiddleware(authMiddleware)

	// Public / read question definitions
	r.Get("/projects/{id}/questions", h.ListQuestions)
	r.Get("/{id}/questions", h.ListQuestions)

	// Protected routes
	r.Group(func(pr chi.Router) {
		pr.Use(authMiddleware)

		// Question configuration
		pr.Post("/projects/{id}/questions", h.CreateQuestion)
		pr.Post("/{id}/questions", h.CreateQuestion)

		// Project applications
		pr.Post("/projects/{id}/applications", h.SubmitApplication)
		pr.Post("/{id}/applications", h.SubmitApplication)

		pr.Get("/projects/{id}/applications", h.ListProjectApplications)
		pr.Get("/{id}/applications", h.ListProjectApplications)

		// Application review and lifecycle actions
		pr.Get("/applications/me", h.ListMyApplications)
		pr.Get("/applications/mine", h.ListMyApplications)
		pr.Get("/applications/{id}", h.GetApplication)
		pr.Post("/applications/{id}/accept", h.AcceptApplication)
		pr.Post("/applications/{id}/reject", h.RejectApplication)
		pr.Post("/applications/{id}/shortlist", h.ShortlistApplication)
		pr.Post("/applications/{id}/withdraw", h.WithdrawApplication)

		// Direct mounts when mounted at /applications sub-router
		pr.Get("/me", h.ListMyApplications)
		pr.Get("/mine", h.ListMyApplications)
		pr.Get("/{id}", h.GetApplication)
		pr.Post("/{id}/accept", h.AcceptApplication)
		pr.Post("/{id}/reject", h.RejectApplication)
		pr.Post("/{id}/shortlist", h.ShortlistApplication)
		pr.Post("/{id}/withdraw", h.WithdrawApplication)
	})

	return r
}

// SubmitApplication handles POST /api/v1/projects/{id}/applications.
func (h *Handler) SubmitApplication(w http.ResponseWriter, r *http.Request) {
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

	var input SubmitApplicationInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_REQUEST_BODY", "Failed to parse application JSON body")
		return
	}

	app, err := h.service.SubmitApplication(r.Context(), claims, projectID, input)
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	httputil.WriteSuccess(w, http.StatusCreated, app)
}

// ListProjectApplications handles GET /api/v1/projects/{id}/applications.
func (h *Handler) ListProjectApplications(w http.ResponseWriter, r *http.Request) {
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

	var statusFilter *ApplicationStatus
	if st := strings.TrimSpace(r.URL.Query().Get("status")); st != "" {
		parsed := ApplicationStatus(st)
		statusFilter = &parsed
	}

	apps, err := h.service.ListProjectApplications(r.Context(), claims, projectID, statusFilter)
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	if apps == nil {
		apps = []Application{}
	}
	httputil.WriteSuccess(w, http.StatusOK, apps)
}

// ListQuestions handles GET /api/v1/projects/{id}/questions.
func (h *Handler) ListQuestions(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_PROJECT_ID", "Invalid project ID UUID")
		return
	}

	questions, err := h.service.ListQuestions(r.Context(), projectID)
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	if questions == nil {
		questions = []ApplicationQuestion{}
	}
	httputil.WriteSuccess(w, http.StatusOK, questions)
}

// CreateQuestion handles POST /api/v1/projects/{id}/questions.
func (h *Handler) CreateQuestion(w http.ResponseWriter, r *http.Request) {
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

	var input CreateQuestionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_REQUEST_BODY", "Failed to parse question JSON body")
		return
	}

	q, err := h.service.CreateQuestion(r.Context(), claims, projectID, input)
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	httputil.WriteSuccess(w, http.StatusCreated, q)
}

// ListMyApplications handles GET /api/v1/applications/me.
func (h *Handler) ListMyApplications(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil || claims.UserID == "" {
		httputil.WriteUnauthorized(w, r, "UNAUTHORIZED", "Authentication required")
		return
	}

	apps, err := h.service.ListMyApplications(r.Context(), claims)
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	if apps == nil {
		apps = []Application{}
	}
	httputil.WriteSuccess(w, http.StatusOK, apps)
}

// GetApplication handles GET /api/v1/applications/{id}.
func (h *Handler) GetApplication(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_APPLICATION_ID", "Invalid application ID UUID")
		return
	}

	claims, err := auth.GetUserContext(r.Context())
	if err != nil || claims.UserID == "" {
		httputil.WriteUnauthorized(w, r, "UNAUTHORIZED", "Authentication required")
		return
	}

	app, err := h.service.GetApplicationByID(r.Context(), claims, appID)
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, app)
}

// AcceptApplication handles POST /api/v1/applications/{id}/accept.
func (h *Handler) AcceptApplication(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_APPLICATION_ID", "Invalid application ID UUID")
		return
	}

	claims, err := auth.GetUserContext(r.Context())
	if err != nil || claims.UserID == "" {
		httputil.WriteUnauthorized(w, r, "UNAUTHORIZED", "Authentication required")
		return
	}

	if err := h.service.AcceptApplication(r.Context(), claims, appID); err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, map[string]string{
		"message": "application accepted and workspace membership created",
	})
}

// RejectApplication handles POST /api/v1/applications/{id}/reject.
func (h *Handler) RejectApplication(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_APPLICATION_ID", "Invalid application ID UUID")
		return
	}

	claims, err := auth.GetUserContext(r.Context())
	if err != nil || claims.UserID == "" {
		httputil.WriteUnauthorized(w, r, "UNAUTHORIZED", "Authentication required")
		return
	}

	var input ReviewDecisionInput
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&input)
	}

	if err := h.service.RejectApplication(r.Context(), claims, appID, input.ReviewerNotes); err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, map[string]string{
		"message": "application rejected successfully",
	})
}

// ShortlistApplication handles POST /api/v1/applications/{id}/shortlist.
func (h *Handler) ShortlistApplication(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_APPLICATION_ID", "Invalid application ID UUID")
		return
	}

	claims, err := auth.GetUserContext(r.Context())
	if err != nil || claims.UserID == "" {
		httputil.WriteUnauthorized(w, r, "UNAUTHORIZED", "Authentication required")
		return
	}

	var input ReviewDecisionInput
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&input)
	}

	if err := h.service.ShortlistApplication(r.Context(), claims, appID, input.ReviewerNotes); err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, map[string]string{
		"message": "application shortlisted successfully",
	})
}

// WithdrawApplication handles POST /api/v1/applications/{id}/withdraw.
func (h *Handler) WithdrawApplication(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_APPLICATION_ID", "Invalid application ID UUID")
		return
	}

	claims, err := auth.GetUserContext(r.Context())
	if err != nil || claims.UserID == "" {
		httputil.WriteUnauthorized(w, r, "UNAUTHORIZED", "Authentication required")
		return
	}

	if err := h.service.WithdrawApplication(r.Context(), claims, appID); err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, map[string]string{
		"message": "application withdrawn successfully",
	})
}

func (h *Handler) handleServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrUnauthorized):
		httputil.WriteUnauthorized(w, r, "UNAUTHORIZED", err.Error())
	case errors.Is(err, ErrEmailNotVerified):
		httputil.WriteForbidden(w, r, "EMAIL_NOT_VERIFIED", err.Error())
	case errors.Is(err, ErrForbidden):
		httputil.WriteForbidden(w, r, "FORBIDDEN", err.Error())
	case errors.Is(err, ErrApplicationNotFound):
		httputil.WriteNotFound(w, r, "APPLICATION_NOT_FOUND", err.Error())
	case errors.Is(err, project.ErrProjectNotFound):
		httputil.WriteNotFound(w, r, "PROJECT_NOT_FOUND", err.Error())
	case errors.Is(err, ErrAlreadyApplied):
		httputil.WriteConflict(w, r, "ALREADY_APPLIED", err.Error())
	case errors.Is(err, ErrAlreadyDecided):
		httputil.WriteConflict(w, r, "ALREADY_DECIDED", err.Error())
	case errors.Is(err, ErrTeamCapacityExceeded):
		httputil.WriteConflict(w, r, "TEAM_CAPACITY_EXCEEDED", err.Error())
	case errors.Is(err, ErrRecruitmentClosed):
		httputil.WriteConflict(w, r, "RECRUITMENT_CLOSED", err.Error())
	case errors.Is(err, ErrCannotWithdraw):
		httputil.WriteBadRequest(w, r, "CANNOT_WITHDRAW", err.Error())
	case errors.Is(err, ErrMissingRequiredAnswer), errors.Is(err, ErrInvalidStatement), errors.Is(err, ErrInvalidQuestionText):
		httputil.WriteBadRequest(w, r, "BAD_REQUEST", err.Error())
	default:
		httputil.WriteInternalServerError(w, r, err)
	}
}
