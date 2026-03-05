package middleware

import (
	"fmt"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func RequestTracing(tracer trace.Tracer) echo.MiddlewareFunc {
	if tracer == nil {
		tracer = otel.Tracer("sessionflow/http")
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := otel.GetTextMapPropagator().Extract(c.Request().Context(), propagation.HeaderCarrier(c.Request().Header))

			path := c.Path()
			if path == "" {
				path = "unmatched"
			}
			spanName := fmt.Sprintf("%s %s", c.Request().Method, path)

			ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindServer))
			span.SetAttributes(
				attribute.String("http.method", c.Request().Method),
				attribute.String("http.route", path),
			)
			defer span.End()

			c.SetRequest(c.Request().WithContext(ctx))

			err := next(c)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				c.Error(err)
			}

			status := responseStatus(c)
			span.SetAttributes(attribute.Int("http.status_code", status))
			if status >= 500 {
				span.SetStatus(codes.Error, "http status >= 500")
			}
			return nil
		}
	}
}
