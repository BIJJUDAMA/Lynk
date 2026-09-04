package contract

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

// ContractRoutes provides a subrouter suitable for mounting at /api/v1/contracts.
func (h *Handler) ContractRoutes(authMiddleware func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	if authMiddleware != nil {
		r.Use(authMiddleware)
	}
	r.Get("/", h.ListContracts)
	r.Get("/{id}", h.GetContractByID)
	r.Patch("/{id}/status", h.UpdateContractStatus)
	return r
}

// ListContracts handles GET /api/v1/contracts (Lists contracts involving authenticated caller).
func (h *Handler) ListContracts(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
		return
	}

	contracts, err := h.service.ListContracts(r.Context(), claims)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve contracts")
		return
	}

	writeJSONSuccess(w, http.StatusOK, contracts)
}

// GetContractByID handles GET /api/v1/contracts/{id} (Participant student, employer, or admin).
func (h *Handler) GetContractByID(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
		return
	}

	contractID, err := extractContractID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Invalid contract UUID")
		return
	}

	contract, err := h.service.GetContractByID(r.Context(), claims, contractID)
	if err != nil {
		if errors.Is(err, ErrContractNotFound) {
			writeJSONError(w, http.StatusNotFound, "CONTRACT_NOT_FOUND", "Contract not found")
			return
		}
		if errors.Is(err, ErrForbidden) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve contract")
		return
	}

	writeJSONSuccess(w, http.StatusOK, contract)
}

// UpdateContractStatus handles PATCH /api/v1/contracts/{id}/status (Participant updates status).
func (h *Handler) UpdateContractStatus(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.GetUserContext(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing credentials")
		return
	}

	contractID, err := extractContractID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Invalid contract UUID")
		return
	}

	var req UpdateContractStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Request body cannot be empty")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON request body")
		return
	}

	if strings.TrimSpace(req.Status) == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_INPUT", "Status is required")
		return
	}

	updated, err := h.service.UpdateContractStatus(r.Context(), claims, contractID, req)
	if err != nil {
		if errors.Is(err, ErrContractNotFound) {
			writeJSONError(w, http.StatusNotFound, "CONTRACT_NOT_FOUND", "Contract not found")
			return
		}
		if errors.Is(err, ErrForbidden) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		if errors.Is(err, ErrTerminalStatus) || errors.Is(err, ErrInvalidTransition) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_TRANSITION", err.Error())
			return
		}
		if errors.Is(err, ErrInvalidStatus) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_STATUS", err.Error())
			return
		}
		if errors.Is(err, ErrInvalidInput) {
			writeJSONError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update contract status")
		return
	}

	writeJSONSuccess(w, http.StatusOK, updated)
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
