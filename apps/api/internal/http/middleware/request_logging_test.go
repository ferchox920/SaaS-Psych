package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestRequestLoggingSetsRequestIDHeader(t *testing.T) {
	e := echo.New()
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	handler := RequestLogging(logger)(func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	if err := handler(e.NewContext(req, rec)); err != nil {
		t.Fatalf("request logging handler: %v", err)
	}

	if rec.Header().Get(RequestIDHeader()) == "" {
		t.Fatalf("expected %s header", RequestIDHeader())
	}
	if logs.Len() == 0 {
		t.Fatalf("expected log output")
	}

	entry := map[string]any{}
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatalf("decode json log: %v", err)
	}
	if _, ok := entry["request_id"]; !ok {
		t.Fatalf("expected request_id in log entry: %v", entry)
	}
	traceID, hasTraceID := entry["trace_id"]
	spanID, hasSpanID := entry["span_id"]
	if !hasTraceID || !hasSpanID {
		t.Fatalf("expected trace_id/span_id in log entry: %v", entry)
	}
	if traceID != "" || spanID != "" {
		t.Fatalf("expected empty trace_id/span_id without active span, got trace_id=%v span_id=%v", traceID, spanID)
	}
}

func TestRequestLoggingPreservesIncomingRequestID(t *testing.T) {
	e := echo.New()
	handler := RequestLogging(nil)(func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set(RequestIDHeader(), "req-123")
	rec := httptest.NewRecorder()
	if err := handler(e.NewContext(req, rec)); err != nil {
		t.Fatalf("request logging handler: %v", err)
	}

	if got := rec.Header().Get(RequestIDHeader()); got != "req-123" {
		t.Fatalf("expected request id req-123, got %q", got)
	}
}

func TestRequestLoggingIncludesTraceAndSpanWhenTracingActive(t *testing.T) {
	e := echo.New()
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	tp := sdktrace.NewTracerProvider()
	defer func() {
		if err := tp.Shutdown(t.Context()); err != nil {
			t.Fatalf("shutdown tracer provider: %v", err)
		}
	}()
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(previous)

	e.Use(RequestTracing(tp.Tracer("test-http")))
	e.Use(RequestLogging(logger))
	e.GET("/health", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	entry := map[string]any{}
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatalf("decode json log: %v", err)
	}

	traceID, ok := entry["trace_id"].(string)
	if !ok || traceID == "" {
		t.Fatalf("expected non-empty trace_id, got %v", entry["trace_id"])
	}
	spanID, ok := entry["span_id"].(string)
	if !ok || spanID == "" {
		t.Fatalf("expected non-empty span_id, got %v", entry["span_id"])
	}
}
