package handlers

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	calendarusecase "sessionflow/apps/api/internal/usecase/googlecalendar"
)

type GoogleCalendarHandler struct {
	service   *calendarusecase.Service
	webOrigin string
}

func NewGoogleCalendarHandler(service *calendarusecase.Service, webOrigin string) *GoogleCalendarHandler {
	return &GoogleCalendarHandler{service: service, webOrigin: strings.TrimRight(webOrigin, "/")}
}

func (h *GoogleCalendarHandler) Status(c echo.Context) error {
	tenantID, userID, ok := calendarPrincipal(c)
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "calendar context missing")
	}
	status, err := h.service.Status(c.Request().Context(), tenantID, userID)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(http.StatusOK, status)
}

func (h *GoogleCalendarHandler) BeginAuthorization(c echo.Context) error {
	tenantID, userID, ok := calendarPrincipal(c)
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "calendar context missing")
	}
	authorizationURL, err := h.service.BeginAuthorization(c.Request().Context(), tenantID, userID)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"authorization_url": authorizationURL})
}

func (h *GoogleCalendarHandler) Callback(c echo.Context) error {
	result := "connected"
	if c.QueryParam("error") != "" {
		result = "denied"
	} else if _, _, err := h.service.CompleteAuthorization(c.Request().Context(), c.QueryParam("state"), c.QueryParam("code")); err != nil {
		result = "error"
	}
	target := h.webOrigin + "/appointments?google_calendar=" + url.QueryEscape(result)
	return c.Redirect(http.StatusFound, target)
}

func (h *GoogleCalendarHandler) ListCandidates(c echo.Context) error {
	tenantID, userID, ok := calendarPrincipal(c)
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "calendar context missing")
	}
	from, err := time.Parse(time.RFC3339, c.QueryParam("from"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "from must be RFC3339")
	}
	to, err := time.Parse(time.RFC3339, c.QueryParam("to"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "to must be RFC3339")
	}
	items, err := h.service.ListCandidates(c.Request().Context(), tenantID, userID, from, to)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *GoogleCalendarHandler) ImportEvent(c echo.Context) error {
	tenantID, userID, ok := calendarPrincipal(c)
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "calendar context missing")
	}
	var request struct {
		ClientID string `json:"client_id"`
	}
	if err := c.Bind(&request); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body")
	}
	clientID, err := uuid.Parse(request.ClientID)
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "client_id must be a valid uuid")
	}
	appointment, err := h.service.ImportEvent(c.Request().Context(), tenantID, userID, clientID, c.Param("event_id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(http.StatusCreated, toAppointmentResponse(appointment))
}

func (h *GoogleCalendarHandler) PushAppointment(c echo.Context) error {
	tenantID, userID, ok := calendarPrincipal(c)
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "calendar context missing")
	}
	appointmentID, err := uuid.Parse(c.Param("appointment_id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "appointment_id must be a valid uuid")
	}
	link, err := h.service.PushAppointment(c.Request().Context(), tenantID, userID, appointmentID)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"appointment_id": link.AppointmentID, "sync_status": link.SyncStatus, "last_synced_at": link.LastSyncedAt})
}

func (h *GoogleCalendarHandler) Disconnect(c echo.Context) error {
	tenantID, userID, ok := calendarPrincipal(c)
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "calendar context missing")
	}
	if err := h.service.Disconnect(c.Request().Context(), tenantID, userID); err != nil {
		return h.handleError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *GoogleCalendarHandler) handleError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, calendarusecase.ErrDisabled):
		return writeAPIError(c, http.StatusServiceUnavailable, "google_calendar_disabled", "Google Calendar no está configurado")
	case errors.Is(err, calendarusecase.ErrNotConnected):
		return writeAPIError(c, http.StatusConflict, "google_calendar_not_connected", "Conecta Google Calendar primero")
	case errors.Is(err, calendarusecase.ErrReauthorizationRequired):
		return writeAPIError(c, http.StatusUnauthorized, "google_calendar_reauthorization_required", "Google Calendar requiere nueva autorización")
	case errors.Is(err, domainerrors.ErrForbidden):
		return writeAPIError(c, http.StatusForbidden, "forbidden", "No tienes acceso al paciente asociado")
	case errors.Is(err, domainerrors.ErrConflict):
		return writeAPIError(c, http.StatusConflict, "conflict", "El evento se superpone con un turno existente")
	case errors.Is(err, domainerrors.ErrNotFound):
		return writeAPIError(c, http.StatusNotFound, "not_found", "No se encontró el recurso solicitado")
	default:
		var validation *domainerrors.ValidationError
		if errors.As(err, &validation) {
			return writeAPIError(c, http.StatusBadRequest, "validation_error", validation.Error())
		}
		return writeAPIError(c, http.StatusBadGateway, "google_calendar_error", "No se pudo completar la operación con Google Calendar")
	}
}

func calendarPrincipal(c echo.Context) (uuid.UUID, uuid.UUID, bool) {
	tenantID, ok := httpmiddleware.TenantIDFromContext(c.Request().Context())
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	principal, ok := httpmiddleware.PrincipalFromContext(c.Request().Context())
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	return tenantID, principal.UserID, true
}
