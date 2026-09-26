package http

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"sessionflow/apps/api/internal/http/handlers"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	authusecase "sessionflow/apps/api/internal/usecase/auth"
	clinicalaccessusecase "sessionflow/apps/api/internal/usecase/clinicalaccess"
)

type tenantUserDirectoryStub struct {
	byTenant map[uuid.UUID][]clinicalaccessusecase.TenantUser
}

func (s tenantUserDirectoryStub) ListTenantUsers(_ context.Context, tenantID uuid.UUID) ([]clinicalaccessusecase.TenantUser, error) {
	return s.byTenant[tenantID], nil
}

func TestAssignmentUserDirectoryIsAdminOnlyAndTenantScoped(t *testing.T) {
	tenantA, tenantB, clientID := uuid.New(), uuid.New(), uuid.New()
	userA, userB := uuid.New(), uuid.New()
	secret := "directory-test-secret"
	repo := &integrationClinicalAssignmentRepo{items: make(map[uuid.UUID]clinicalaccessusecase.Assignment), exceptions: make(map[uuid.UUID]clinicalaccessusecase.AccessException)}
	directory := tenantUserDirectoryStub{byTenant: map[uuid.UUID][]clinicalaccessusecase.TenantUser{
		tenantA: {{ID: userA, Email: "clinician@a.local"}},
		tenantB: {{ID: userB, Email: "clinician@b.local"}},
	}}
	server := NewServer(ServerDeps{
		TenantMiddleware:      httpmiddleware.RequireTenant(integrationTenantChecker{tenants: map[uuid.UUID]struct{}{tenantA: {}, tenantB: {}}}),
		AuthMiddleware:        httpmiddleware.RequireAuth(secret),
		ClinicalAccessHandler: handlers.NewClinicalAccessHandler(clinicalaccessusecase.NewService(repo)).WithUserDirectory(directory),
	})
	tokens := authusecase.NewTokenService(secret, 15*time.Minute)
	ownerA, _, _ := tokens.IssueAccessToken(uuid.New(), tenantA, "owner")
	memberA, _, _ := tokens.IssueAccessToken(uuid.New(), tenantA, "member")
	ownerB, _, _ := tokens.IssueAccessToken(uuid.New(), tenantB, "owner")
	path := "/api/v1/clients/" + clientID.String() + "/assignments/users"
	status, _ := doJSONRequest(t, server, "GET", path, tenantA, memberA, nil)
	if status != 403 {
		t.Fatalf("member directory status = %d, want 403", status)
	}
	status, body := doJSONRequest(t, server, "GET", path, tenantA, ownerA, nil)
	if status != 200 {
		t.Fatalf("owner directory status = %d: %s", status, body)
	}
	var response struct {
		Items []struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 1 || response.Items[0].ID != userA.String() || response.Items[0].Email != "clinician@a.local" {
		t.Fatalf("tenant A items = %+v", response.Items)
	}
	status, body = doJSONRequest(t, server, "GET", path, tenantB, ownerB, nil)
	if status != 200 || len(body) == 0 {
		t.Fatalf("tenant B status = %d: %s", status, body)
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 1 || response.Items[0].ID != userB.String() {
		t.Fatalf("tenant B items = %+v", response.Items)
	}
}
