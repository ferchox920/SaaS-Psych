package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	transcription "sessionflow/apps/api/internal/usecase/transcription"
)

type TranscriptionHandler struct {
	service  *transcription.Service
	maxBytes int64
}

func NewTranscriptionHandler(service *transcription.Service, maxBytes int64) *TranscriptionHandler {
	return &TranscriptionHandler{service: service, maxBytes: maxBytes}
}

func (h *TranscriptionHandler) Status(c echo.Context) error {
	status, err := h.service.Status(c.Request().Context())
	if err != nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "transcriber_unavailable", "El transcriptor local no está disponible")
	}
	return c.JSON(http.StatusOK, status)
}

func (h *TranscriptionHandler) Transcribe(c echo.Context) error {
	tenantID, ok := httpmiddleware.TenantIDFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "tenant context missing")
	}
	principal, ok := httpmiddleware.PrincipalFromContext(c.Request().Context())
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "auth context missing")
	}
	appointmentID, err := uuid.Parse(c.Param("appointment_id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "appointment_id must be a valid uuid")
	}
	consent := strings.EqualFold(strings.TrimSpace(c.Request().Header.Get("X-Clinical-Audio-Consent")), "true")
	format, ok := audioFormat(c.Request().Header.Get(echo.HeaderContentType))
	if !ok {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "unsupported audio content type")
	}
	limited := io.LimitReader(c.Request().Body, h.maxBytes+1)
	audio, err := io.ReadAll(limited)
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "invalid_audio", "could not read audio")
	}
	if int64(len(audio)) > h.maxBytes {
		return writeAPIError(c, http.StatusRequestEntityTooLarge, "audio_too_large", "audio exceeds the ephemeral size limit")
	}
	result, err := h.service.Transcribe(c.Request().Context(), transcription.TranscribeInput{
		TenantID: tenantID, ActorUserID: principal.UserID, AppointmentID: appointmentID,
		ExplicitConsent: consent, Format: format, Audio: audio,
	})
	if err != nil {
		return handleTranscriptionError(c, err)
	}
	return c.JSON(http.StatusOK, result)
}

func audioFormat(contentType string) (string, bool) {
	base := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch base {
	case "audio/wav", "audio/wave", "audio/x-wav":
		return "wav", true
	case "audio/webm":
		return "webm", true
	case "audio/ogg":
		return "ogg", true
	case "audio/mp4":
		return "mp4", true
	case "audio/m4a", "audio/x-m4a":
		return "m4a", true
	default:
		return "", false
	}
}

func handleTranscriptionError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, transcription.ErrDisabled):
		return writeAPIError(c, http.StatusServiceUnavailable, "transcription_disabled", "La captura de audio local está deshabilitada por configuración")
	case errors.Is(err, transcription.ErrBusy):
		return writeAPIError(c, http.StatusConflict, "transcriber_busy", "El transcriptor local está ocupado")
	case errors.Is(err, transcription.ErrUnavailable):
		return writeAPIError(c, http.StatusServiceUnavailable, "transcriber_unavailable", "El transcriptor local no está disponible")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return writeAPIError(c, http.StatusGatewayTimeout, "transcription_canceled", "La transcripción fue cancelada o excedió el tiempo límite")
	case errors.Is(err, domainerrors.ErrForbidden):
		return writeAPIError(c, http.StatusForbidden, "clinical_assignment_required", "Se requiere asignación tratante activa")
	default:
		var validation *domainerrors.ValidationError
		if errors.As(err, &validation) {
			return writeAPIError(c, http.StatusBadRequest, "validation_error", validation.Error())
		}
		return writeAPIError(c, http.StatusInternalServerError, "transcription_failed", "No fue posible completar la transcripción local")
	}
}
