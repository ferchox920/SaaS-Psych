package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	httpresponse "sessionflow/apps/api/internal/http/response"
)

type RateLimitStore interface {
	Increment(ctx context.Context, key string, window time.Duration) (int64, error)
}

func RequireLoginRateLimit(store RateLimitStore, limit int, window time.Duration) echo.MiddlewareFunc {
	if store == nil || limit <= 0 || window <= 0 {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				return next(c)
			}
		}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := strings.TrimSpace(c.RealIP())
			if ip == "" {
				ip = "unknown"
			}

			key := fmt.Sprintf("ratelimit:auth_login:ip:%s", ip)
			count, err := store.Increment(c.Request().Context(), key, window)
			if err != nil {
				return httpresponse.WriteError(c, http.StatusServiceUnavailable, "service_unavailable", "rate limit unavailable")
			}

			if count > int64(limit) {
				return httpresponse.WriteError(c, http.StatusTooManyRequests, "too_many_requests", "too many login attempts, try again later", map[string]any{
					"field":  "ip",
					"reason": "limit_exceeded",
				})
			}

			remaining := limit - int(count)
			if remaining < 0 {
				remaining = 0
			}
			c.Response().Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
			c.Response().Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

			return next(c)
		}
	}
}
