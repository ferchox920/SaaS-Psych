package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestHTTPMetricsMiddlewareRecordsRequests(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewHTTPMetrics(registry)
	if err != nil {
		t.Fatalf("new metrics: %v", err)
	}

	e := echo.New()
	e.Use(metrics.Middleware())
	e.GET("/clients/:id", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/clients/123", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	requests := findFamily(t, families, "requests_total")
	if requests.GetType() != dto.MetricType_COUNTER {
		t.Fatalf("expected counter type, got %v", requests.GetType())
	}
	if len(requests.GetMetric()) == 0 {
		t.Fatalf("expected requests_total samples")
	}
	if !hasLabelSet(requests.GetMetric(), map[string]string{
		"method": "GET",
		"path":   "/clients/:id",
		"status": "204",
	}) {
		t.Fatalf("expected requests_total labels method=GET path=/clients/:id status=204")
	}

	duration := findFamily(t, families, "request_duration_seconds")
	if duration.GetType() != dto.MetricType_HISTOGRAM {
		t.Fatalf("expected histogram type, got %v", duration.GetType())
	}
	if len(duration.GetMetric()) == 0 {
		t.Fatalf("expected request_duration_seconds samples")
	}
	if !hasLabelSet(duration.GetMetric(), map[string]string{
		"method": "GET",
		"path":   "/clients/:id",
		"status": "204",
	}) {
		t.Fatalf("expected request_duration_seconds labels method=GET path=/clients/:id status=204")
	}
}

func findFamily(t *testing.T, families []*dto.MetricFamily, name string) *dto.MetricFamily {
	t.Helper()
	for _, family := range families {
		if family.GetName() == name {
			return family
		}
	}
	t.Fatalf("metric family %q not found", name)
	return nil
}

func hasLabelSet(metrics []*dto.Metric, expected map[string]string) bool {
	for _, metric := range metrics {
		actual := make(map[string]string, len(metric.GetLabel()))
		for _, label := range metric.GetLabel() {
			actual[label.GetName()] = label.GetValue()
		}

		match := true
		for key, value := range expected {
			if actual[key] != value {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
