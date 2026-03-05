package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type fakeTenantChecker struct {
	exists bool
	err    error
}

func (f fakeTenantChecker) TenantExists(_ context.Context, _ uuid.UUID) (bool, error) {
	return f.exists, f.err
}

func TestRequireTenant_MissingHeader(t *testing.T) {
	t.Parallel()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := RequireTenant(fakeTenantChecker{exists: true})(func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	if err := handler(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal error payload: %v", err)
	}
	errObj, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object in payload")
	}
	if errObj["code"] != "validation_error" {
		t.Fatalf("expected code validation_error, got %v", errObj["code"])
	}
	details, ok := errObj["details"].(map[string]any)
	if !ok {
		t.Fatalf("expected details object in payload")
	}
	if details["field"] != "X-Tenant-ID" {
		t.Fatalf("expected details.field X-Tenant-ID, got %v", details["field"])
	}
	if details["reason"] != "required" {
		t.Fatalf("expected details.reason required, got %v", details["reason"])
	}
}

func TestRequireTenant_SetsTenantInContext(t *testing.T) {
	t.Parallel()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.Header.Set("X-Tenant-ID", uuid.New().String())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var hasTenant bool
	handler := RequireTenant(fakeTenantChecker{exists: true})(func(c echo.Context) error {
		_, ok := TenantIDFromContext(c.Request().Context())
		hasTenant = ok
		return c.NoContent(http.StatusNoContent)
	})

	if err := handler(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if !hasTenant {
		t.Fatalf("expected tenant_id in request context")
	}
}
