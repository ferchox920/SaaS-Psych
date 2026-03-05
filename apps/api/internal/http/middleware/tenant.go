package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	httpresponse "sessionflow/apps/api/internal/http/response"
)

const tenantHeader = "X-Tenant-ID"

type TenantChecker interface {
	TenantExists(ctx context.Context, tenantID uuid.UUID) (bool, error)
}

func RequireTenant(checker TenantChecker) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			rawTenantID := strings.TrimSpace(c.Request().Header.Get(tenantHeader))
			if rawTenantID == "" {
				return httpresponse.WriteError(c, http.StatusBadRequest, "validation_error", "missing X-Tenant-ID header", map[string]any{
					"field":  tenantHeader,
					"reason": "required",
				})
			}

			tenantID, err := uuid.Parse(rawTenantID)
			if err != nil {
				return httpresponse.WriteError(c, http.StatusBadRequest, "validation_error", "invalid X-Tenant-ID format", map[string]any{
					"field":  tenantHeader,
					"reason": "invalid_uuid",
				})
			}

			exists, err := checker.TenantExists(c.Request().Context(), tenantID)
			if err != nil {
				return httpresponse.WriteError(c, http.StatusInternalServerError, "internal_error", "tenant validation failed")
			}
			if !exists {
				return httpresponse.WriteError(c, http.StatusUnauthorized, "unauthorized", "tenant does not exist")
			}

			ctx := WithTenantID(c.Request().Context(), tenantID)
			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}
