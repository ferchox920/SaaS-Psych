package middleware

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/trace"
)

const requestIDHeader = "X-Request-ID"

func RequestLogging(logger *slog.Logger) echo.MiddlewareFunc {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			requestID := c.Request().Header.Get(requestIDHeader)
			if requestID == "" {
				requestID = uuid.NewString()
			}
			c.Response().Header().Set(requestIDHeader, requestID)

			start := time.Now()
			err := next(c)
			if err != nil {
				c.Error(err)
			}

			path := c.Path()
			if path == "" {
				path = c.Request().URL.Path
			}

			tenantID, hasTenant := TenantIDFromContext(c.Request().Context())
			principal, hasPrincipal := PrincipalFromContext(c.Request().Context())
			traceID, spanID := traceContextIDs(c.Request().Context())

			attrs := []any{
				slog.String("request_id", requestID),
				slog.String("trace_id", traceID),
				slog.String("span_id", spanID),
				slog.String("method", c.Request().Method),
				slog.String("path", path),
				slog.Int("status", responseStatus(c)),
				slog.Int64("latency_ms", time.Since(start).Milliseconds()),
			}
			if hasTenant {
				attrs = append(attrs, slog.String("tenant_id", tenantID.String()))
			}
			if hasPrincipal {
				attrs = append(attrs, slog.String("user_id", principal.UserID.String()))
			}

			logger.Info("http_request", attrs...)
			return nil
		}
	}
}

func responseStatus(c echo.Context) int {
	status := c.Response().Status
	if status == 0 {
		return 200
	}
	return status
}

func RequestIDFromHeader(c echo.Context) (string, bool) {
	value := c.Response().Header().Get(requestIDHeader)
	if value == "" {
		return "", false
	}
	return value, true
}

func RequestIDHeader() string {
	return requestIDHeader
}

func traceContextIDs(ctx context.Context) (string, string) {
	spanCtx := trace.SpanFromContext(ctx).SpanContext()
	if !spanCtx.IsValid() {
		return "", ""
	}
	return spanCtx.TraceID().String(), spanCtx.SpanID().String()
}
