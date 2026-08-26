package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	clinicalanalysis "sessionflow/apps/api/internal/usecase/clinicalanalysis"
)

type ClinicalAnalysisHandler struct {
	service *clinicalanalysis.Service
}

func NewClinicalAnalysisHandler(service *clinicalanalysis.Service) *ClinicalAnalysisHandler {
	return &ClinicalAnalysisHandler{service: service}
}

func (h *ClinicalAnalysisHandler) Status(c echo.Context) error {
	status, err := h.service.Status(c.Request().Context())
	if err != nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "local_model_unavailable", status.Message)
	}
	return c.JSON(http.StatusOK, status)
}

func (h *ClinicalAnalysisHandler) Models(c echo.Context) error {
	models, err := h.service.ListModels(c.Request().Context())
	if err != nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "local_model_unavailable", "Ollama local no está disponible")
	}
	return c.JSON(http.StatusOK, map[string]any{"items": models})
}

func (h *ClinicalAnalysisHandler) Warm(c echo.Context) error {
	if err := h.service.Warm(c.Request().Context()); err != nil {
		return h.writeProviderError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"status": "loaded"})
}

func (h *ClinicalAnalysisHandler) Unload(c echo.Context) error {
	if err := h.service.Unload(c.Request().Context()); err != nil {
		return h.writeProviderError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"status": "unloaded"})
}

type analyzeLiveRequest struct {
	AppointmentID        string                     `json:"appointment_id"`
	Fragment             string                     `json:"fragment"`
	PreviousIntervention string                     `json:"previous_intervention,omitempty"`
	PreviousAction       clinicalanalysis.NowAction `json:"previous_action,omitempty"`
	PatientResponse      string                     `json:"patient_response,omitempty"`
}

func (h *ClinicalAnalysisHandler) AnalyzeLive(c echo.Context) error {
	if h.service == nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "service_unavailable", "local clinical analysis unavailable")
	}
	tenantID, principal, ok := clinicalPrincipal(c)
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "clinical context missing")
	}
	var request analyzeLiveRequest
	if err := c.Bind(&request); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body")
	}
	appointmentID, err := uuid.Parse(request.AppointmentID)
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "appointment_id must be a valid uuid", map[string]any{"field": "appointment_id"})
	}

	response := c.Response()
	response.Header().Set(echo.HeaderContentType, "text/event-stream; charset=utf-8")
	response.Header().Set(echo.HeaderCacheControl, "no-cache, no-store")
	response.Header().Set("X-Accel-Buffering", "no")
	response.WriteHeader(http.StatusOK)
	_ = writeSSE(response, "status", map[string]any{"state": "validating", "message": "Validando acceso clínico"})

	output, err := h.service.AnalyzeLive(c.Request().Context(), clinicalanalysis.AnalyzeLiveInput{
		TenantID:      tenantID,
		ActorUserID:   principal.UserID,
		AppointmentID: appointmentID,
		Request: clinicalanalysis.LiveRequest{
			Fragment:             request.Fragment,
			PreviousIntervention: request.PreviousIntervention,
			PreviousAction:       request.PreviousAction,
			PatientResponse:      request.PatientResponse,
		},
	}, func(progress clinicalanalysis.GenerationProgress) {
		_ = writeSSE(response, "progress", map[string]any{
			"state":                "generating",
			"generated_characters": progress.GeneratedCharacters,
			"elapsed_ms":           progress.Elapsed.Milliseconds(),
		})
	})
	if err != nil {
		code, message := clinicalStreamError(err)
		_ = writeSSE(response, "error", map[string]any{"code": code, "message": message})
		return nil
	}
	_ = writeSSE(response, "result", output)
	_ = writeSSE(response, "done", map[string]any{"state": "complete"})
	return nil
}

type reviewSessionRequest struct {
	AppointmentID string `json:"appointment_id"`
	SessionText   string `json:"session_text"`
}

func (h *ClinicalAnalysisHandler) ReviewSession(c echo.Context) error {
	if h.service == nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "service_unavailable", "local clinical review unavailable")
	}
	tenantID, principal, ok := clinicalPrincipal(c)
	if !ok {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "clinical context missing")
	}
	var request reviewSessionRequest
	if err := c.Bind(&request); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body")
	}
	appointmentID, err := uuid.Parse(request.AppointmentID)
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "appointment_id must be a valid uuid", map[string]any{"field": "appointment_id"})
	}
	response := c.Response()
	response.Header().Set(echo.HeaderContentType, "text/event-stream; charset=utf-8")
	response.Header().Set(echo.HeaderCacheControl, "no-cache, no-store")
	response.Header().Set("X-Accel-Buffering", "no")
	response.WriteHeader(http.StatusOK)
	_ = writeSSE(response, "status", map[string]any{"state": "validating", "message": "Validando revisión clínica"})
	output, err := h.service.ReviewSession(c.Request().Context(), clinicalanalysis.ReviewSessionInput{TenantID: tenantID, ActorUserID: principal.UserID, AppointmentID: appointmentID, SessionText: request.SessionText}, func(progress clinicalanalysis.GenerationProgress) {
		_ = writeSSE(response, "progress", map[string]any{"state": "reviewing", "generated_characters": progress.GeneratedCharacters, "elapsed_ms": progress.Elapsed.Milliseconds()})
	})
	if err != nil {
		code, message := clinicalStreamError(err)
		_ = writeSSE(response, "error", map[string]any{"code": code, "message": message})
		return nil
	}
	_ = writeSSE(response, "result", output)
	_ = writeSSE(response, "done", map[string]any{"state": "complete"})
	return nil
}

func clinicalPrincipal(c echo.Context) (uuid.UUID, httpmiddleware.Principal, bool) {
	tenantID, tenantOK := httpmiddleware.TenantIDFromContext(c.Request().Context())
	principal, principalOK := httpmiddleware.PrincipalFromContext(c.Request().Context())
	return tenantID, principal, tenantOK && principalOK
}

func writeSSE(response *echo.Response, event string, payload any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(response, "event: %s\ndata: %s\n\n", event, encoded); err != nil {
		return err
	}
	response.Flush()
	return nil
}

func clinicalStreamError(err error) (string, string) {
	switch {
	case errors.Is(err, domainerrors.ErrForbidden):
		return "clinical_assignment_required", "Se requiere una asignación tratante activa para analizar esta sesión."
	case errors.Is(err, clinicalanalysis.ErrProviderBusy):
		return "local_model_busy", "El modelo local está ocupado; espera o cancela la generación activa."
	case errors.Is(err, clinicalanalysis.ErrProviderUnavailable):
		return "local_model_unavailable", "Ollama o el modelo configurado no están disponibles localmente."
	case errors.Is(err, clinicalanalysis.ErrInvalidModelOutput):
		return "invalid_model_output", "La salida local no superó la validación clínica y fue descartada."
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "generation_canceled", "La generación local fue cancelada o excedió el tiempo límite."
	default:
		return "analysis_failed", "No fue posible completar el análisis local."
	}
}

func (h *ClinicalAnalysisHandler) writeProviderError(c echo.Context, err error) error {
	if errors.Is(err, clinicalanalysis.ErrProviderBusy) {
		return writeAPIError(c, http.StatusConflict, "local_model_busy", "El modelo local está ocupado")
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return writeAPIError(c, http.StatusGatewayTimeout, "local_model_timeout", "La operación local fue cancelada o excedió el tiempo límite")
	}
	return writeAPIError(c, http.StatusServiceUnavailable, "local_model_unavailable", "Ollama o el modelo configurado no están disponibles localmente")
}
