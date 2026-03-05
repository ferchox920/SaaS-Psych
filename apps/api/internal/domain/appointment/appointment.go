package appointment

import (
	"strings"
	"time"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

const (
	StatusScheduled = "scheduled"
	StatusCanceled  = "canceled"
)

type Entity struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	ClientID  uuid.UUID
	StartsAt  time.Time
	EndsAt    time.Time
	Status    string
	Location  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewEntity(tenantID, clientID uuid.UUID, startsAt, endsAt time.Time, location string, now time.Time) (Entity, error) {
	if tenantID == uuid.Nil {
		return Entity{}, domainerrors.NewValidation("tenant_id is required")
	}
	if clientID == uuid.Nil {
		return Entity{}, domainerrors.NewValidation("client_id is required")
	}
	if !startsAt.Before(endsAt) {
		return Entity{}, domainerrors.NewValidation("starts_at must be before ends_at")
	}

	if now.IsZero() {
		now = time.Now().UTC()
	}

	return Entity{
		ID:        uuid.New(),
		TenantID:  tenantID,
		ClientID:  clientID,
		StartsAt:  startsAt.UTC(),
		EndsAt:    endsAt.UTC(),
		Status:    StatusScheduled,
		Location:  strings.TrimSpace(location),
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (e *Entity) Update(startsAt, endsAt time.Time, location string, now time.Time) error {
	if e.Status == StatusCanceled {
		return domainerrors.NewValidation("canceled appointment cannot be updated")
	}
	if !startsAt.Before(endsAt) {
		return domainerrors.NewValidation("starts_at must be before ends_at")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	e.StartsAt = startsAt.UTC()
	e.EndsAt = endsAt.UTC()
	e.Location = strings.TrimSpace(location)
	e.UpdatedAt = now
	return nil
}

func (e *Entity) Cancel(now time.Time) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	e.Status = StatusCanceled
	e.UpdatedAt = now
}

func Overlaps(aStart, aEnd, bStart, bEnd time.Time) bool {
	return aStart.Before(bEnd) && bStart.Before(aEnd)
}
