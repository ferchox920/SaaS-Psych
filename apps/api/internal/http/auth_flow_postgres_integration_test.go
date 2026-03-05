package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"sessionflow/apps/api/internal/http/handlers"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	"sessionflow/apps/api/internal/infra/db"
	authusecase "sessionflow/apps/api/internal/usecase/auth"
	tenantusecase "sessionflow/apps/api/internal/usecase/tenant"
)

func TestAuthFlowPostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1 to run postgres integration auth flow test")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://sessionflow:sessionflow@127.0.0.1:5432/sessionflow?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := db.NewPostgresPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create postgres pool: %v", err)
	}
	defer pool.Close()

	tenantRepo := db.NewTenantRepository(pool)
	tenantService := tenantusecase.NewService(tenantRepo)

	authRepo := db.NewAuthRepository(pool)
	auditRepo := db.NewAuditRepository(pool)
	tokenService := authusecase.NewTokenService("change-me", 15*time.Minute)
	authService := authusecase.NewService(authRepo, tokenService, 30*24*time.Hour, auditRepo)
	authHandler := handlers.NewAuthHandler(authService)

	server := NewServer(ServerDeps{
		TenantMiddleware: httpmiddleware.RequireTenant(tenantService),
		AuthMiddleware:   httpmiddleware.RequireAuth("change-me"),
		AuthHandler:      authHandler,
	})

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	email := "owner@tenant-a.local"
	password := "ChangeMe123!"

	loginResp := struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}{}
	status, body := doJSONRequest(t, server, stdhttp.MethodPost, "/api/v1/auth/login", tenantID, "", map[string]string{
		"email":    email,
		"password": password,
	})
	if status != stdhttp.StatusOK {
		t.Fatalf("login expected status %d, got %d body=%s", stdhttp.StatusOK, status, string(body))
	}
	if err := json.Unmarshal(body, &loginResp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if loginResp.AccessToken == "" || loginResp.RefreshToken == "" {
		t.Fatalf("login must return access and refresh tokens")
	}

	meResp := struct {
		UserID   string `json:"user_id"`
		TenantID string `json:"tenant_id"`
		Role     string `json:"role"`
	}{}
	status, body = doJSONRequest(t, server, stdhttp.MethodGet, "/api/v1/auth/me", tenantID, loginResp.AccessToken, nil)
	if status != stdhttp.StatusOK {
		t.Fatalf("me expected status %d, got %d body=%s", stdhttp.StatusOK, status, string(body))
	}
	if err := json.Unmarshal(body, &meResp); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	if meResp.TenantID != tenantID.String() || meResp.Role != "owner" {
		t.Fatalf("me response mismatch: %+v", meResp)
	}

	refreshResp := struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}{}
	status, body = doJSONRequest(t, server, stdhttp.MethodPost, "/api/v1/auth/refresh", tenantID, "", map[string]string{
		"refresh_token": loginResp.RefreshToken,
	})
	if status != stdhttp.StatusOK {
		t.Fatalf("refresh expected status %d, got %d body=%s", stdhttp.StatusOK, status, string(body))
	}
	if err := json.Unmarshal(body, &refreshResp); err != nil {
		t.Fatalf("decode refresh response: %v", err)
	}
	if refreshResp.RefreshToken == "" || refreshResp.RefreshToken == loginResp.RefreshToken {
		t.Fatalf("refresh should rotate refresh token")
	}

	status, body = doJSONRequest(t, server, stdhttp.MethodPost, "/api/v1/auth/logout", tenantID, "", map[string]string{
		"refresh_token": refreshResp.RefreshToken,
	})
	if status != stdhttp.StatusNoContent {
		t.Fatalf("logout expected status %d, got %d body=%s", stdhttp.StatusNoContent, status, string(body))
	}

	status, body = doJSONRequest(t, server, stdhttp.MethodPost, "/api/v1/auth/refresh", tenantID, "", map[string]string{
		"refresh_token": refreshResp.RefreshToken,
	})
	if status != stdhttp.StatusUnauthorized {
		t.Fatalf("refresh after logout expected status %d, got %d body=%s", stdhttp.StatusUnauthorized, status, string(body))
	}
}
