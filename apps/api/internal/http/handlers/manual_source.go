package handlers

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"net/http"
	material "sessionflow/apps/api/internal/domain/manualsource"
	"sessionflow/apps/api/internal/usecase/manualsource"
)

type ManualSourceHandler struct{ service *manualsource.Service }

func NewManualSourceHandler(s *manualsource.Service) *ManualSourceHandler {
	return &ManualSourceHandler{s}
}
func (h *ManualSourceHandler) Import(c echo.Context) error {
	t, p, id, err := clinicalSessionContext(c, "client_id")
	if err != nil {
		return err
	}
	var v material.Import
	if err = decodeIngestionCommand(c, &v, 1000000); err != nil {
		return err
	}
	out, created, err := h.service.Import(c.Request().Context(), t, id, p.UserID, v)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	return c.JSON(status, map[string]any{"material": out, "created": created})
}
func (h *ManualSourceHandler) List(c echo.Context) error {
	t, p, id, err := clinicalSessionContext(c, "client_id")
	if err != nil {
		return err
	}
	limit, offset, valid := parsePage(c)
	if !valid {
		return nil
	}
	items, err := h.service.List(c.Request().Context(), t, id, p.UserID, limit, offset)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(200, map[string]any{"items": items, "next_offset": func() any {
		if len(items) == limit {
			return offset + limit
		}
		return nil
	}()})
}
func (h *ManualSourceHandler) Get(c echo.Context) error {
	t, p, id, err := clinicalSessionContext(c, "client_id")
	if err != nil {
		return err
	}
	source, err := uuid.Parse(c.Param("source_id"))
	if err != nil {
		return writeAPIError(c, 400, "validation_error", "invalid source id")
	}
	var revision *uuid.UUID
	if value := c.QueryParam("revision_id"); value != "" {
		parsed, e := uuid.Parse(value)
		if e != nil {
			return writeAPIError(c, 400, "validation_error", "invalid revision id")
		}
		revision = &parsed
	}
	out, err := h.service.Get(c.Request().Context(), t, id, p.UserID, source, revision)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(200, out)
}

func (h *ManualSourceHandler) Versions(c echo.Context) error {
	t, p, id, err := clinicalSessionContext(c, "client_id")
	if err != nil {
		return err
	}
	source, err := uuid.Parse(c.Param("source_id"))
	if err != nil {
		return writeAPIError(c, 400, "validation_error", "invalid source id")
	}
	limit, offset, valid := parsePage(c)
	if !valid {
		return nil
	}
	items, err := h.service.Versions(c.Request().Context(), t, id, p.UserID, source, limit, offset)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	var next *int
	if len(items) == limit {
		n := offset + limit
		next = &n
	}
	return c.JSON(200, map[string]any{"items": items, "next_offset": next})
}
