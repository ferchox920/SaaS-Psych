package handlers

import (
	"github.com/labstack/echo/v4"

	httpresponse "sessionflow/apps/api/internal/http/response"
)

func writeAPIError(c echo.Context, status int, code, message string, details ...any) error {
	return httpresponse.WriteError(c, status, code, message, details...)
}
