package team

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
)

// Handler serves HTTP endpoints for team memberships and invitations.
type Handler struct {
	service *Service
	repo    Repository
}

// NewHandler constructs a new team handler.
func NewHandler(service *Service, repo Repository) *Handler {
	if repo == nil && service != nil {
		repo = service.repo
	}
	return &Handler{service: service, repo: repo}
}

// Routes configures a Chi router mounting all team and invitation routes.
func (h *Handler) Routes(authMiddleware func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	authMiddleware = httpx.DefaultAuthMiddleware(authMiddleware)

	// Projects routes (both canonical and nested)
	r.Get("/projects/{id}/members", h.ListMembers)
	r.Get("/{id}/members", h.ListMembers)

	r.Group(func(pr chi.Router) {
		pr.Use(authMiddleware)

		pr.Patch("/projects/{id}/members/{memberId}", h.UpdateRole)
		pr.Patch("/{id}/members/{memberId}", h.UpdateRole)

		pr.Delete("/projects/{id}/members/{memberId}", h.RemoveMember)
		pr.Delete("/{id}/members/{memberId}", h.RemoveMember)

		pr.Post("/projects/{id}/invitations", h.InviteMember)
		pr.Post("/{id}/invitations", h.InviteMember)

		pr.Post("/projects/{id}/transfer-leadership", h.TransferLeadership)
		pr.Post("/{id}/transfer-leadership", h.TransferLeadership)

		// Invitations route
		pr.Post("/invitations/{id}/accept", h.AcceptInvitation)
	})

	return r
}

// ListMembers handles GET /api/v1/projects/{id}/members.
func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_PROJECT_ID", "Invalid project ID UUID")
		return
	}

	members, err := h.service.ListMembers(r.Context(), projectID)
	if err != nil {
		httputil.WriteInternalServerError(w, r, err)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, members)
}

// UpdateRole handles PATCH /api/v1/projects/{id}/members/{memberId}.
func (h *Handler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_PROJECT_ID", "Invalid project ID UUID")
		return
	}
	memberID := strings.TrimSpace(chi.URLParam(r, "memberId"))
	if memberID == "" {
		httputil.WriteBadRequest(w, r, "INVALID_MEMBER_ID", "Member ID is required")
		return
	}

	claims, err := auth.GetUserContext(r.Context())
	if err != nil || claims.UserID == "" {
		httputil.WriteUnauthorized(w, r, "UNAUTHORIZED", "Authentication required")
		return
	}

	var input UpdateRoleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_REQUEST_BODY", "Failed to parse JSON request body")
		return
	}

	if err := h.service.UpdateMemberRole(r.Context(), projectID, claims.UserID, memberID, input.Role); err != nil {
		switch {
		case errors.Is(err, ErrUnauthorizedAction):
			httputil.WriteForbidden(w, r, "FORBIDDEN", err.Error())
		case errors.Is(err, ErrMemberNotFound):
			httputil.WriteNotFound(w, r, "MEMBER_NOT_FOUND", err.Error())
		case errors.Is(err, ErrCannotDemoteLead), errors.Is(err, ErrCannotChangeOwnRole), errors.Is(err, ErrInvalidRole):
			httputil.WriteBadRequest(w, r, "BAD_REQUEST", err.Error())
		default:
			httputil.WriteInternalServerError(w, r, err)
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, map[string]string{"message": "member role updated successfully"})
}

// RemoveMember handles DELETE /api/v1/projects/{id}/members/{memberId}.
func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_PROJECT_ID", "Invalid project ID UUID")
		return
	}
	memberID := strings.TrimSpace(chi.URLParam(r, "memberId"))
	if memberID == "" {
		httputil.WriteBadRequest(w, r, "INVALID_MEMBER_ID", "Member ID is required")
		return
	}

	claims, err := auth.GetUserContext(r.Context())
	if err != nil || claims.UserID == "" {
		httputil.WriteUnauthorized(w, r, "UNAUTHORIZED", "Authentication required")
		return
	}

	if err := h.service.RemoveMember(r.Context(), projectID, claims.UserID, memberID); err != nil {
		switch {
		case errors.Is(err, ErrUnauthorizedAction):
			httputil.WriteForbidden(w, r, "FORBIDDEN", err.Error())
		case errors.Is(err, ErrMemberNotFound):
			httputil.WriteNotFound(w, r, "MEMBER_NOT_FOUND", err.Error())
		case errors.Is(err, ErrCannotRemoveLastLead):
			httputil.WriteBadRequest(w, r, "BAD_REQUEST", err.Error())
		default:
			httputil.WriteInternalServerError(w, r, err)
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, map[string]string{"message": "member removed successfully"})
}

