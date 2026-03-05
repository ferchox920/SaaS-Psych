package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	appointmentusecase "sessionflow/apps/api/internal/usecase/appointment"
	authusecase "sessionflow/apps/api/internal/usecase/auth"
)

func TestWriteAPIError_WithDetails(t *testing.T) {
	t.Parallel()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body", map[string]any{"field": "body"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	errObj, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object, got %T", payload["error"])
	}
	if errObj["code"] != "validation_error" {
		t.Fatalf("expected code validation_error, got %v", errObj["code"])
	}
	details, ok := errObj["details"].(map[string]any)
	if !ok {
		t.Fatalf("expected details object, got %T", errObj["details"])
	}
	if details["field"] != "body" {
		t.Fatalf("expected details.field body, got %v", details["field"])
	}
}

func TestAuthLogin_MissingCredentials_IncludesDetails(t *testing.T) {
	t.Parallel()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"","password":""}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := NewAuthHandler(&authusecase.Service{})
	if err := handler.Login(c); err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	errObj := payload["error"].(map[string]any)
	details, ok := errObj["details"].(map[string]any)
	if !ok {
		t.Fatalf("expected details object, got %T", errObj["details"])
	}
	fields, ok := details["fields"].([]any)
	if !ok || len(fields) != 2 {
		t.Fatalf("expected details.fields with 2 items, got %v", details["fields"])
	}
}

func TestAppointmentCreate_InvalidClientID_IncludesFieldDetails(t *testing.T) {
	t.Parallel()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/appointments", strings.NewReader(`{"client_id":"bad","starts_at":"2026-03-10T10:00:00Z","ends_at":"2026-03-10T11:00:00Z"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	ctx := httpmiddleware.WithTenantID(req.Context(), uuid.New())
	ctx = httpmiddleware.WithPrincipal(ctx, httpmiddleware.Principal{UserID: uuid.New(), Role: "member"})
	c.SetRequest(req.WithContext(ctx))

	handler := NewAppointmentHandler(&appointmentusecase.Service{})
	if err := handler.Create(c); err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	errObj := payload["error"].(map[string]any)
	details, ok := errObj["details"].(map[string]any)
	if !ok {
		t.Fatalf("expected details object, got %T", errObj["details"])
	}
	if details["field"] != "client_id" {
		t.Fatalf("expected details.field client_id, got %v", details["field"])
	}
}

func TestHandleDomainError_StillCompatibleWithoutDetails(t *testing.T) {
	t.Parallel()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handleDomainError(c, domainerrors.NewValidation("invalid input"), defaultAuthErrorMappings()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	errObj := payload["error"].(map[string]any)
	if _, exists := errObj["details"]; exists {
		t.Fatalf("details should be omitted when not provided")
	}
}
