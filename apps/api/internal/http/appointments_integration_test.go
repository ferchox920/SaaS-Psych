package http

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	domainappointment "sessionflow/apps/api/internal/domain/appointment"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/http/handlers"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	appointmentusecase "sessionflow/apps/api/internal/usecase/appointment"
	authusecase "sessionflow/apps/api/internal/usecase/auth"
)

type integrationAppointmentRepo struct {
	mu       sync.Mutex
	byTenant map[uuid.UUID]map[uuid.UUID]domainappointment.Entity
	clients  map[uuid.UUID]map[uuid.UUID]struct{}
}

func newIntegrationAppointmentRepo() *integrationAppointmentRepo {
	return &integrationAppointmentRepo{
		byTenant: make(map[uuid.UUID]map[uuid.UUID]domainappointment.Entity),
		clients:  make(map[uuid.UUID]map[uuid.UUID]struct{}),
	}
}

func (r *integrationAppointmentRepo) seedClient(tenantID, clientID uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.clients[tenantID]; !ok {
		r.clients[tenantID] = make(map[uuid.UUID]struct{})
	}
	r.clients[tenantID][clientID] = struct{}{}
}

func (r *integrationAppointmentRepo) Create(_ context.Context, in domainappointment.Entity) (domainappointment.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byTenant[in.TenantID]; !ok {
		r.byTenant[in.TenantID] = make(map[uuid.UUID]domainappointment.Entity)
	}
	r.byTenant[in.TenantID][in.ID] = in
	return in, nil
}

func (r *integrationAppointmentRepo) ListByRange(_ context.Context, tenantID uuid.UUID, from, to time.Time) ([]domainappointment.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := make([]domainappointment.Entity, 0)
	for _, item := range r.byTenant[tenantID] {
		if !item.StartsAt.Before(from) && item.StartsAt.Before(to) {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].StartsAt.Equal(items[j].StartsAt) {
			return items[i].ID.String() < items[j].ID.String()
		}
		return items[i].StartsAt.Before(items[j].StartsAt)
	})
	return items, nil
}

func (r *integrationAppointmentRepo) GetByID(_ context.Context, tenantID, appointmentID uuid.UUID) (domainappointment.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items, ok := r.byTenant[tenantID]
	if !ok {
		return domainappointment.Entity{}, domainerrors.ErrNotFound
	}
	item, ok := items[appointmentID]
	if !ok {
		return domainappointment.Entity{}, domainerrors.ErrNotFound
	}
	return item, nil
}

func (r *integrationAppointmentRepo) Update(_ context.Context, in domainappointment.Entity) (domainappointment.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items, ok := r.byTenant[in.TenantID]
	if !ok {
		return domainappointment.Entity{}, domainerrors.ErrNotFound
	}
	if _, ok := items[in.ID]; !ok {
		return domainappointment.Entity{}, domainerrors.ErrNotFound
	}
	items[in.ID] = in
	return in, nil
}

func (r *integrationAppointmentRepo) ExistsOverlap(_ context.Context, tenantID uuid.UUID, startsAt, endsAt time.Time, excludeID *uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, item := range r.byTenant[tenantID] {
		if item.Status == domainappointment.StatusCanceled {
			continue
		}
		if excludeID != nil && item.ID == *excludeID {
			continue
		}
		if domainappointment.Overlaps(item.StartsAt, item.EndsAt, startsAt, endsAt) {
			return true, nil
		}
	}
	return false, nil
}

func (r *integrationAppointmentRepo) ClientExists(_ context.Context, tenantID, clientID uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.clients[tenantID][clientID]
	return ok, nil
}

