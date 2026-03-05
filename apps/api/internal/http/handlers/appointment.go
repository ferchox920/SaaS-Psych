package handlers

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	domainappointment "sessionflow/apps/api/internal/domain/appointment"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	appointmentusecase "sessionflow/apps/api/internal/usecase/appointment"
)

type AppointmentHandler struct {
	service *appointmentusecase.Service
}

func NewAppointmentHandler(service *appointmentusecase.Service) *AppointmentHandler {
	return &AppointmentHandler{service: service}
}

type createAppointmentRequest struct {
	ClientID string `json:"client_id"`
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
	Location string `json:"location"`
}

type updateAppointmentRequest struct {
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
	Location string `json:"location"`
}

type appointmentResponse struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	ClientID  string `json:"client_id"`
	StartsAt  string `json:"starts_at"`
	EndsAt    string `json:"ends_at"`
	Status    string `json:"status"`
	Location  string `json:"location"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func (h *AppointmentHandler) Create(c echo.Context) error {
	if h.service == nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "service_unavailable", "appointment service unavailable")
	}
	tenantID, ok := httpmiddleware.TenantIDFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "tenant context missing")
	}
	principal, ok := httpmiddleware.PrincipalFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "auth context missing")
	}

	var req createAppointmentRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body", map[string]any{"field": "body"})
	}

	clientID, err := uuid.Parse(req.ClientID)
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "client_id must be a valid uuid", map[string]any{"field": "client_id"})
	}
	startsAt, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "starts_at must be RFC3339", map[string]any{"field": "starts_at"})
	}
	endsAt, err := time.Parse(time.RFC3339, req.EndsAt)
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "ends_at must be RFC3339", map[string]any{"field": "ends_at"})
	}

	out, err := h.service.Create(c.Request().Context(), appointmentusecase.CreateInput{
		TenantID:    tenantID,
		ClientID:    clientID,
		ActorUserID: principal.UserID,
		StartsAt:    startsAt,
		EndsAt:      endsAt,
		Location:    req.Location,
	})
	if err != nil {
		return h.handleAppointmentError(c, err)
	}

	return c.JSON(http.StatusCreated, toAppointmentResponse(out))
}

func (h *AppointmentHandler) List(c echo.Context) error {
	if h.service == nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "service_unavailable", "appointment service unavailable")
	}
	tenantID, ok := httpmiddleware.TenantIDFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "tenant context missing")
	}

	from, err := time.Parse(time.RFC3339, c.QueryParam("from"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "from must be RFC3339", map[string]any{"field": "from"})
	}
	to, err := time.Parse(time.RFC3339, c.QueryParam("to"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "to must be RFC3339", map[string]any{"field": "to"})
	}

	items, err := h.service.ListByRange(c.Request().Context(), appointmentusecase.ListInput{
		TenantID: tenantID,
		From:     from,
		To:       to,
	})
	if err != nil {
		return h.handleAppointmentError(c, err)
	}

	resp := make([]appointmentResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, toAppointmentResponse(item))
	}
	return c.JSON(http.StatusOK, map[string]any{"items": resp})
}

func (h *AppointmentHandler) Update(c echo.Context) error {
	if h.service == nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "service_unavailable", "appointment service unavailable")
	}
	tenantID, ok := httpmiddleware.TenantIDFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "tenant context missing")
	}
	principal, ok := httpmiddleware.PrincipalFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "auth context missing")
	}

	appointmentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "appointment id must be a valid uuid", map[string]any{"field": "id"})
	}

	var req updateAppointmentRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body", map[string]any{"field": "body"})
	}
	startsAt, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "starts_at must be RFC3339", map[string]any{"field": "starts_at"})
	}
	endsAt, err := time.Parse(time.RFC3339, req.EndsAt)
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "ends_at must be RFC3339", map[string]any{"field": "ends_at"})
	}

	out, err := h.service.Update(c.Request().Context(), appointmentusecase.UpdateInput{
		TenantID:      tenantID,
		AppointmentID: appointmentID,
		ActorUserID:   principal.UserID,
		StartsAt:      startsAt,
		EndsAt:        endsAt,
		Location:      req.Location,
	})
	if err != nil {
		return h.handleAppointmentError(c, err)
	}
	return c.JSON(http.StatusOK, toAppointmentResponse(out))
}

func (h *AppointmentHandler) Cancel(c echo.Context) error {
	if h.service == nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "service_unavailable", "appointment service unavailable")
	}
	tenantID, ok := httpmiddleware.TenantIDFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "tenant context missing")
	}
	principal, ok := httpmiddleware.PrincipalFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "auth context missing")
	}
	appointmentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "appointment id must be a valid uuid", map[string]any{"field": "id"})
	}

	out, err := h.service.Cancel(c.Request().Context(), tenantID, appointmentID, principal.UserID)
	if err != nil {
		return h.handleAppointmentError(c, err)
	}
	return c.JSON(http.StatusOK, toAppointmentResponse(out))
}

func (h *AppointmentHandler) handleAppointmentError(c echo.Context, err error) error {
	return handleDomainError(c, err, defaultAppointmentErrorMappings())
}

func defaultAppointmentErrorMappings() []domainErrorMapping {
	return []domainErrorMapping{
		{Target: domainerrors.ErrValidation, Status: http.StatusBadRequest, Code: "validation_error"},
		{Target: domainerrors.ErrNotFound, Status: http.StatusNotFound, Code: "not_found", Message: "appointment or client not found"},
		{Target: domainerrors.ErrForbidden, Status: http.StatusForbidden, Code: "forbidden", Message: "client does not belong to tenant"},
		{Target: domainerrors.ErrConflict, Status: http.StatusConflict, Code: "conflict", Message: "appointment overlaps existing slot"},
	}
}

func toAppointmentResponse(entity domainappointment.Entity) appointmentResponse {
	return appointmentResponse{
		ID:        entity.ID.String(),
		TenantID:  entity.TenantID.String(),
		ClientID:  entity.ClientID.String(),
		StartsAt:  entity.StartsAt.UTC().Format(time.RFC3339),
		EndsAt:    entity.EndsAt.UTC().Format(time.RFC3339),
		Status:    entity.Status,
		Location:  entity.Location,
		CreatedAt: entity.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: entity.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
