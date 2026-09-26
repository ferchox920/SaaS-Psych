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
	mu          sync.Mutex
	byTenant    map[uuid.UUID]map[uuid.UUID]domainclient.Entity
	assignments map[uuid.UUID]map[uuid.UUID]map[uuid.UUID]string
}

func newIntegrationClientRepo() *integrationClientRepo {
	return &integrationClientRepo{
		byTenant:    make(map[uuid.UUID]map[uuid.UUID]domainclient.Entity),
		assignments: make(map[uuid.UUID]map[uuid.UUID]map[uuid.UUID]string),
	}
}

func (r *integrationClientRepo) Create(_ context.Context, in domainclient.Entity, actorUserID uuid.UUID) (domainclient.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byTenant[in.TenantID]; !ok {
		r.byTenant[in.TenantID] = make(map[uuid.UUID]domainclient.Entity)
	}
	r.byTenant[in.TenantID][in.ID] = in
	if r.assignments[in.TenantID] == nil {
		r.assignments[in.TenantID] = make(map[uuid.UUID]map[uuid.UUID]string)
	}
	r.assignments[in.TenantID][in.ID] = map[uuid.UUID]string{actorUserID: "treating"}
	return in, nil
}

func (r *integrationClientRepo) CanAccessClient(_ context.Context, tenantID, userID, clientID uuid.UUID, relationships ...string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	relationship := r.assignments[tenantID][clientID][userID]
	for _, allowed := range relationships {
		if relationship == allowed {
			return true, nil
		}
	}
	return false, nil
}

func (r *integrationClientRepo) List(_ context.Context, tenantID uuid.UUID) ([]domainclient.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := make([]domainclient.Entity, 0)
	for _, item := range r.byTenant[tenantID] {
		if item.ArchivedAt == nil {
			items = append(items, item)
		}
	}
	return items, nil
}

