package http

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	domainsessionnote "sessionflow/apps/api/internal/domain/sessionnote"
	"sessionflow/apps/api/internal/http/handlers"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	authusecase "sessionflow/apps/api/internal/usecase/auth"
	sessionnoteusecase "sessionflow/apps/api/internal/usecase/sessionnote"
)

type integrationSessionNoteRepo struct {
	mu                   sync.Mutex
	byTenant             map[uuid.UUID]map[uuid.UUID]domainsessionnote.Entity
	appointmentsByTenant map[uuid.UUID]map[uuid.UUID]struct{}
}

func newIntegrationSessionNoteRepo() *integrationSessionNoteRepo {
	return &integrationSessionNoteRepo{
		byTenant:             make(map[uuid.UUID]map[uuid.UUID]domainsessionnote.Entity),
		appointmentsByTenant: make(map[uuid.UUID]map[uuid.UUID]struct{}),
	}
}

func (r *integrationSessionNoteRepo) seedAppointment(tenantID, appointmentID uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.appointmentsByTenant[tenantID]; !ok {
		r.appointmentsByTenant[tenantID] = make(map[uuid.UUID]struct{})
	}
	r.appointmentsByTenant[tenantID][appointmentID] = struct{}{}
}

func (r *integrationSessionNoteRepo) AppointmentExists(_ context.Context, tenantID, appointmentID uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.appointmentsByTenant[tenantID][appointmentID]
	return ok, nil
}

func (r *integrationSessionNoteRepo) Create(_ context.Context, in domainsessionnote.Entity) (domainsessionnote.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byTenant[in.TenantID]; !ok {
		r.byTenant[in.TenantID] = make(map[uuid.UUID]domainsessionnote.Entity)
	}
	r.byTenant[in.TenantID][in.ID] = in
	return in, nil
}

func (r *integrationSessionNoteRepo) Update(_ context.Context, in domainsessionnote.Entity) (domainsessionnote.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	notes, ok := r.byTenant[in.TenantID]
	if !ok {
		return domainsessionnote.Entity{}, domainerrors.ErrNotFound
	}
	if _, ok := notes[in.ID]; !ok {
		return domainsessionnote.Entity{}, domainerrors.ErrNotFound
	}
	notes[in.ID] = in
	return in, nil
}

func (r *integrationSessionNoteRepo) GetByID(_ context.Context, tenantID, noteID uuid.UUID) (domainsessionnote.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	notes, ok := r.byTenant[tenantID]
	if !ok {
		return domainsessionnote.Entity{}, domainerrors.ErrNotFound
	}
	note, ok := notes[noteID]
	if !ok {
		return domainsessionnote.Entity{}, domainerrors.ErrNotFound
	}
	return note, nil
}

func (r *integrationSessionNoteRepo) ListByAppointment(_ context.Context, tenantID, appointmentID uuid.UUID) ([]domainsessionnote.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]domainsessionnote.Entity, 0)
	for _, note := range r.byTenant[tenantID] {
		if note.AppointmentID == appointmentID {
			items = append(items, note)
		}
	}
	return items, nil
}

func TestSessionNotesPrivacyIntegration(t *testing.T) {
	tenantID := uuid.New()
	appointmentID := uuid.New()
	authorID := uuid.New()
	ownerID := uuid.New()
	memberID := uuid.New()
	secret := "notes-integration-secret"

	repo := newIntegrationSessionNoteRepo()
	repo.seedAppointment(tenantID, appointmentID)
	service := sessionnoteusecase.NewService(repo, nil)
	handler := handlers.NewSessionNoteHandler(service)

	server := NewServer(ServerDeps{
		TenantMiddleware:   httpmiddleware.RequireTenant(integrationTenantChecker{tenants: map[uuid.UUID]struct{}{tenantID: {}}}),
		AuthMiddleware:     httpmiddleware.RequireAuth(secret),
		AppointmentHandler: handlers.NewAppointmentHandler(nil),
		SessionNoteHandler: handler,
	})

	tokenService := authusecase.NewTokenService(secret, 15*time.Minute)
	authorToken, _, _ := tokenService.IssueAccessToken(authorID, tenantID, "member")
	ownerToken, _, _ := tokenService.IssueAccessToken(ownerID, tenantID, "owner")
	memberToken, _, _ := tokenService.IssueAccessToken(memberID, tenantID, "member")

	status, body := doJSONRequest(t, server, "POST", "/api/v1/appointments/"+appointmentID.String()+"/notes", tenantID, authorToken, map[string]any{
		"body":       "nota privada",
		"is_private": true,
	})
	if status != 201 {
		t.Fatalf("create note expected 201, got %d body=%s", status, string(body))
	}

	created := struct {
		ID string `json:"id"`
	}{}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode create note: %v", err)
	}

	status, body = doJSONRequest(t, server, "GET", "/api/v1/notes/"+created.ID, tenantID, memberToken, nil)
	if status != 403 {
		t.Fatalf("member should not access private note, got %d body=%s", status, string(body))
	}

	status, body = doJSONRequest(t, server, "GET", "/api/v1/notes/"+created.ID, tenantID, ownerToken, nil)
	if status != 200 {
		t.Fatalf("owner should access private note, got %d body=%s", status, string(body))
	}

	status, body = doJSONRequest(t, server, "GET", "/api/v1/appointments/"+appointmentID.String()+"/notes", tenantID, memberToken, nil)
	if status != 200 {
		t.Fatalf("member list expected 200, got %d body=%s", status, string(body))
	}
	memberList := struct {
		Items []map[string]any `json:"items"`
	}{}
	if err := json.Unmarshal(body, &memberList); err != nil {
		t.Fatalf("decode member list: %v", err)
	}
	if len(memberList.Items) != 0 {
		t.Fatalf("member should not see private notes, got %d", len(memberList.Items))
	}

	status, body = doJSONRequest(t, server, "GET", "/api/v1/appointments/"+appointmentID.String()+"/notes", tenantID, authorToken, nil)
	if status != 200 {
		t.Fatalf("author list expected 200, got %d body=%s", status, string(body))
	}
	authorList := struct {
		Items []map[string]any `json:"items"`
	}{}
	if err := json.Unmarshal(body, &authorList); err != nil {
		t.Fatalf("decode author list: %v", err)
	}
	if len(authorList.Items) != 1 {
		t.Fatalf("author should see private note, got %d", len(authorList.Items))
	}
}

