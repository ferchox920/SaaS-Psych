package handlers

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	clinicalsession "sessionflow/apps/api/internal/usecase/clinicalsession"
)

type ClinicalSessionHandler struct{ service *clinicalsession.Service }

func NewClinicalSessionHandler(service *clinicalsession.Service) *ClinicalSessionHandler {
	return &ClinicalSessionHandler{service: service}
}

type createClinicalSessionRequest struct {
	ClientID      string    `json:"client_id"`
	AppointmentID *string   `json:"appointment_id"`
	StartedAt     time.Time `json:"started_at"`
}

func (h *ClinicalSessionHandler) Create(c echo.Context) error {
	tenantID, principal, err := tenantAndPrincipal(c)
	if err != nil {
		return err
	}
	var req createClinicalSessionRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body")
	}
	clientID, err := uuid.Parse(req.ClientID)
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "client_id must be a valid uuid")
	}
	var appointmentID *uuid.UUID
	if req.AppointmentID != nil {
		parsed, parseErr := uuid.Parse(*req.AppointmentID)
		if parseErr != nil {
			return writeAPIError(c, http.StatusBadRequest, "validation_error", "appointment_id must be a valid uuid")
		}
		appointmentID = &parsed
	}
	item, err := h.service.Create(c.Request().Context(), clinicalsession.CreateInput{TenantID: tenantID, ClientID: clientID, TherapistUserID: principal.UserID, AppointmentID: appointmentID, StartedAt: req.StartedAt})
	if err != nil {
		return handleClinicalSessionError(c, err)
	}
	return c.JSON(http.StatusCreated, item)
}
func (h *ClinicalSessionHandler) Get(c echo.Context) error {
	tenantID, principal, id, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	item, err := h.service.Get(c.Request().Context(), tenantID, id, principal.UserID)
	if err != nil {
		return handleClinicalSessionError(c, err)
	}
	return c.JSON(http.StatusOK, item)
}
func (h *ClinicalSessionHandler) ListByClient(c echo.Context) error {
	tenantID, principal, clientID, err := clinicalSessionContext(c, "client_id")
	if err != nil {
		return err
	}
	items, err := h.service.ListByClient(c.Request().Context(), tenantID, clientID, principal.UserID)
	if err != nil {
		return handleClinicalSessionError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}
func (h *ClinicalSessionHandler) Complete(c echo.Context) error { return h.transition(c, true) }
func (h *ClinicalSessionHandler) Void(c echo.Context) error     { return h.transition(c, false) }
func (h *ClinicalSessionHandler) transition(c echo.Context, complete bool) error {
	tenantID, principal, id, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	var item any
	if complete {
		item, err = h.service.Complete(c.Request().Context(), tenantID, id, principal.UserID)
	} else {
		item, err = h.service.Void(c.Request().Context(), tenantID, id, principal.UserID)
	}
	if err != nil {
		return handleClinicalSessionError(c, err)
	}
	return c.JSON(http.StatusOK, item)
}
func clinicalSessionContext(c echo.Context, param string) (uuid.UUID, httpmiddleware.Principal, uuid.UUID, error) {
	tenantID, principal, err := tenantAndPrincipal(c)
	if err != nil {
		return uuid.Nil, httpmiddleware.Principal{}, uuid.Nil, err
	}
	id, parseErr := uuid.Parse(c.Param(param))
	if parseErr != nil {
		return uuid.Nil, httpmiddleware.Principal{}, uuid.Nil, writeAPIError(c, http.StatusBadRequest, "validation_error", param+" must be a valid uuid")
	}
	return tenantID, principal, id, nil
}

func handleClinicalSessionError(c echo.Context, err error) error {
	return handleDomainError(c, err, []domainErrorMapping{
		{Target: domainerrors.ErrValidation, Status: http.StatusBadRequest, Code: "validation_error"},
		{Target: domainerrors.ErrNotFound, Status: http.StatusNotFound, Code: "not_found", Message: "clinical session not found"},
		{Target: domainerrors.ErrForbidden, Status: http.StatusForbidden, Code: "forbidden", Message: "clinical treating assignment required"},
		{Target: domainerrors.ErrConflict, Status: http.StatusConflict, Code: "conflict", Message: "clinical session cannot change in its current state"},
	})
}
