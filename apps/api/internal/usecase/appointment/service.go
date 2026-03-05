package appointment

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	domainappointment "sessionflow/apps/api/internal/domain/appointment"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type Repository interface {
	Create(ctx context.Context, in domainappointment.Entity) (domainappointment.Entity, error)
	ListByRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]domainappointment.Entity, error)
	GetByID(ctx context.Context, tenantID, appointmentID uuid.UUID) (domainappointment.Entity, error)
	Update(ctx context.Context, in domainappointment.Entity) (domainappointment.Entity, error)
	ExistsOverlap(ctx context.Context, tenantID uuid.UUID, startsAt, endsAt time.Time, excludeID *uuid.UUID) (bool, error)
	ClientExists(ctx context.Context, tenantID, clientID uuid.UUID) (bool, error)
}

type Service struct {
	repo    Repository
	auditor Auditor
	now     func() time.Time
}

type Auditor interface {
	RecordDomainEvent(
		ctx context.Context,
		tenantID uuid.UUID,
		actorUserID uuid.UUID,
		action string,
		entity string,
		entityID *uuid.UUID,
		metadata map[string]any,
	) error
}

type CreateInput struct {
	TenantID    uuid.UUID
	ClientID    uuid.UUID
	ActorUserID uuid.UUID
	StartsAt    time.Time
	EndsAt      time.Time
	Location    string
}

type ListInput struct {
	TenantID uuid.UUID
	From     time.Time
	To       time.Time
}

type UpdateInput struct {
	TenantID      uuid.UUID
	AppointmentID uuid.UUID
	ActorUserID   uuid.UUID
	StartsAt      time.Time
	EndsAt        time.Time
	Location      string
}

func NewService(repo Repository, auditor Auditor) *Service {
	return &Service{repo: repo, auditor: auditor, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (domainappointment.Entity, error) {
	entity, err := domainappointment.NewEntity(input.TenantID, input.ClientID, input.StartsAt, input.EndsAt, input.Location, s.now())
	if err != nil {
		return domainappointment.Entity{}, err
	}

	clientExists, err := s.repo.ClientExists(ctx, input.TenantID, input.ClientID)
	if err != nil {
		return domainappointment.Entity{}, fmt.Errorf("check client exists: %w", err)
	}
	if !clientExists {
		return domainappointment.Entity{}, fmt.Errorf("client does not belong to tenant: %w", domainerrors.ErrForbidden)
	}

	hasOverlap, err := s.repo.ExistsOverlap(ctx, input.TenantID, entity.StartsAt, entity.EndsAt, nil)
	if err != nil {
		return domainappointment.Entity{}, fmt.Errorf("check overlap: %w", err)
	}
	if hasOverlap {
		return domainappointment.Entity{}, fmt.Errorf("appointment overlaps existing slot: %w", domainerrors.ErrConflict)
	}

	out, err := s.repo.Create(ctx, entity)
	if err != nil {
		return domainappointment.Entity{}, fmt.Errorf("create appointment: %w", err)
	}
	s.recordAudit(ctx, input.TenantID, input.ActorUserID, "appointment.create", "appointment", out.ID, map[string]any{})
	return out, nil
}

func (s *Service) ListByRange(ctx context.Context, input ListInput) ([]domainappointment.Entity, error) {
	if input.TenantID == uuid.Nil {
		return nil, domainerrors.NewValidation("tenant_id is required")
	}
	if !input.From.Before(input.To) {
		return nil, domainerrors.NewValidation("from must be before to")
	}

	items, err := s.repo.ListByRange(ctx, input.TenantID, input.From.UTC(), input.To.UTC())
	if err != nil {
		return nil, fmt.Errorf("list appointments by range: %w", err)
	}
	return items, nil
}

func (s *Service) Update(ctx context.Context, input UpdateInput) (domainappointment.Entity, error) {
	if input.TenantID == uuid.Nil {
		return domainappointment.Entity{}, domainerrors.NewValidation("tenant_id is required")
	}
	if input.AppointmentID == uuid.Nil {
		return domainappointment.Entity{}, domainerrors.NewValidation("appointment_id is required")
	}

	existing, err := s.repo.GetByID(ctx, input.TenantID, input.AppointmentID)
	if err != nil {
		return domainappointment.Entity{}, fmt.Errorf("get appointment: %w", err)
	}

	if err := existing.Update(input.StartsAt, input.EndsAt, input.Location, s.now()); err != nil {
		return domainappointment.Entity{}, err
	}

	clientExists, err := s.repo.ClientExists(ctx, input.TenantID, existing.ClientID)
	if err != nil {
		return domainappointment.Entity{}, fmt.Errorf("check client exists: %w", err)
	}
	if !clientExists {
		return domainappointment.Entity{}, fmt.Errorf("client does not belong to tenant: %w", domainerrors.ErrForbidden)
	}

	excludeID := existing.ID
	hasOverlap, err := s.repo.ExistsOverlap(ctx, input.TenantID, existing.StartsAt, existing.EndsAt, &excludeID)
	if err != nil {
		return domainappointment.Entity{}, fmt.Errorf("check overlap: %w", err)
	}
	if hasOverlap {
		return domainappointment.Entity{}, fmt.Errorf("appointment overlaps existing slot: %w", domainerrors.ErrConflict)
	}

	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return domainappointment.Entity{}, fmt.Errorf("update appointment: %w", err)
	}
	s.recordAudit(ctx, input.TenantID, input.ActorUserID, "appointment.update", "appointment", updated.ID, map[string]any{})
	return updated, nil
}

func (s *Service) Cancel(ctx context.Context, tenantID, appointmentID, actorUserID uuid.UUID) (domainappointment.Entity, error) {
	// Design decision: appointments are never hard-deleted from the API.
	// Clinical history is preserved by transitioning lifecycle state scheduled -> canceled.
	if tenantID == uuid.Nil {
		return domainappointment.Entity{}, domainerrors.NewValidation("tenant_id is required")
	}
	if appointmentID == uuid.Nil {
		return domainappointment.Entity{}, domainerrors.NewValidation("appointment_id is required")
	}

	existing, err := s.repo.GetByID(ctx, tenantID, appointmentID)
	if err != nil {
		return domainappointment.Entity{}, fmt.Errorf("get appointment: %w", err)
	}
	existing.Cancel(s.now())

	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return domainappointment.Entity{}, fmt.Errorf("cancel appointment: %w", err)
	}
	s.recordAudit(ctx, tenantID, actorUserID, "appointment.cancel", "appointment", updated.ID, map[string]any{})
	return updated, nil
}

func (s *Service) recordAudit(
	ctx context.Context,
	tenantID uuid.UUID,
	actorUserID uuid.UUID,
	action string,
	entity string,
	entityID uuid.UUID,
	metadata map[string]any,
) {
	if s.auditor == nil {
		return
	}
	entityIDCopy := entityID
	_ = s.auditor.RecordDomainEvent(ctx, tenantID, actorUserID, action, entity, &entityIDCopy, metadata)
}