func TestAppointmentsTenantIsolationAndRangeIntegration(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	userID := uuid.New()
	secret := "appointments-integration-secret"

	repo := newIntegrationAppointmentRepo()
	clientA := uuid.New()
	clientB := uuid.New()
	repo.seedClient(tenantA, clientA)
	repo.seedClient(tenantB, clientB)
	service := appointmentusecase.NewService(repo, nil)
	handler := handlers.NewAppointmentHandler(service)

	server := NewServer(ServerDeps{
		TenantMiddleware:   httpmiddleware.RequireTenant(integrationTenantChecker{tenants: map[uuid.UUID]struct{}{tenantA: {}, tenantB: {}}}),
		AuthMiddleware:     httpmiddleware.RequireAuth(secret),
		AppointmentHandler: handler,
	})

	tokenService := authusecase.NewTokenService(secret, 15*time.Minute)
	tokenA, _, err := tokenService.IssueAccessToken(userID, tenantA, "member")
	if err != nil {
		t.Fatalf("issue token tenantA: %v", err)
	}
	tokenB, _, err := tokenService.IssueAccessToken(userID, tenantB, "member")
	if err != nil {
		t.Fatalf("issue token tenantB: %v", err)
	}

	base := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	status, body := doJSONRequest(t, server, "POST", "/api/v1/appointments", tenantA, tokenA, map[string]string{
		"client_id": clientA.String(),
		"starts_at": base.Format(time.RFC3339),
		"ends_at":   base.Add(time.Hour).Format(time.RFC3339),
		"location":  "Room A",
	})
	if status != 201 {
		t.Fatalf("create appointment expected 201, got %d body=%s", status, string(body))
	}

	status, body = doJSONRequest(t, server, "POST", "/api/v1/appointments", tenantA, tokenA, map[string]string{
		"client_id": clientA.String(),
		"starts_at": base.Add(30 * time.Minute).Format(time.RFC3339),
		"ends_at":   base.Add(90 * time.Minute).Format(time.RFC3339),
		"location":  "Room B",
	})
	if status != 409 {
		t.Fatalf("overlap expected 409, got %d body=%s", status, string(body))
	}

	status, body = doJSONRequest(t, server, "GET", "/api/v1/appointments?from=2026-03-05T09:00:00Z&to=2026-03-05T12:00:00Z", tenantA, tokenA, nil)
	if status != 200 {
		t.Fatalf("list tenantA expected 200, got %d body=%s", status, string(body))
	}

	listA := struct {
		Items []map[string]any `json:"items"`
	}{}
	if err := json.Unmarshal(body, &listA); err != nil {
		t.Fatalf("decode list tenantA: %v", err)
	}
	if len(listA.Items) != 1 {
		t.Fatalf("expected 1 appointment for tenantA, got %d", len(listA.Items))
	}

	status, body = doJSONRequest(t, server, "GET", "/api/v1/appointments?from=2026-03-05T09:00:00Z&to=2026-03-05T12:00:00Z", tenantB, tokenB, nil)
	if status != 200 {
		t.Fatalf("list tenantB expected 200, got %d body=%s", status, string(body))
	}
	listB := struct {
		Items []map[string]any `json:"items"`
	}{}
	if err := json.Unmarshal(body, &listB); err != nil {
		t.Fatalf("decode list tenantB: %v", err)
	}
	if len(listB.Items) != 0 {
		t.Fatalf("expected 0 appointments for tenantB, got %d", len(listB.Items))
	}

	status, body = doJSONRequest(t, server, "GET", "/api/v1/appointments?from=2026-03-05T11:00:00Z&to=2026-03-05T12:00:00Z", tenantA, tokenA, nil)
	if status != 200 {
		t.Fatalf("list out of range expected 200, got %d body=%s", status, string(body))
	}
	filtered := struct {
		Items []map[string]any `json:"items"`
	}{}
	if err := json.Unmarshal(body, &filtered); err != nil {
		t.Fatalf("decode filtered list: %v", err)
	}
	if len(filtered.Items) != 0 {
		t.Fatalf("expected 0 appointments out of range, got %d", len(filtered.Items))
	}

	status, body = doJSONRequest(t, server, "POST", "/api/v1/appointments", tenantA, tokenA, map[string]string{
		"client_id": clientB.String(),
		"starts_at": base.Add(2 * time.Hour).Format(time.RFC3339),
		"ends_at":   base.Add(3 * time.Hour).Format(time.RFC3339),
		"location":  "Room C",
	})
	if status != 403 {
		t.Fatalf("cross-tenant client expected 403, got %d body=%s", status, string(body))
	}
}
