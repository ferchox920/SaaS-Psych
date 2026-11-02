package manualsource

import (
	"context"

	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	material "sessionflow/apps/api/internal/domain/manualsource"
)

type Repository interface {
	Import(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, material.Import, []byte, string) (material.Material, bool, error)
	List(context.Context, uuid.UUID, uuid.UUID, int, int) ([]material.Material, error)
	Get(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, *uuid.UUID) (material.Material, error)
	Versions(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, int, int) ([]material.Material, error)
}
type Access interface {
	CanAccessClient(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, ...string) (bool, error)
}
type Service struct {
	repo   Repository
	access Access
}

func New(r Repository, a Access) *Service { return &Service{r, a} }
func (s *Service) require(ctx context.Context, t, c, a uuid.UUID, write bool) error {
	if t == uuid.Nil || c == uuid.Nil || a == uuid.Nil || s.access == nil {
		return domainerrors.ErrForbidden
	}
	roles := []string{"treating"}
	if !write {
		roles = append(roles, "supervisor")
	}
	ok, err := s.access.CanAccessClient(ctx, t, a, c, roles...)
	if err != nil {
		return err
	}
	if !ok {
		return domainerrors.ErrForbidden
	}
	return nil
}
func (s *Service) Import(ctx context.Context, t, c, a uuid.UUID, v material.Import) (material.Material, bool, error) {
	if err := s.require(ctx, t, c, a, true); err != nil {
		return material.Material{}, false, err
	}
	raw, hash, err := v.Canonical()
	if err != nil {
		return material.Material{}, false, err
	}
	return s.repo.Import(ctx, t, c, a, v, raw, hash)
}
func (s *Service) List(ctx context.Context, t, c, a uuid.UUID, limit, offset int) ([]material.Material, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, domainerrors.NewValidation("invalid source page")
	}
	if err := s.require(ctx, t, c, a, false); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, t, c, limit, offset)
}
func (s *Service) Get(ctx context.Context, t, c, a, id uuid.UUID, revision *uuid.UUID) (material.Material, error) {
	if err := s.require(ctx, t, c, a, false); err != nil {
		return material.Material{}, err
	}
	return s.repo.Get(ctx, t, c, id, revision)
}

func (s *Service) Versions(ctx context.Context, t, c, a, id uuid.UUID, limit, offset int) ([]material.Material, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, domainerrors.NewValidation("invalid revision page")
	}
	if err := s.require(ctx, t, c, a, false); err != nil {
		return nil, err
	}
	return s.repo.Versions(ctx, t, c, id, limit, offset)
}
