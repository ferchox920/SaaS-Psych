package tenant

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type Repository interface {
	Exists(ctx context.Context, tenantID uuid.UUID) (bool, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Exists(ctx context.Context, tenantID uuid.UUID) error {
	exists, err := s.repo.Exists(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("check tenant existence: %w", err)
	}
	if !exists {
		return domainerrors.ErrNotFound
	}

	return nil
}

func (s *Service) TenantExists(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	err := s.Exists(ctx, tenantID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, domainerrors.ErrNotFound) {
		return false, nil
	}

	return false, err
}

func IsNotFound(err error) bool {
	return errors.Is(err, domainerrors.ErrNotFound)
}
