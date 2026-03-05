package http

import (
	"context"
	"encoding/json"
	"fmt"
	stdhttp "net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"sessionflow/apps/api/internal/http/handlers"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	"sessionflow/apps/api/internal/infra/db"
	auditusecase "sessionflow/apps/api/internal/usecase/audit"
	authusecase "sessionflow/apps/api/internal/usecase/auth"
	tenantusecase "sessionflow/apps/api/internal/usecase/tenant"
)

func TestAuditListPostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1 to run postgres integration audit list test")
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
	auditService := auditusecase.NewService(auditRepo)
	tokenService := authusecase.NewTokenService("change-me", 15*time.Minute)
	authService := authusecase.NewService(authRepo, tokenService, 30*24*time.Hour, auditRepo)

	server := NewServer(ServerDeps{
		TenantMiddleware: httpmiddleware.RequireTenant(tenantService),
		AuthMiddleware:   httpmiddleware.RequireAuth("change-me"),
		AuthHandler:      handlers.NewAuthHandler(authService),
		AuditHandler:     handlers.NewAuditHandler(auditService),
	})

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	loginResp := struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}{}
	status, body := doJSONRequest(t, server, stdhttp.MethodPost, "/api/v1/auth/login", tenantID, "", map[string]string{
		"email":    "owner@tenant-a.local",
		"password": "ChangeMe123!",
	})
	if status != stdhttp.StatusOK {
		t.Fatalf("login expected %d, got %d body=%s", stdhttp.StatusOK, status, string(body))
	}
	if err := json.Unmarshal(body, &loginResp); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	if loginResp.AccessToken == "" {
		t.Fatalf("missing access token in login response")
	}
	if loginResp.RefreshToken == "" {
		t.Fatalf("missing refresh token in login response")
	}

	refreshResp := struct {
		RefreshToken string `json:"refresh_token"`
	}{}
	status, body = doJSONRequest(t, server, stdhttp.MethodPost, "/api/v1/auth/refresh", tenantID, "", map[string]string{
		"refresh_token": loginResp.RefreshToken,
	})
	if status != stdhttp.StatusOK {
		t.Fatalf("refresh expected %d, got %d body=%s", stdhttp.StatusOK, status, string(body))
	}
	if err := json.Unmarshal(body, &refreshResp); err != nil {
		t.Fatalf("decode refresh: %v", err)
	}
	if refreshResp.RefreshToken == "" {
		t.Fatalf("missing rotated refresh token")
	}

	status, body = doJSONRequest(t, server, stdhttp.MethodPost, "/api/v1/auth/logout", tenantID, "", map[string]string{
		"refresh_token": refreshResp.RefreshToken,
	})
	if status != stdhttp.StatusNoContent {
		t.Fatalf("logout expected %d, got %d body=%s", stdhttp.StatusNoContent, status, string(body))
	}

	auditResp := struct {
		Items []struct {
			ID        string `json:"id"`
			Action    string `json:"action"`
			Entity    string `json:"entity"`
			TenantID  string `json:"tenant_id"`
			CreatedAt string `json:"created_at"`
		} `json:"items"`
		Pagination struct {
			Limit        int     `json:"limit"`
			Offset       int     `json:"offset"`
			Count        int     `json:"count"`
			TotalCount   int     `json:"total_count"`
			NextCursor   *string `json:"next_cursor"`
			NextCursorID *string `json:"next_cursor_id"`
		} `json:"pagination"`
	}{}

	firstQuery := "/api/v1/audit?limit=1&order=desc&action_prefix=auth&entity=auth"
	status, body = doJSONRequest(t, server, stdhttp.MethodGet, firstQuery, tenantID, loginResp.AccessToken, nil)
	if status != stdhttp.StatusOK {
		t.Fatalf("audit list expected %d, got %d body=%s", stdhttp.StatusOK, status, string(body))
	}
	if err := json.Unmarshal(body, &auditResp); err != nil {
		t.Fatalf("decode audit response: %v", err)
	}

	if auditResp.Pagination.Limit != 1 {
		t.Fatalf("expected limit=1, got %d", auditResp.Pagination.Limit)
	}
	if auditResp.Pagination.TotalCount < 1 {
		t.Fatalf("expected total_count >=1, got %d", auditResp.Pagination.TotalCount)
	}
	if len(auditResp.Items) < 1 {
		t.Fatalf("expected at least one audit item")
	}
	if auditResp.Pagination.NextCursor == nil || auditResp.Pagination.NextCursorID == nil {
		t.Fatalf("expected next cursor and next cursor id for multi-page pagination")
	}
	for _, item := range auditResp.Items {
		if len(item.Action) < 4 || item.Action[:4] != "auth" {
			t.Fatalf("expected action prefix auth, got %s", item.Action)
		}
		if item.Entity != "auth" {
			t.Fatalf("expected entity auth, got %s", item.Entity)
		}
	}

	secondQuery := fmt.Sprintf(
		"/api/v1/audit?limit=1&order=desc&action_prefix=auth&entity=auth&cursor=%s&cursor_id=%s",
		url.QueryEscape(*auditResp.Pagination.NextCursor),
		url.QueryEscape(*auditResp.Pagination.NextCursorID),
	)
	secondPage := auditResp
	status, body = doJSONRequest(t, server, stdhttp.MethodGet, secondQuery, tenantID, loginResp.AccessToken, nil)
	if status != stdhttp.StatusOK {
		t.Fatalf("audit second page expected %d, got %d body=%s", stdhttp.StatusOK, status, string(body))
	}
	if err := json.Unmarshal(body, &secondPage); err != nil {
		t.Fatalf("decode second page audit response: %v", err)
	}
	if len(secondPage.Items) < 1 {
		t.Fatalf("expected at least one item on second page")
	}
	if secondPage.Items[0].ID == auditResp.Items[0].ID {
		t.Fatalf("expected non-overlapping items across pages")
	}

	descOrderResp := auditResp
	descQuery := "/api/v1/audit?limit=5&order=desc&action_prefix=auth&entity=auth"
	status, body = doJSONRequest(t, server, stdhttp.MethodGet, descQuery, tenantID, loginResp.AccessToken, nil)
	if status != stdhttp.StatusOK {
		t.Fatalf("audit desc page expected %d, got %d body=%s", stdhttp.StatusOK, status, string(body))
	}
	if err := json.Unmarshal(body, &descOrderResp); err != nil {
		t.Fatalf("decode desc page: %v", err)
	}
	assertChronologicalOrder(t, descOrderResp.Items, "desc")

	ascOrderResp := auditResp
	ascQuery := "/api/v1/audit?limit=5&order=asc&action_prefix=auth&entity=auth"
	status, body = doJSONRequest(t, server, stdhttp.MethodGet, ascQuery, tenantID, loginResp.AccessToken, nil)
	if status != stdhttp.StatusOK {
		t.Fatalf("audit asc page expected %d, got %d body=%s", stdhttp.StatusOK, status, string(body))
	}
	if err := json.Unmarshal(body, &ascOrderResp); err != nil {
		t.Fatalf("decode asc page: %v", err)
	}
	assertChronologicalOrder(t, ascOrderResp.Items, "asc")
}

