package http

import (
	"bytes"
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/http/handlers"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	authusecase "sessionflow/apps/api/internal/usecase/auth"
)

type integrationAuthRepo struct {
	mu            sync.Mutex
	usersByTenant map[uuid.UUID]map[string]authusecase.User
	rolesByTenant map[uuid.UUID]map[uuid.UUID]string
	refreshByHash map[uuid.UUID]map[string]authusecase.StoredRefreshToken
}

func newIntegrationAuthRepo(t *testing.T, tenantID, userID uuid.UUID, email, password string, role string) *integrationAuthRepo {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("generate bcrypt hash: %v", err)
	}

	return &integrationAuthRepo{
		usersByTenant: map[uuid.UUID]map[string]authusecase.User{
			tenantID: {
				email: {
					ID:           userID,
					PasswordHash: string(hash),
				},
			},
		},
		rolesByTenant: map[uuid.UUID]map[uuid.UUID]string{
			tenantID: {
				userID: role,
			},
		},
		refreshByHash: make(map[uuid.UUID]map[string]authusecase.StoredRefreshToken),
	}
}

func (r *integrationAuthRepo) GetUserByEmail(_ context.Context, tenantID uuid.UUID, email string) (authusecase.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	users, ok := r.usersByTenant[tenantID]
	if !ok {
		return authusecase.User{}, errors.ErrNotFound
	}

	user, ok := users[email]
	if !ok {
		return authusecase.User{}, errors.ErrNotFound
	}

	return user, nil
}

func (r *integrationAuthRepo) GetUserRole(_ context.Context, tenantID, userID uuid.UUID) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	roles, ok := r.rolesByTenant[tenantID]
	if !ok {
		return "", errors.ErrNotFound
	}

	role, ok := roles[userID]
	if !ok {
		return "", errors.ErrNotFound
	}

	return role, nil
}

func (r *integrationAuthRepo) CreateRefreshToken(_ context.Context, token authusecase.RefreshTokenWrite) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.refreshByHash[token.TenantID]; !ok {
		r.refreshByHash[token.TenantID] = make(map[string]authusecase.StoredRefreshToken)
	}

	r.refreshByHash[token.TenantID][token.TokenHash] = authusecase.StoredRefreshToken{
		UserID:    token.UserID,
		ExpiresAt: token.ExpiresAt,
		RevokedAt: nil,
	}

	return nil
}

func (r *integrationAuthRepo) GetRefreshToken(_ context.Context, tenantID uuid.UUID, tokenHash string) (authusecase.StoredRefreshToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tokens, ok := r.refreshByHash[tenantID]
	if !ok {
		return authusecase.StoredRefreshToken{}, errors.ErrNotFound
	}

	token, ok := tokens[tokenHash]
	if !ok {
		return authusecase.StoredRefreshToken{}, errors.ErrNotFound
	}

	return token, nil
}

func (r *integrationAuthRepo) RevokeRefreshToken(_ context.Context, tenantID uuid.UUID, tokenHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tokens, ok := r.refreshByHash[tenantID]
	if !ok {
		return errors.ErrNotFound
	}

	token, ok := tokens[tokenHash]
	if !ok {
		return errors.ErrNotFound
	}

	if token.RevokedAt != nil {
		return errors.ErrNotFound
	}

	now := time.Now().UTC()
	token.RevokedAt = &now
	tokens[tokenHash] = token

	return nil
}

type integrationTenantChecker struct {
	tenants map[uuid.UUID]struct{}
}

func (c integrationTenantChecker) TenantExists(_ context.Context, tenantID uuid.UUID) (bool, error) {
	_, ok := c.tenants[tenantID]
	return ok, nil
}

func TestAuthFlowIntegration(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	email := "owner@test.local"
	password := "ChangeMe123!"
	secret := "integration-access-secret"

	repo := newIntegrationAuthRepo(t, tenantID, userID, email, password, "owner")
	tokenService := authusecase.NewTokenService(secret, 15*time.Minute)
	authService := authusecase.NewService(repo, tokenService, 30*24*time.Hour, nil)
	authHandler := handlers.NewAuthHandler(authService)

	server := NewServer(ServerDeps{
		TenantMiddleware: httpmiddleware.RequireTenant(integrationTenantChecker{
			tenants: map[uuid.UUID]struct{}{tenantID: {}},
		}),
		AuthMiddleware: httpmiddleware.RequireAuth(secret),
		AuthHandler:    authHandler,
	})

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
	if meResp.UserID != userID.String() || meResp.TenantID != tenantID.String() || meResp.Role != "owner" {
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

func doJSONRequest(
	t *testing.T,
	server stdhttp.Handler,
	method string,
	path string,
	tenantID uuid.UUID,
	accessToken string,
	body any,
) (int, []byte) {
	t.Helper()

	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("X-Tenant-ID", tenantID.String())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes()
}
