package contract

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
	"github.com/lynk/backend/internal/httpx"
	"github.com/lynk/backend/internal/httputil"
)

// Handler handles HTTP requests for contracts and their state transitions.
type Handler struct {
	service *Service
}

// NewHandler creates a new contract handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Routes constructs a chi.Router mounting all contract routes.
func (h *Handler) Routes(authMiddleware func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()

	routeSetup := func(rt chi.Router) {
		rt.Get("/", h.ListContracts)
		rt.Get("/contracts", h.ListContracts)
		rt.Get("/api/v1/contracts", h.ListContracts)

		rt.Get("/{id}", h.GetContractByID)
		rt.Get("/contracts/{id}", h.GetContractByID)
		rt.Get("/api/v1/contracts/{id}", h.GetContractByID)

		rt.Patch("/{id}/status", h.UpdateContractStatus)
		rt.Patch("/contracts/{id}/status", h.UpdateContractStatus)
		rt.Patch("/api/v1/contracts/{id}/status", h.UpdateContractStatus)
	}

	authMiddleware = httpx.DefaultAuthMiddleware(authMiddleware)
	r.Group(func(pr chi.Router) {
		pr.Use(authMiddleware)
		routeSetup(pr)
	})

	return r
}

// ContractRoutes provides a subrouter suitable for mounting at /api/v1/contracts.
func (h *Handler) ContractRoutes(authMiddleware func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Use(httpx.DefaultAuthMiddleware(authMiddleware))
	r.Get("/", h.ListContracts)
	r.Get("/{id}", h.GetContractByID)
	r.Patch("/{id}/status", h.UpdateContractStatus)
	return r
}

// ListContracts handles GET /api/v1/contracts (Lists contracts involving authenticated caller).
func (h *Handler) ListContracts(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials", nil)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	contracts, err := h.service.ListContracts(r.Context(), claims, limit, offset)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			httputil.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error(), err)
			return
		}
		httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve contracts", err)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, contracts)
}

// GetContractByID handles GET /api/v1/contracts/{id} (Participant student, employer, or admin).
func (h *Handler) GetContractByID(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials", nil)
		return
	}

	contractID, err := extractContractID(r)
	if err != nil {
		httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "Invalid contract UUID", err)
		return
	}

	contract, err := h.service.GetContractByID(r.Context(), claims, contractID)
	if err != nil {
		if errors.Is(err, ErrContractNotFound) {
			httputil.WriteError(w, r, http.StatusNotFound, "CONTRACT_NOT_FOUND", "Contract not found", nil)
			return
		}
		if errors.Is(err, ErrForbidden) {
			httputil.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error(), err)
			return
		}
		httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve contract", err)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, contract)
}

// UpdateContractStatus handles PATCH /api/v1/contracts/{id}/status (Participant updates status).
func (h *Handler) UpdateContractStatus(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		httputil.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials", nil)
		return
	}

	contractID, err := extractContractID(r)
	if err != nil {
		httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "Invalid contract UUID", err)
		return
	}

	var req UpdateContractStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			httputil.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Request body cannot be empty", err)
			return
		}
		httputil.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON request body", err)
		return
	}

	if strings.TrimSpace(req.Status) == "" {
		httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_INPUT", "Status is required", nil)
		return
	}

	updated, err := h.service.UpdateContractStatus(r.Context(), claims, contractID, req)
	if err != nil {
		if errors.Is(err, auth.ErrEmailNotVerified) {
			httputil.WriteError(w, r, http.StatusForbidden, "EMAIL_NOT_VERIFIED", auth.CampusVerificationPendingMsg, nil)
			return
		}
		if errors.Is(err, ErrContractNotFound) {
			httputil.WriteError(w, r, http.StatusNotFound, "CONTRACT_NOT_FOUND", "Contract not found", nil)
			return
		}
		if errors.Is(err, ErrForbidden) {
			httputil.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		if errors.Is(err, ErrTerminalStatus) || errors.Is(err, ErrInvalidTransition) {
			httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_TRANSITION", err.Error(), err)
			return
		}
		if errors.Is(err, ErrInvalidStatus) {
			httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_STATUS", err.Error(), err)
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			httputil.WriteError(w, r, http.StatusBadRequest, "INVALID_INPUT", err.Error(), err)
			return
		}
		httputil.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update contract status", err)
		return
	}

	httputil.WriteSuccess(w, http.StatusOK, updated)
}

func extractContractID(r *http.Request) (uuid.UUID, error) {
	if val := chi.URLParam(r, "id"); val != "" {
		if id, err := uuid.Parse(val); err == nil {
			return id, nil
		}
	}
	if val := chi.URLParam(r, "contractID"); val != "" {
		if id, err := uuid.Parse(val); err == nil {
			return id, nil
		}
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for i, p := range parts {
		if (p == "contracts" || p == "contract") && i+1 < len(parts) && parts[i+1] != "status" {
			return uuid.Parse(parts[i+1])
		}
	}
	return uuid.Nil, errors.New("invalid contract id in url")
}
