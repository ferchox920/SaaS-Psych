package handlers

import (
	"context"
	stdhttp "net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type ReadinessCheck func(context.Context) error

func Health(c echo.Context) error {
	return c.JSON(stdhttp.StatusOK, map[string]string{
		"status": "ok",
	})
}

func Readiness(checks map[string]ReadinessCheck) echo.HandlerFunc {
	return func(c echo.Context) error {
		if len(checks) == 0 {
			return c.JSON(stdhttp.StatusServiceUnavailable, map[string]any{
				"status": "not_ready",
				"checks": map[string]string{"configuration": "unavailable"},
			})
		}

		ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
		defer cancel()
		results := make(map[string]string, len(checks))
		ready := true
		for name, check := range checks {
			if check == nil || check(ctx) != nil {
				results[name] = "unavailable"
				ready = false
				continue
			}
			results[name] = "ok"
		}
		if !ready {
			return c.JSON(stdhttp.StatusServiceUnavailable, map[string]any{"status": "not_ready", "checks": results})
		}
		return c.JSON(stdhttp.StatusOK, map[string]any{"status": "ready", "checks": results})
	}
}
