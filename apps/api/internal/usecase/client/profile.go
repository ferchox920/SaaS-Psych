package client

import (
	"context"
	"github.com/google/uuid"
	domainclient "sessionflow/apps/api/internal/domain/client"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type profileRepository interface {
	GetProfile(context.Context, uuid.UUID, uuid.UUID) (domainclient.Profile, error)
	SaveProfile(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, domainclient.Profile) (domainclient.Profile, error)
}

func (s *Service) GetProfile(ctx context.Context, tenantID, clientID, actorID uuid.UUID) (domainclient.Profile, error) {
	if _, err := s.Get(ctx, tenantID, clientID, actorID); err != nil {
		return domainclient.Profile{}, err
	}
	repo, ok := s.repo.(profileRepository)
	if !ok {
		return domainclient.Profile{}, domainerrors.ErrNotFound
	}
	return repo.GetProfile(ctx, tenantID, clientID)
}

func (s *Service) SaveProfile(ctx context.Context, tenantID, clientID, actorID uuid.UUID, profile domainclient.Profile) (domainclient.Profile, error) {
	if tenantID == uuid.Nil || clientID == uuid.Nil || actorID == uuid.Nil {
		return domainclient.Profile{}, domainerrors.ErrUnauthorized
	}
	if _, err := s.repo.GetByID(ctx, tenantID, clientID); err != nil {
		return domainclient.Profile{}, err
	}
	if err := s.requireClinicalAccess(ctx, tenantID, actorID, clientID, "treating"); err != nil {
		return domainclient.Profile{}, err
	}
	if err := profile.Validate(s.now()); err != nil {
		return domainclient.Profile{}, err
	}
	repo, ok := s.repo.(profileRepository)
	if !ok {
		return domainclient.Profile{}, domainerrors.ErrNotFound
	}
	return repo.SaveProfile(ctx, tenantID, clientID, actorID, profile)
}
