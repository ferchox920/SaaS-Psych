package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

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
