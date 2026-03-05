package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

func TestHandleAuthError_Validation(t *testing.T) {
	t.Parallel()

	handler := NewAuthHandler(nil)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.handleAuthError(c, domainerrors.NewValidation("refresh_token is required")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "refresh_token is required") {
		t.Fatalf("expected response to include validation message, got %s", rec.Body.String())
	}
}

func TestHandleAuthError_Forbidden(t *testing.T) {
	t.Parallel()

	handler := NewAuthHandler(nil)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.handleAuthError(c, domainerrors.ErrForbidden); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}
