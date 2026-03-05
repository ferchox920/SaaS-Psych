package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	auditusecase "sessionflow/apps/api/internal/usecase/audit"
)

type fakeAuditLister struct {
	result auditusecase.ListResult
	err    error
}

func (f *fakeAuditLister) List(_ context.Context, _ auditusecase.ListFilter) (auditusecase.ListResult, error) {
	return f.result, f.err
}

func TestAuditHandlerList(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	actorID := uuid.New()
	entryID := uuid.New()
	handler := NewAuditHandler(&fakeAuditLister{
		result: auditusecase.ListResult{
			Items: []auditusecase.LogEntry{
				{
					ID:          entryID,
					TenantID:    tenantID,
					ActorUserID: &actorID,
					Action:      "auth.login.success",
					Entity:      "auth",
					Metadata:    map[string]any{"method": "password"},
					CreatedAt:   time.Now().UTC(),
				},
			},
			Limit:      20,
			Offset:     0,
			TotalCount: 1,
		},
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetRequest(c.Request().WithContext(httpmiddleware.WithTenantID(c.Request().Context(), tenantID)))

	if err := handler.List(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestAuditHandlerList_InvalidActor(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	handler := NewAuditHandler(&fakeAuditLister{})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit?actor=invalid-uuid", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetRequest(c.Request().WithContext(httpmiddleware.WithTenantID(c.Request().Context(), tenantID)))

	if err := handler.List(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestAuditHandlerList_InvalidCursorID(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	handler := NewAuditHandler(&fakeAuditLister{})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit?cursor_id=invalid-uuid", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetRequest(c.Request().WithContext(httpmiddleware.WithTenantID(c.Request().Context(), tenantID)))

	if err := handler.List(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestAuditHandlerList_ValidationErrorFromService(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	handler := NewAuditHandler(&fakeAuditLister{
		err: domainerrors.NewValidation("action and action_prefix are mutually exclusive"),
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit?action=a&action_prefix=b", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetRequest(c.Request().WithContext(httpmiddleware.WithTenantID(c.Request().Context(), tenantID)))

	if err := handler.List(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}