func TestSessionNotesUpdatePermissionsIntegration(t *testing.T) {
	tenantID := uuid.New()
	appointmentID := uuid.New()
	authorID := uuid.New()
	ownerID := uuid.New()
	memberID := uuid.New()
	secret := "notes-update-integration-secret"

	repo := newIntegrationSessionNoteRepo()
	repo.seedAppointment(tenantID, appointmentID)
	service := sessionnoteusecase.NewService(repo, nil)
	handler := handlers.NewSessionNoteHandler(service)

	server := NewServer(ServerDeps{
		TenantMiddleware:   httpmiddleware.RequireTenant(integrationTenantChecker{tenants: map[uuid.UUID]struct{}{tenantID: {}}}),
		AuthMiddleware:     httpmiddleware.RequireAuth(secret),
		AppointmentHandler: handlers.NewAppointmentHandler(nil),
		SessionNoteHandler: handler,
	})

	tokenService := authusecase.NewTokenService(secret, 15*time.Minute)
	authorToken, _, _ := tokenService.IssueAccessToken(authorID, tenantID, "member")
	ownerToken, _, _ := tokenService.IssueAccessToken(ownerID, tenantID, "owner")
	memberToken, _, _ := tokenService.IssueAccessToken(memberID, tenantID, "member")

	status, body := doJSONRequest(t, server, "POST", "/api/v1/appointments/"+appointmentID.String()+"/notes", tenantID, authorToken, map[string]any{
		"body":       "original",
		"is_private": true,
	})
	if status != 201 {
		t.Fatalf("create note expected 201, got %d body=%s", status, string(body))
	}

	created := struct {
		ID            string `json:"id"`
		AppointmentID string `json:"appointment_id"`
		AuthorUserID  string `json:"author_user_id"`
	}{}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode create note: %v", err)
	}

	status, body = doJSONRequest(t, server, "PUT", "/api/v1/notes/"+created.ID, tenantID, memberToken, map[string]any{
		"body":       "member update",
		"is_private": false,
	})
	if status != 403 {
		t.Fatalf("member update should be forbidden, got %d body=%s", status, string(body))
	}

	status, body = doJSONRequest(t, server, "PUT", "/api/v1/notes/"+created.ID, tenantID, ownerToken, map[string]any{
		"body":       "owner updated",
		"is_private": false,
	})
	if status != 200 {
		t.Fatalf("owner update expected 200, got %d body=%s", status, string(body))
	}

	updated := struct {
		AppointmentID string `json:"appointment_id"`
		AuthorUserID  string `json:"author_user_id"`
		Body          string `json:"body"`
		IsPrivate     bool   `json:"is_private"`
	}{}
	if err := json.Unmarshal(body, &updated); err != nil {
		t.Fatalf("decode updated note: %v", err)
	}
	if updated.AppointmentID != created.AppointmentID {
		t.Fatalf("appointment_id should remain unchanged, got %s want %s", updated.AppointmentID, created.AppointmentID)
	}
	if updated.AuthorUserID != created.AuthorUserID {
		t.Fatalf("author_user_id should remain unchanged, got %s want %s", updated.AuthorUserID, created.AuthorUserID)
	}
	if updated.Body != "owner updated" || updated.IsPrivate {
		t.Fatalf("unexpected updated payload: body=%q is_private=%v", updated.Body, updated.IsPrivate)
	}
}
