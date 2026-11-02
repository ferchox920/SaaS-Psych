package googlecalendar

import (
	"context"
	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	domaincalendar "sessionflow/apps/api/internal/domain/googlecalendar"
	"time"
)

func (s *Service) ListLinkedEvents(ctx context.Context, tenantID, userID uuid.UUID, from, to time.Time, limit, offset int) ([]domaincalendar.LinkedEvent, error) {
	if !s.enabled {
		return nil, ErrDisabled
	}
	if tenantID == uuid.Nil || userID == uuid.Nil {
		return nil, domainerrors.ErrUnauthorized
	}
	if !from.Before(to) || to.Sub(from) > 90*24*time.Hour || limit < 1 || limit > 101 || offset < 0 {
		return nil, domainerrors.NewValidation("invalid linked event range or page")
	}
	repo, ok := s.repo.(interface {
		ListLinkedEvents(context.Context, uuid.UUID, uuid.UUID, time.Time, time.Time, int, int) ([]domaincalendar.LinkedEvent, error)
	})
	if !ok {
		return nil, domainerrors.ErrNotFound
	}
	return repo.ListLinkedEvents(ctx, tenantID, userID, from, to, limit, offset)
}
