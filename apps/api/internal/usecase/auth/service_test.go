package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type fakeMetrics struct {
	endpoints []string
	errors    []error
}

func (f *fakeMetrics) RecordAuthError(endpoint string, err error) {
	f.endpoints = append(f.endpoints, endpoint)
	f.errors = append(f.errors, err)
}

func TestLogin_ValidationErrors(t *testing.T) {
	t.Parallel()

	service := NewService(nil, nil, 0, nil)

	_, err := service.Login(context.Background(), uuid.Nil, "user@example.com", "secret")
	if !errors.Is(err, domainerrors.ErrValidation) {
		t.Fatalf("expected validation error for nil tenant, got %v", err)
	}

	_, err = service.Login(context.Background(), uuid.New(), "", "")
	if !errors.Is(err, domainerrors.ErrValidation) {
		t.Fatalf("expected validation error for empty credentials, got %v", err)
	}
}

func TestRefresh_ValidationErrors(t *testing.T) {
	t.Parallel()

	service := NewService(nil, nil, 0, nil)

	_, err := service.Refresh(context.Background(), uuid.Nil, "token")
	if !errors.Is(err, domainerrors.ErrValidation) {
		t.Fatalf("expected validation error for nil tenant, got %v", err)
	}

	_, err = service.Refresh(context.Background(), uuid.New(), "")
	if !errors.Is(err, domainerrors.ErrValidation) {
		t.Fatalf("expected validation error for empty refresh token, got %v", err)
	}
}

func TestLogout_ValidationErrors(t *testing.T) {
	t.Parallel()

	service := NewService(nil, nil, 0, nil)

	err := service.Logout(context.Background(), uuid.Nil, "token")
	if !errors.Is(err, domainerrors.ErrValidation) {
		t.Fatalf("expected validation error for nil tenant, got %v", err)
	}

	err = service.Logout(context.Background(), uuid.New(), "")
	if !errors.Is(err, domainerrors.ErrValidation) {
		t.Fatalf("expected validation error for empty refresh token, got %v", err)
	}
}

func TestLoginRecordsAuthMetricOnUnauthorized(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	metrics := &fakeMetrics{}
	service := NewService(&fakeAuthRepository{
		tenantID: tenantID,
		email:    "owner@test.local",
		byHash:   make(map[string]StoredRefreshToken),
	}, NewTokenService("test-secret", 15*time.Minute), 30*24*time.Hour, nil).WithMetrics(metrics)

	_, err := service.Login(context.Background(), tenantID, "owner@test.local", "wrong")
	if !errors.Is(err, domainerrors.ErrUnauthorized) {
		t.Fatalf("expected unauthorized, got %v", err)
	}
	if len(metrics.endpoints) != 1 || metrics.endpoints[0] != "login" {
		t.Fatalf("expected login metric, got %#v", metrics.endpoints)
	}
	if !errors.Is(metrics.errors[0], domainerrors.ErrUnauthorized) {
		t.Fatalf("expected unauthorized metric error, got %v", metrics.errors[0])
	}
}
