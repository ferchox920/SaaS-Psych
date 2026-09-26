package handlers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

func parsePage(c echo.Context) (int, int, bool) {
	limit, offset := 50, 0
	if raw := c.QueryParam("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			_ = writeAPIError(c, http.StatusBadRequest, "validation_error", "limit must be between 1 and 100", map[string]any{"field": "limit"})
			return 0, 0, false
		}
		limit = value
	}
	if raw := c.QueryParam("offset"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 || value > 1000000 {
			_ = writeAPIError(c, http.StatusBadRequest, "validation_error", "offset must be between 0 and 1000000", map[string]any{"field": "offset"})
			return 0, 0, false
		}
		offset = value
	}
	return limit, offset, true
}
