package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestHealth(t *testing.T) {
	t.Parallel()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := Health(c); err != nil {
		t.Fatalf("health handler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	expected := "{\"status\":\"ok\"}\n"
	if rec.Body.String() != expected {
		t.Fatalf("expected body %q, got %q", expected, rec.Body.String())
	}
}

func TestReadinessReportsDependencyStateWithoutLeakingErrors(t *testing.T) {
	t.Parallel()

	checks := map[string]ReadinessCheck{
		"postgres": func(context.Context) error { return nil },
		"redis":    func(context.Context) error { return errors.New("dial redis://secret@internal:6379 failed") },
	}
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	if err := Readiness(checks)(e.NewContext(req, rec)); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
	expected := "{\"checks\":{\"postgres\":\"ok\",\"redis\":\"unavailable\"},\"status\":\"not_ready\"}\n"
	if rec.Body.String() != expected {
		t.Fatalf("expected sanitized readiness body %q, got %q", expected, rec.Body.String())
	}
}

func TestReadinessReturnsOKWhenAllDependenciesRespond(t *testing.T) {
	t.Parallel()

	checks := map[string]ReadinessCheck{
		"postgres": func(context.Context) error { return nil },
		"redis":    func(context.Context) error { return nil },
	}
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	if err := Readiness(checks)(e.NewContext(req, rec)); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}