// InviteMember handles POST /api/v1/projects/{id}/invitations.
func (h *Handler) InviteMember(w http.ResponseWriter, r *http.Request) {
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

	var input InviteMemberInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_REQUEST_BODY", "Failed to parse JSON request body")
		return
	}

	inv, err := h.service.InviteMember(r.Context(), projectID, claims.UserID, input.InviteeID, input.ProposedRole)
	if err != nil {
		switch {
		case errors.Is(err, ErrUnauthorizedAction):
			httputil.WriteForbidden(w, r, "FORBIDDEN", err.Error())
		case errors.Is(err, ErrSelfInvitation), errors.Is(err, ErrInvalidRole):
			httputil.WriteBadRequest(w, r, "BAD_REQUEST", err.Error())
		case errors.Is(err, ErrAlreadyMember), errors.Is(err, ErrDuplicateInvitation):
			httputil.WriteConflict(w, r, "CONFLICT", err.Error())
		default:
			httputil.WriteInternalServerError(w, r, err)
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusCreated, inv)
}

// AcceptInvitation handles POST /api/v1/invitations/{id}/accept.
func (h *Handler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	invitationID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_INVITATION_ID", "Invalid invitation ID UUID")
		return
	}

	claims, err := auth.GetUserContext(r.Context())
	if err != nil || claims.UserID == "" {
		httputil.WriteUnauthorized(w, r, "UNAUTHORIZED", "Authentication required")
		return
	}

	if err := h.service.AcceptInvitation(r.Context(), invitationID, claims.UserID); err != nil {
		switch {
		case errors.Is(err, ErrInvitationNotFound):
			httputil.WriteNotFound(w, r, "NOT_FOUND", err.Error())
		case errors.Is(err, ErrUnauthorizedAction):
			httputil.WriteForbidden(w, r, "FORBIDDEN", err.Error())
		case errors.Is(err, ErrInvitationExpired):
			httputil.WriteError(w, r, http.StatusGone, "INVITATION_EXPIRED", err.Error(), nil)
		case errors.Is(err, ErrInvitationNotPending):
			httputil.WriteBadRequest(w, r, "BAD_REQUEST", err.Error())
		case errors.Is(err, ErrAlreadyMember):
			httputil.WriteConflict(w, r, "ALREADY_MEMBER", err.Error())
		default:
			httputil.WriteInternalServerError(w, r, err)
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, map[string]string{"message": "invitation accepted successfully"})
}

// TransferLeadership handles POST /api/v1/projects/{id}/transfer-leadership.
func (h *Handler) TransferLeadership(w http.ResponseWriter, r *http.Request) {
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

	var input TransferLeadershipInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteBadRequest(w, r, "INVALID_REQUEST_BODY", "Failed to parse JSON request body")
		return
	}

	if err := h.service.TransferLeadership(r.Context(), projectID, claims.UserID, input.NewLeadID); err != nil {
		switch {
		case errors.Is(err, ErrUnauthorizedAction):
			httputil.WriteForbidden(w, r, "FORBIDDEN", err.Error())
		case errors.Is(err, ErrMemberNotFound):
			httputil.WriteNotFound(w, r, "MEMBER_NOT_FOUND", err.Error())
		case errors.Is(err, ErrInvalidLeadership):
			httputil.WriteBadRequest(w, r, "BAD_REQUEST", err.Error())
		default:
			httputil.WriteInternalServerError(w, r, err)
		}
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, map[string]string{"message": "leadership transferred successfully"})
}
