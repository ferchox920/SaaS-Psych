package handlers

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"strconv"
)

func (h *ClinicalLongitudinalHandler) ApprovedContext(c echo.Context) error {
	return h.withClient(c, func(t, id, a uuid.UUID) (any, error) {
		return h.service.ApprovedContext(c.Request().Context(), t, id, a)
	})
}
func (h *ClinicalLongitudinalHandler) RunProvenance(c echo.Context) error {
	id, e := uuid.Parse(c.Param("run_id"))
	if e != nil {
		return writeAPIError(c, 400, "validation_error", "invalid run id")
	}
	return h.withClient(c, func(t, client, a uuid.UUID) (any, error) {
		return h.service.RunProvenance(c.Request().Context(), t, client, id, a)
	})
}
func pageOffset(c echo.Context) (int, error) {
	if c.QueryParam("offset") == "" {
		return 0, nil
	}
	return strconv.Atoi(c.QueryParam("offset"))
}
func (h *ClinicalLongitudinalHandler) EvidencePage(c echo.Context) error {
	offset, e := pageOffset(c)
	if e != nil || offset < 0 || offset > 1000000 {
		return writeAPIError(c, 400, "validation_error", "invalid offset")
	}
	return h.withClient(c, func(t, client, a uuid.UUID) (any, error) {
		return h.service.EvidencePage(c.Request().Context(), t, client, a, offset)
	})
}
func (h *ClinicalLongitudinalHandler) History(c echo.Context) error {
	id, e := uuid.Parse(c.Param("entity_id"))
	if e != nil {
		return writeAPIError(c, 400, "validation_error", "invalid entity id")
	}
	offset, e := pageOffset(c)
	if e != nil || offset < 0 || offset > 1000000 {
		return writeAPIError(c, 400, "validation_error", "invalid offset")
	}
	return h.withClient(c, func(t, client, a uuid.UUID) (any, error) {
		return h.service.HistoryPage(c.Request().Context(), t, client, id, a, c.Param("kind"), offset)
	})
}
