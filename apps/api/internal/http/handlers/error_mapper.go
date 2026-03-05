package handlers

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type domainErrorMapping struct {
	Target  error
	Status  int
	Code    string
	Message string
}

func handleDomainError(c echo.Context, err error, mappings []domainErrorMapping) error {
	for _, mapping := range mappings {
		if errors.Is(err, mapping.Target) {
			message := mapping.Message
			if message == "" {
				message = err.Error()
			}
			return writeAPIError(c, mapping.Status, mapping.Code, message)
		}
	}

	return writeAPIError(c, http.StatusInternalServerError, "internal_error", "internal server error")
}

func defaultAuthErrorMappings() []domainErrorMapping {
	return []domainErrorMapping{
		{
			Target:  domainerrors.ErrUnauthorized,
			Status:  http.StatusUnauthorized,
			Code:    "unauthorized",
			Message: "invalid credentials or token",
		},
		{
			Target:  domainerrors.ErrForbidden,
			Status:  http.StatusForbidden,
			Code:    "forbidden",
			Message: "forbidden",
		},
		{
			Target: domainerrors.ErrValidation,
			Status: http.StatusBadRequest,
			Code:   "validation_error",
		},
	}
}

func defaultAuditErrorMappings() []domainErrorMapping {
	return []domainErrorMapping{
		{
			Target: domainerrors.ErrValidation,
			Status: http.StatusBadRequest,
			Code:   "validation_error",
		},
	}
}
