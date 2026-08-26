package handlers

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	clinicalmemory "sessionflow/apps/api/internal/usecase/clinicalmemory"
)

type ClinicalMemoryHandler struct{ service *clinicalmemory.Service }

func NewClinicalMemoryHandler(service *clinicalmemory.Service) *ClinicalMemoryHandler {
	return &ClinicalMemoryHandler{service: service}
}

func (h *ClinicalMemoryHandler) ListSuggestions(c echo.Context) error {
	tenantID, principal, ok := clinicalPrincipal(c)
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "clinical context missing")
	}
	appointmentID, err := uuid.Parse(c.Param("appointment_id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "appointment_id must be a valid uuid")
	}
	items, err := h.service.ListSuggestions(c.Request().Context(), tenantID, appointmentID, principal.UserID)
	if err != nil {
		return handleClinicalMemoryError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

type decideSuggestionRequest struct {
	Disposition    string `json:"disposition"`
	CorrectionText string `json:"correction_text"`
	Reason         string `json:"reason"`
}

func (h *ClinicalMemoryHandler) DecideSuggestion(c echo.Context) error {
	tenantID, principal, ok := clinicalPrincipal(c)
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "clinical context missing")
	}
	suggestionID, err := uuid.Parse(c.Param("suggestion_id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "suggestion_id must be a valid uuid")
	}
	var request decideSuggestionRequest
	if err := c.Bind(&request); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body")
	}
	item, err := h.service.DecideSuggestion(c.Request().Context(), tenantID, suggestionID, principal.UserID, request.Disposition, request.CorrectionText, request.Reason)
	if err != nil {
		return handleClinicalMemoryError(c, err)
	}
	return c.JSON(http.StatusOK, item)
}

type createSnapshotRequest struct {
	ApprovedSummary string                  `json:"approved_summary"`
	Anchors         []clinicalmemory.Anchor `json:"anchors"`
}

func (h *ClinicalMemoryHandler) CreateSnapshot(c echo.Context) error {
	tenantID, principal, clientID, ok := formulationContext(c)
	if !ok {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "client_id must be a valid uuid")
	}
	var request createSnapshotRequest
	if err := c.Bind(&request); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body")
	}
	item, err := h.service.CreateSnapshot(c.Request().Context(), tenantID, clientID, principal.UserID, request.ApprovedSummary, request.Anchors)
	if err != nil {
		return handleClinicalMemoryError(c, err)
	}
	return c.JSON(http.StatusCreated, item)
}

func (h *ClinicalMemoryHandler) ListSnapshots(c echo.Context) error {
	tenantID, principal, clientID, ok := formulationContext(c)
	if !ok {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "client_id must be a valid uuid")
	}
	items, err := h.service.ListSnapshots(c.Request().Context(), tenantID, clientID, principal.UserID)
	if err != nil {
		return handleClinicalMemoryError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *ClinicalMemoryHandler) ApproveSnapshot(c echo.Context) error {
	tenantID, principal, clientID, ok := formulationContext(c)
	if !ok {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "client_id must be a valid uuid")
	}
	snapshotID, err := uuid.Parse(c.Param("snapshot_id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "snapshot_id must be a valid uuid")
	}
	item, err := h.service.ApproveSnapshot(c.Request().Context(), tenantID, clientID, snapshotID, principal.UserID)
	if err != nil {
		return handleClinicalMemoryError(c, err)
	}
	return c.JSON(http.StatusOK, item)
}

func formulationContext(c echo.Context) (uuid.UUID, httpmiddleware.Principal, uuid.UUID, bool) {
	tenantID, principal, ok := clinicalPrincipal(c)
	if !ok {
		return uuid.Nil, httpmiddleware.Principal{}, uuid.Nil, false
	}
	clientID, err := uuid.Parse(c.Param("client_id"))
	return tenantID, principal, clientID, err == nil
}

func handleClinicalMemoryError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, domainerrors.ErrForbidden):
		return writeAPIError(c, http.StatusForbidden, "clinical_assignment_required", "clinical assignment required")
	case errors.Is(err, domainerrors.ErrNotFound):
		return writeAPIError(c, http.StatusNotFound, "not_found", "clinical memory item not found")
	case errors.Is(err, domainerrors.ErrConflict):
		return writeAPIError(c, http.StatusConflict, "conflict", "clinical memory item cannot be changed in its current state")
	default:
		var validation *domainerrors.ValidationError
		if errors.As(err, &validation) {
			return writeAPIError(c, http.StatusBadRequest, "validation_error", validation.Error())
		}
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "clinical memory operation failed")
	}
}