func assertChronologicalOrder(t *testing.T, items []struct {
	ID        string `json:"id"`
	Action    string `json:"action"`
	Entity    string `json:"entity"`
	TenantID  string `json:"tenant_id"`
	CreatedAt string `json:"created_at"`
}, order string) {
	t.Helper()

	if len(items) < 2 {
		return
	}

	for i := 1; i < len(items); i++ {
		prevTime, err := time.Parse(time.RFC3339, items[i-1].CreatedAt)
		if err != nil {
			t.Fatalf("parse previous created_at: %v", err)
		}
		currTime, err := time.Parse(time.RFC3339, items[i].CreatedAt)
		if err != nil {
			t.Fatalf("parse current created_at: %v", err)
		}

		switch order {
		case "desc":
			if prevTime.Before(currTime) {
				t.Fatalf("expected desc order by created_at at position %d", i)
			}
			if prevTime.Equal(currTime) && items[i-1].ID < items[i].ID {
				t.Fatalf("expected desc tie-break order by id at position %d", i)
			}
		case "asc":
			if prevTime.After(currTime) {
				t.Fatalf("expected asc order by created_at at position %d", i)
			}
			if prevTime.Equal(currTime) && items[i-1].ID > items[i].ID {
				t.Fatalf("expected asc tie-break order by id at position %d", i)
			}
		default:
			t.Fatalf("unknown order %q", order)
		}
	}
}
