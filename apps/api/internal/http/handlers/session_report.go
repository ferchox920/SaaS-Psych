package handlers

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	sessionreport "sessionflow/apps/api/internal/usecase/sessionreport"
)

type SessionReportHandler struct{ service *sessionreport.Service }

func NewSessionReportHandler(service *sessionreport.Service) *SessionReportHandler {
	return &SessionReportHandler{service: service}
}

type generateSessionReportRequest struct {
	SessionText string `json:"session_text"`
}
type updateSessionReportRequest struct {
	ExpectedRevision int                    `json:"expected_revision"`
	Report           sessionreport.ReportV1 `json:"report"`
}
type approveSessionReportRequest struct {
	ExpectedRevision int `json:"expected_revision"`
}

func (h *SessionReportHandler) Generate(c echo.Context) error {
	tenantID, principal, sessionID, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	var req generateSessionReportRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body")
	}
	item, err := h.service.Generate(c.Request().Context(), sessionreport.GenerateInput{TenantID: tenantID, SessionID: sessionID, ActorUserID: principal.UserID, SessionText: req.SessionText})
	if err != nil {
		return handleSessionReportError(c, err)
	}
	return c.JSON(http.StatusCreated, item)
}
func (h *SessionReportHandler) List(c echo.Context) error {
	tenantID, principal, sessionID, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	items, err := h.service.List(c.Request().Context(), tenantID, sessionID, principal.UserID)
	if err != nil {
		return handleSessionReportError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}
func (h *SessionReportHandler) Get(c echo.Context) error {
	tenantID, principal, err := tenantAndPrincipal(c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("report_id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "report_id must be a valid uuid")
	}
	item, err := h.service.Get(c.Request().Context(), tenantID, id, principal.UserID)
	if err != nil {
		return handleSessionReportError(c, err)
	}
	return c.JSON(http.StatusOK, item)
}
func (h *SessionReportHandler) Update(c echo.Context) error {
	tenantID, principal, err := tenantAndPrincipal(c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("report_id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "report_id must be a valid uuid")
	}
	var req updateSessionReportRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body")
	}
	item, err := h.service.Update(c.Request().Context(), sessionreport.UpdateInput{TenantID: tenantID, ReportID: id, ActorUserID: principal.UserID, ExpectedRevision: req.ExpectedRevision, Report: req.Report})
	if err != nil {
		return handleSessionReportError(c, err)
	}
	return c.JSON(http.StatusOK, item)
}
func (h *SessionReportHandler) Approve(c echo.Context) error {
	tenantID, principal, err := tenantAndPrincipal(c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("report_id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "report_id must be a valid uuid")
	}
	var req approveSessionReportRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body")
	}
	item, err := h.service.Approve(c.Request().Context(), tenantID, id, principal.UserID, req.ExpectedRevision)
	if err != nil {
		return handleSessionReportError(c, err)
	}
	return c.JSON(http.StatusOK, item)
}
func handleSessionReportError(c echo.Context, err error) error {
	return handleDomainError(c, err, []domainErrorMapping{{Target: domainerrors.ErrValidation, Status: http.StatusBadRequest, Code: "validation_error"}, {Target: domainerrors.ErrNotFound, Status: http.StatusNotFound, Code: "not_found", Message: "session report not found"}, {Target: domainerrors.ErrForbidden, Status: http.StatusForbidden, Code: "forbidden", Message: "clinical assignment required"}, {Target: domainerrors.ErrConflict, Status: http.StatusConflict, Code: "conflict", Message: "session report version or state conflict"}, {Target: sessionreport.ErrInvalidOutput, Status: http.StatusUnprocessableEntity, Code: "invalid_model_output", Message: "generated report did not satisfy session-report-v1"}})
}
