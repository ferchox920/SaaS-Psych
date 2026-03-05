package http

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	domainclient "sessionflow/apps/api/internal/domain/client"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/http/handlers"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	authusecase "sessionflow/apps/api/internal/usecase/auth"
	clientusecase "sessionflow/apps/api/internal/usecase/client"
)

type integrationClientRepo struct {
	mu       sync.Mutex
	byTenant map[uuid.UUID]map[uuid.UUID]domainclient.Entity
}

func newIntegrationClientRepo() *integrationClientRepo {
	return &integrationClientRepo{byTenant: make(map[uuid.UUID]map[uuid.UUID]domainclient.Entity)}
}

func (r *integrationClientRepo) Create(_ context.Context, in domainclient.Entity) (domainclient.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byTenant[in.TenantID]; !ok {
		r.byTenant[in.TenantID] = make(map[uuid.UUID]domainclient.Entity)
	}
	r.byTenant[in.TenantID][in.ID] = in
	return in, nil
}

func (r *integrationClientRepo) List(_ context.Context, tenantID uuid.UUID) ([]domainclient.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := make([]domainclient.Entity, 0)
	for _, item := range r.byTenant[tenantID] {
		items = append(items, item)
	}
	return items, nil
}

func (r *integrationClientRepo) GetByID(_ context.Context, tenantID, clientID uuid.UUID) (domainclient.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tenantItems, ok := r.byTenant[tenantID]
	if !ok {
		return domainclient.Entity{}, domainerrors.ErrNotFound
	}
	item, ok := tenantItems[clientID]
	if !ok {
		return domainclient.Entity{}, domainerrors.ErrNotFound
	}
	return item, nil
}

func (r *integrationClientRepo) Update(_ context.Context, in domainclient.Entity) (domainclient.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tenantItems, ok := r.byTenant[in.TenantID]
	if !ok {
		return domainclient.Entity{}, domainerrors.ErrNotFound
	}
	if _, ok := tenantItems[in.ID]; !ok {
		return domainclient.Entity{}, domainerrors.ErrNotFound
	}
	tenantItems[in.ID] = in
	return in, nil
}

func (r *integrationClientRepo) Delete(_ context.Context, tenantID, clientID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tenantItems, ok := r.byTenant[tenantID]
	if !ok {
		return domainerrors.ErrNotFound
	}
	if _, ok := tenantItems[clientID]; !ok {
		return domainerrors.ErrNotFound
	}
	delete(tenantItems, clientID)
	return nil
}

func TestClientsTenantIsolationIntegration(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	userID := uuid.New()
	secret := "clients-integration-secret"

	repo := newIntegrationClientRepo()
	service := clientusecase.NewService(repo, nil)
	handler := handlers.NewClientHandler(service)

	server := NewServer(ServerDeps{
		TenantMiddleware: httpmiddleware.RequireTenant(integrationTenantChecker{
			tenants: map[uuid.UUID]struct{}{tenantA: {}, tenantB: {}},
		}),
		AuthMiddleware: httpmiddleware.RequireAuth(secret),
		ClientHandler:  handler,
	})

	tokenService := authusecase.NewTokenService(secret, 15*time.Minute)
	accessA, _, err := tokenService.IssueAccessToken(userID, tenantA, "member")
	if err != nil {
		t.Fatalf("issue token tenantA: %v", err)
	}
	accessB, _, err := tokenService.IssueAccessToken(userID, tenantB, "member")
	if err != nil {
		t.Fatalf("issue token tenantB: %v", err)
	}

	status, body := doJSONRequest(t, server, "POST", "/api/v1/clients", tenantA, accessA, map[string]string{
		"fullname":     "Paciente A",
		"contact":      "paciente-a@test.local",
		"notes_public": "seguimiento",
	})
	if status != 201 {
		t.Fatalf("create client expected 201, got %d body=%s", status, string(body))
	}

	created := struct {
		ID string `json:"id"`
	}{}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected client id in create response")
	}

	status, body = doJSONRequest(t, server, "GET", "/api/v1/clients", tenantA, accessA, nil)
	if status != 200 {
		t.Fatalf("list clients tenantA expected 200, got %d body=%s", status, string(body))
	}

	listA := struct {
		Items []map[string]any `json:"items"`
	}{}
	if err := json.Unmarshal(body, &listA); err != nil {
		t.Fatalf("decode list tenantA response: %v", err)
	}
	if len(listA.Items) != 1 {
		t.Fatalf("expected 1 client for tenantA, got %d", len(listA.Items))
	}

	status, body = doJSONRequest(t, server, "GET", "/api/v1/clients", tenantB, accessB, nil)
	if status != 200 {
		t.Fatalf("list clients tenantB expected 200, got %d body=%s", status, string(body))
	}

	listB := struct {
		Items []map[string]any `json:"items"`
	}{}
	if err := json.Unmarshal(body, &listB); err != nil {
		t.Fatalf("decode list tenantB response: %v", err)
	}
	if len(listB.Items) != 0 {
		t.Fatalf("expected 0 clients for tenantB, got %d", len(listB.Items))
	}

	status, body = doJSONRequest(t, server, "GET", "/api/v1/clients/"+created.ID, tenantB, accessB, nil)
	if status != 404 {
		t.Fatalf("get tenantB for tenantA client expected 404, got %d body=%s", status, string(body))
	}

	errResp := struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}{}
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("decode not found response: %v", err)
	}
	if errResp.Error.Code != "not_found" {
		t.Fatalf("expected not_found code, got %q", errResp.Error.Code)
	}
}