func (r *integrationClientRepo) ListArchived(_ context.Context, tenantID uuid.UUID) ([]domainclient.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]domainclient.Entity, 0)
	for _, item := range r.byTenant[tenantID] {
		if item.ArchivedAt != nil {
			items = append(items, item)
		}
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

func (r *integrationClientRepo) Update(_ context.Context, in domainclient.Entity, _ uuid.UUID) (domainclient.Entity, error) {
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

func (r *integrationClientRepo) Archive(_ context.Context, tenantID, clientID, actorUserID uuid.UUID, reason string, archivedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tenantItems, ok := r.byTenant[tenantID]
	if !ok {
		return domainerrors.ErrNotFound
	}
	if _, ok := tenantItems[clientID]; !ok {
		return domainerrors.ErrNotFound
	}
	item := tenantItems[clientID]
	item.ArchivedAt = &archivedAt
	item.ArchivedByUserID = &actorUserID
	item.ArchiveReason = reason
	tenantItems[clientID] = item
	return nil
}

func (r *integrationClientRepo) Restore(_ context.Context, tenantID, clientID, _ uuid.UUID, restoredAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.byTenant[tenantID][clientID]
	if !ok || item.ArchivedAt == nil {
		return domainerrors.ErrNotFound
	}
	item.ArchivedAt = nil
	item.ArchivedByUserID = nil
	item.ArchiveReason = ""
	item.UpdatedAt = restoredAt
	r.byTenant[tenantID][clientID] = item
	return nil
}

func TestClientsTenantIsolationIntegration(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	userID := uuid.New()
	unassignedOwnerID := uuid.New()
	secret := "clients-integration-secret"

	repo := newIntegrationClientRepo()
	service := clientusecase.NewService(repo, nil).WithClinicalAccess(repo)
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
	unassignedOwnerToken, _, err := tokenService.IssueAccessToken(unassignedOwnerID, tenantA, "owner")
	if err != nil {
		t.Fatalf("issue unassigned owner token: %v", err)
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
	var pageEnvelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &pageEnvelope); err != nil {
		t.Fatal(err)
	}
	if string(pageEnvelope["next_offset"]) != "null" {
		t.Fatalf("last page must expose null next_offset: %s", body)
	}
	status, body = doJSONRequest(t, server, "GET", "/api/v1/clients?limit=101", tenantA, accessA, nil)
	if status != 400 {
		t.Fatalf("invalid page limit expected 400, got %d body=%s", status, body)
	}
	status, body = doJSONRequest(t, server, "GET", "/api/v1/clients", tenantA, unassignedOwnerToken, nil)
	unassignedList := struct {
		Items []map[string]any `json:"items"`
	}{}
	if status != 200 || json.Unmarshal(body, &unassignedList) != nil || len(unassignedList.Items) != 0 {
		t.Fatalf("unassigned owner must not list clinical clients, status=%d body=%s", status, string(body))
	}
	status, body = doJSONRequest(t, server, "GET", "/api/v1/clients/"+created.ID, tenantA, unassignedOwnerToken, nil)
	if status != 403 {
		t.Fatalf("administrative role alone must not read client content, got %d body=%s", status, string(body))
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

func TestClientArchiveAndRestorePreservesIdentityIntegration(t *testing.T) {
	tenantID, userID := uuid.New(), uuid.New()
	secret := "client-archive-integration-secret"
	repo := newIntegrationClientRepo()
	service := clientusecase.NewService(repo, nil).WithClinicalAccess(repo)
	server := NewServer(ServerDeps{
		TenantMiddleware: httpmiddleware.RequireTenant(integrationTenantChecker{tenants: map[uuid.UUID]struct{}{tenantID: {}}}),
		AuthMiddleware:   httpmiddleware.RequireAuth(secret),
		ClientHandler:    handlers.NewClientHandler(service),
	})
	tokenService := authusecase.NewTokenService(secret, 15*time.Minute)
	token, _, _ := tokenService.IssueAccessToken(userID, tenantID, "member")

	status, body := doJSONRequest(t, server, "POST", "/api/v1/clients", tenantID, token, map[string]string{"fullname": "Fictitious Client"})
	if status != 201 {
		t.Fatalf("create client expected 201, got %d body=%s", status, string(body))
	}
	created := struct {
		ID string `json:"id"`
	}{}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode client: %v", err)
	}
	status, body = doJSONRequest(t, server, "POST", "/api/v1/clients/"+created.ID+"/archive", tenantID, token, map[string]string{"reason": "care completed"})
	if status != 204 {
		t.Fatalf("archive expected 204, got %d body=%s", status, string(body))
	}
	status, body = doJSONRequest(t, server, "GET", "/api/v1/clients", tenantID, token, nil)
	if status != 200 || len(body) == 0 {
		t.Fatalf("active list expected 200, got %d body=%s", status, string(body))
	}
	active := struct {
		Items []map[string]any `json:"items"`
	}{}
	_ = json.Unmarshal(body, &active)
	if len(active.Items) != 0 {
		t.Fatalf("archived client must be hidden from active list: %+v", active.Items)
	}
	status, body = doJSONRequest(t, server, "GET", "/api/v1/clients/archived", tenantID, token, nil)
	archived := struct {
		Items []struct {
			ID            string `json:"id"`
			ArchiveReason string `json:"archive_reason"`
		} `json:"items"`
	}{}
	if status != 200 || json.Unmarshal(body, &archived) != nil || len(archived.Items) != 1 || archived.Items[0].ID != created.ID {
		t.Fatalf("archived history expected original client, got %d body=%s", status, string(body))
	}
	status, body = doJSONRequest(t, server, "POST", "/api/v1/clients/"+created.ID+"/restore", tenantID, token, nil)
	if status != 204 {
		t.Fatalf("restore expected 204, got %d body=%s", status, string(body))
	}
	status, body = doJSONRequest(t, server, "GET", "/api/v1/clients/"+created.ID, tenantID, token, nil)
	if status != 200 {
		t.Fatalf("restored client expected same id and access, got %d body=%s", status, string(body))
	}
}
