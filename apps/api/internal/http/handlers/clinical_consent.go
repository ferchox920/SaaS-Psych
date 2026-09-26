package handlers

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"io"
	"net/http"
	"sessionflow/apps/api/internal/usecase/consent"
)

type ClinicalConsentHandler struct{ service *consent.Service }

func NewClinicalConsentHandler(s *consent.Service) *ClinicalConsentHandler {
	return &ClinicalConsentHandler{s}
}
func (h *ClinicalConsentHandler) List(c echo.Context) error {
	t, p, id, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	out, err := h.service.List(c.Request().Context(), t, id, p.UserID)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(200, map[string]any{"items": out})
}
func (h *ClinicalConsentHandler) Grant(c echo.Context) error {
	t, p, id, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	var body struct {
		Scope   string `json:"scope"`
		Version int    `json:"definition_version"`
	}
	d := json.NewDecoder(http.MaxBytesReader(c.Response(), c.Request().Body, 4096))
	d.DisallowUnknownFields()
	if err = d.Decode(&body); err != nil {
		return writeAPIError(c, 400, "validation_error", "invalid consent grant")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return writeAPIError(c, 400, "validation_error", "invalid consent grant")
	}
	out, err := h.service.Grant(c.Request().Context(), t, id, p.UserID, body.Scope, body.Version)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(201, out)
}
func (h *ClinicalConsentHandler) Revoke(c echo.Context) error {
	t, p, id, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	grant, err := uuid.Parse(c.Param("grant_id"))
	if err != nil {
		return writeAPIError(c, 400, "validation_error", "invalid grant id")
	}
	out, err := h.service.Revoke(c.Request().Context(), t, id, p.UserID, grant)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(200, out)
}
