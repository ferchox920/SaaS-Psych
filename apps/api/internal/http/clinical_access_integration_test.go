package http

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"sessionflow/apps/api/internal/http/handlers"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	authusecase "sessionflow/apps/api/internal/usecase/auth"
	clinicalaccessusecase "sessionflow/apps/api/internal/usecase/clinicalaccess"
)

type integrationClinicalAssignmentRepo struct {
	mu         sync.Mutex
	items      map[uuid.UUID]clinicalaccessusecase.Assignment
	exceptions map[uuid.UUID]clinicalaccessusecase.AccessException
}

func (r *integrationClinicalAssignmentRepo) GrantException(_ context.Context, exception clinicalaccessusecase.AccessException) (clinicalaccessusecase.AccessException, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.exceptions[exception.ID] = exception
	return exception, nil
}

func (r *integrationClinicalAssignmentRepo) ListExceptions(_ context.Context, tenantID, clientID, _ uuid.UUID) ([]clinicalaccessusecase.AccessException, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]clinicalaccessusecase.AccessException, 0)
	for _, item := range r.exceptions {
		if item.TenantID == tenantID && item.ClientID == clientID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (r *integrationClinicalAssignmentRepo) RevokeException(_ context.Context, tenantID, clientID, exceptionID, actorUserID uuid.UUID, reason string, revokedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item := r.exceptions[exceptionID]
	if item.TenantID == tenantID && item.ClientID == clientID {
		item.RevokedAt, item.RevokedByUserID, item.RevokeReason = &revokedAt, &actorUserID, reason
		r.exceptions[exceptionID] = item
	}
	return nil
}

func (r *integrationClinicalAssignmentRepo) Grant(_ context.Context, assignment clinicalaccessusecase.Assignment) (clinicalaccessusecase.Assignment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[assignment.ID] = assignment
	return assignment, nil
}

func (r *integrationClinicalAssignmentRepo) List(_ context.Context, tenantID, clientID, _ uuid.UUID) ([]clinicalaccessusecase.Assignment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]clinicalaccessusecase.Assignment, 0)
	for _, item := range r.items {
		if item.TenantID == tenantID && item.ClientID == clientID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (r *integrationClinicalAssignmentRepo) End(_ context.Context, tenantID, clientID, assignmentID, actorUserID uuid.UUID, reason string, endedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item := r.items[assignmentID]
	if item.TenantID != tenantID || item.ClientID != clientID {
		return nil
	}
	item.EndsAt = &endedAt
	item.EndedByUserID = &actorUserID
	item.EndReason = reason
	r.items[assignmentID] = item
	return nil
}

func TestClinicalAssignmentEndpointsRequireAdministrativeRoleAndPreserveHistory(t *testing.T) {
	tenantID, clientID, ownerID, memberID, clinicianID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	secret := "clinical-assignment-http-secret"
	repo := &integrationClinicalAssignmentRepo{
		items:      make(map[uuid.UUID]clinicalaccessusecase.Assignment),
		exceptions: make(map[uuid.UUID]clinicalaccessusecase.AccessException),
	}
	server := NewServer(ServerDeps{
		TenantMiddleware:      httpmiddleware.RequireTenant(integrationTenantChecker{tenants: map[uuid.UUID]struct{}{tenantID: {}}}),
		AuthMiddleware:        httpmiddleware.RequireAuth(secret),
		ClinicalAccessHandler: handlers.NewClinicalAccessHandler(clinicalaccessusecase.NewService(repo)),
	})
	tokens := authusecase.NewTokenService(secret, 15*time.Minute)
	ownerToken, _, _ := tokens.IssueAccessToken(ownerID, tenantID, "owner")
	memberToken, _, _ := tokens.IssueAccessToken(memberID, tenantID, "member")
	path := "/api/v1/clients/" + clientID.String() + "/assignments"
	payload := map[string]string{"user_id": clinicianID.String(), "relationship": "supervisor"}

	status, body := doJSONRequest(t, server, "POST", path, tenantID, memberToken, payload)
	if status != 403 {
		t.Fatalf("member must not manage assignments, got %d body=%s", status, string(body))
	}
	status, body = doJSONRequest(t, server, "POST", path, tenantID, ownerToken, payload)
	if status != 201 {
		t.Fatalf("owner grant expected 201, got %d body=%s", status, string(body))
	}
	created := struct {
		ID string `json:"id"`
	}{}
	if err := json.Unmarshal(body, &created); err != nil || created.ID == "" {
		t.Fatalf("decode granted assignment: %v body=%s", err, string(body))
	}
	status, body = doJSONRequest(t, server, "DELETE", path+"/"+created.ID, tenantID, ownerToken, map[string]string{"reason": "supervision concluded"})
	if status != 204 {
		t.Fatalf("end assignment expected 204, got %d body=%s", status, string(body))
	}
	status, body = doJSONRequest(t, server, "GET", path, tenantID, ownerToken, nil)
	history := struct {
		Items []struct {
			EndReason string `json:"end_reason"`
			EndsAt    string `json:"ends_at"`
		} `json:"items"`
	}{}
	if status != 200 || json.Unmarshal(body, &history) != nil || len(history.Items) != 1 || history.Items[0].EndReason != "supervision concluded" || history.Items[0].EndsAt == "" {
		t.Fatalf("assignment history must preserve end metadata, status=%d body=%s", status, string(body))
	}
}
