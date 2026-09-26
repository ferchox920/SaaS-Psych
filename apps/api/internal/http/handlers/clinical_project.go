package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func (h *ClinicalLongitudinalHandler) ExportProject(c echo.Context) error {
	t, p, id, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	var body struct {
		ProcessID uuid.UUID `json:"process_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(c.Response(), c.Request().Body, 4096))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&body); err != nil {
		return writeAPIError(c, 400, "validation_error", "invalid export request")
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return writeAPIError(c, 400, "validation_error", "invalid export request")
	}
	out, err := h.service.ExportProject(c.Request().Context(), t, id, p.UserID, body.ProcessID)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(http.StatusCreated, out)
}
func (h *ClinicalLongitudinalHandler) GetProjectExport(c echo.Context) error {
	t, p, id, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	exportID, err := uuid.Parse(c.Param("export_id"))
	if err != nil {
		return writeAPIError(c, 400, "validation_error", "invalid export id")
	}
	out, err := h.service.GetProjectExport(c.Request().Context(), t, id, p.UserID, exportID)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(http.StatusOK, out)
}
func (h *ClinicalLongitudinalHandler) ImportProject(c echo.Context) error {
	t, p, id, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	raw, err := io.ReadAll(http.MaxBytesReader(c.Response(), c.Request().Body, 1024*1024))
	if err != nil {
		return writeAPIError(c, 400, "validation_error", "invalid import size")
	}
	out, err := h.service.ImportProject(c.Request().Context(), t, id, p.UserID, raw)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(http.StatusCreated, out)
}
func (h *ClinicalLongitudinalHandler) GetProjectProposal(c echo.Context) error {
	t, p, id, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	proposalID, err := uuid.Parse(c.Param("proposal_id"))
	if err != nil {
		return writeAPIError(c, 400, "validation_error", "invalid proposal id")
	}
	out, err := h.service.GetProjectProposal(c.Request().Context(), t, id, p.UserID, proposalID)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(http.StatusOK, out)
}
