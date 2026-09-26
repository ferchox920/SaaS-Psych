package handlers

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func (h *ClinicalLongitudinalHandler) ProjectExports(c echo.Context) error {
	offset, err := pageOffset(c)
	if err != nil || offset < 0 || offset > 1000000 {
		return writeAPIError(c, 400, "validation_error", "invalid offset")
	}
	return h.withClient(c, func(t, client, a uuid.UUID) (any, error) {
		return h.service.ProjectExports(c.Request().Context(), t, client, a, offset)
	})
}
