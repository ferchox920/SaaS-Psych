package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	authusecase "sessionflow/apps/api/internal/usecase/auth"
)

func TestRequireAuth_ValidTokenSetsPrincipal(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := uuid.New()
	tokenService := authusecase.NewTokenService("test-secret", 15*time.Minute)
	accessToken, _, err := tokenService.IssueAccessToken(userID, tenantID, "member")
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetRequest(c.Request().WithContext(WithTenantID(c.Request().Context(), tenantID)))

	var principal Principal
	handler := RequireAuth("test-secret")(func(c echo.Context) error {
		p, ok := PrincipalFromContext(c.Request().Context())
		if !ok {
			t.Fatalf("expected principal in context")
		}
		principal = p
		return c.NoContent(http.StatusNoContent)
	})

	if err := handler(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if principal.UserID != userID {
		t.Fatalf("expected user_id %s, got %s", userID, principal.UserID)
	}
	if principal.Role != "member" {
		t.Fatalf("expected role member, got %s", principal.Role)
	}
}

func TestRequireAuth_TenantMismatch(t *testing.T) {
	t.Parallel()

	tokenService := authusecase.NewTokenService("test-secret", 15*time.Minute)
	accessToken, _, err := tokenService.IssueAccessToken(uuid.New(), uuid.New(), "member")
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetRequest(c.Request().WithContext(WithTenantID(c.Request().Context(), uuid.New())))

	handler := RequireAuth("test-secret")(func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	if err := handler(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestRequireAuth_InvalidTokenErrorFormat(t *testing.T) {
	t.Parallel()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-token")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetRequest(c.Request().WithContext(WithTenantID(c.Request().Context(), uuid.New())))

	handler := RequireAuth("test-secret")(func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	if err := handler(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal error payload: %v", err)
	}
	errObj, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object in payload")
	}
	if errObj["code"] != "unauthorized" {
		t.Fatalf("expected code unauthorized, got %v", errObj["code"])
	}
	details, ok := errObj["details"].(map[string]any)
	if !ok {
		t.Fatalf("expected details object in payload")
	}
	if details["field"] != "Authorization" {
		t.Fatalf("expected details.field Authorization, got %v", details["field"])
	}
	if details["reason"] != "invalid_or_expired_token" {
		t.Fatalf("expected details.reason invalid_or_expired_token, got %v", details["reason"])
	}
}

func TestRequireRole_Forbidden(t *testing.T) {
	t.Parallel()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/admin-check", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetRequest(c.Request().WithContext(WithPrincipal(c.Request().Context(), Principal{
		UserID: uuid.New(),
		Role:   "member",
	})))

	handler := RequireRole("owner", "admin")(func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	if err := handler(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}
