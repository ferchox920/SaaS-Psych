package handlers

import (
	stdhttp "net/http"

	"github.com/labstack/echo/v4"
)

func Health(c echo.Context) error {
	return c.JSON(stdhttp.StatusOK, map[string]string{
		"status": "ok",
	})
}
