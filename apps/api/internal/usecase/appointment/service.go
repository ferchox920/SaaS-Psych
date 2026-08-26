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
	Create(ctx context.Context, in domainappointment.Entity, actorUserID uuid.UUID) (domainappointment.Entity, error)
	ListByRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]domainappointment.Entity, error)
	GetByID(ctx context.Context, tenantID, appointmentID uuid.UUID) (domainappointment.Entity, error)
	Update(ctx context.Context, in domainappointment.Entity, actorUserID uuid.UUID, action string) (domainappointment.Entity, error)
	ExistsOverlap(ctx context.Context, tenantID uuid.UUID, startsAt, endsAt time.Time, excludeID *uuid.UUID) (bool, error)
	ClientExists(ctx context.Context, tenantID, clientID uuid.UUID) (bool, error)
}

type Service struct {
	repo    Repository
	auditor Auditor
	metrics Metrics
	access  ClinicalAccess
	now     func() time.Time
}

type ClinicalAccess interface {
	CanAccessClient(ctx context.Context, tenantID, userID, clientID uuid.UUID, relationships ...string) (bool, error)
}

type TransactionalWriteAuditor interface {
	WritesAreTransactionallyAudited() bool
}

type Metrics interface {
	RecordAppointmentCreated(err error)
	RecordAppointmentCanceled(err error)
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
	TenantID    uuid.UUID
	ActorUserID uuid.UUID
	From        time.Time
	To          time.Time
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

func (s *Service) WithMetrics(metrics Metrics) *Service {
	s.metrics = metrics
	return s
}

func (s *Service) WithClinicalAccess(access ClinicalAccess) *Service {
	s.access = access
	return s
}

func (s *Service) Create(ctx context.Context, input CreateInput) (domainappointment.Entity, error) {
	entity, err := domainappointment.NewEntity(input.TenantID, input.ClientID, input.StartsAt, input.EndsAt, input.Location, s.now())
	if err != nil {
		s.recordCreateMetric(err)
		return domainappointment.Entity{}, err
	}

	clientExists, err := s.repo.ClientExists(ctx, input.TenantID, input.ClientID)
	if err != nil {
		err = fmt.Errorf("check client exists: %w", err)
		s.recordCreateMetric(err)
		return domainappointment.Entity{}, err
	}
	if !clientExists {
		err = fmt.Errorf("client does not belong to tenant: %w", domainerrors.ErrForbidden)
		s.recordCreateMetric(err)
		return domainappointment.Entity{}, err
	}
	if err := s.requireClinicalAccess(ctx, input.TenantID, input.ActorUserID, input.ClientID, "treating"); err != nil {
		s.recordCreateMetric(err)
		return domainappointment.Entity{}, err
	}

	hasOverlap, err := s.repo.ExistsOverlap(ctx, input.TenantID, entity.StartsAt, entity.EndsAt, nil)
	if err != nil {
		err = fmt.Errorf("check overlap: %w", err)
		s.recordCreateMetric(err)
		return domainappointment.Entity{}, err
	}
	if hasOverlap {
		err = fmt.Errorf("appointment overlaps existing slot: %w", domainerrors.ErrConflict)
		s.recordCreateMetric(err)
		return domainappointment.Entity{}, err
	}

	out, err := s.repo.Create(ctx, entity, input.ActorUserID)
	if err != nil {
		err = fmt.Errorf("create appointment: %w", err)
		s.recordCreateMetric(err)
		return domainappointment.Entity{}, err
	}
	s.recordCreateMetric(nil)
	if err := s.recordWriteAudit(ctx, input.TenantID, input.ActorUserID, "appointment.create", out.ID); err != nil {
		return domainappointment.Entity{}, err
	}
	return out, nil
}

func (s *Service) ListByRange(ctx context.Context, input ListInput) ([]domainappointment.Entity, error) {
	if input.TenantID == uuid.Nil {
		return nil, domainerrors.NewValidation("tenant_id is required")
	}
	if !input.From.Before(input.To) {
		return nil, domainerrors.NewValidation("from must be before to")
	}
	if input.ActorUserID == uuid.Nil {
		return nil, domainerrors.NewValidation("actor_user_id is required")
	}

	items, err := s.repo.ListByRange(ctx, input.TenantID, input.From.UTC(), input.To.UTC())
	if err != nil {
		return nil, fmt.Errorf("list appointments by range: %w", err)
	}
	if s.access == nil {
		if err := s.recordAudit(ctx, input.TenantID, input.ActorUserID, "appointment.list", "appointment", uuid.Nil, map[string]any{}); err != nil {
			return nil, fmt.Errorf("audit appointment list: %w", err)
		}
		return items, nil
	}
	visible := make([]domainappointment.Entity, 0, len(items))
	for _, item := range items {
		allowed, err := s.access.CanAccessClient(ctx, input.TenantID, input.ActorUserID, item.ClientID, "treating", "supervisor")
		if err != nil {
			return nil, fmt.Errorf("check clinical appointment access: %w", err)
		}
		if allowed {
			visible = append(visible, item)
		}
	}
	if err := s.recordAudit(ctx, input.TenantID, input.ActorUserID, "appointment.list", "appointment", uuid.Nil, map[string]any{}); err != nil {
		return nil, fmt.Errorf("audit appointment list: %w", err)
	}
	return visible, nil
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
	if err := s.requireClinicalAccess(ctx, input.TenantID, input.ActorUserID, existing.ClientID, "treating"); err != nil {
		return domainappointment.Entity{}, err
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

	updated, err := s.repo.Update(ctx, existing, input.ActorUserID, "appointment.update")
	if err != nil {
		return domainappointment.Entity{}, fmt.Errorf("update appointment: %w", err)
	}
	if err := s.recordWriteAudit(ctx, input.TenantID, input.ActorUserID, "appointment.update", updated.ID); err != nil {
		return domainappointment.Entity{}, err
	}
	return updated, nil
}

func (s *Service) Cancel(ctx context.Context, tenantID, appointmentID, actorUserID uuid.UUID) (domainappointment.Entity, error) {
	// Design decision: appointments are never hard-deleted from the API.
	// Clinical history is preserved by transitioning lifecycle state scheduled -> canceled.
	if tenantID == uuid.Nil {
		err := domainerrors.NewValidation("tenant_id is required")
		s.recordCancelMetric(err)
		return domainappointment.Entity{}, err
	}
	if appointmentID == uuid.Nil {
		err := domainerrors.NewValidation("appointment_id is required")
		s.recordCancelMetric(err)
		return domainappointment.Entity{}, err
	}

	existing, err := s.repo.GetByID(ctx, tenantID, appointmentID)
	if err != nil {
		err = fmt.Errorf("get appointment: %w", err)
		s.recordCancelMetric(err)
		return domainappointment.Entity{}, err
	}
	if err := s.requireClinicalAccess(ctx, tenantID, actorUserID, existing.ClientID, "treating"); err != nil {
		s.recordCancelMetric(err)
		return domainappointment.Entity{}, err
	}
	if err := existing.Cancel(s.now()); err != nil {
		s.recordCancelMetric(err)
		return domainappointment.Entity{}, err
	}

	updated, err := s.repo.Update(ctx, existing, actorUserID, "appointment.cancel")
	if err != nil {
		err = fmt.Errorf("cancel appointment: %w", err)
		s.recordCancelMetric(err)
		return domainappointment.Entity{}, err
	}
	s.recordCancelMetric(nil)
	if err := s.recordWriteAudit(ctx, tenantID, actorUserID, "appointment.cancel", updated.ID); err != nil {
		return domainappointment.Entity{}, err
	}
	return updated, nil
}

func (s *Service) requireClinicalAccess(ctx context.Context, tenantID, userID, clientID uuid.UUID, relationships ...string) error {
	if s.access == nil {
		return nil
	}
	allowed, err := s.access.CanAccessClient(ctx, tenantID, userID, clientID, relationships...)
	if err != nil {
		return fmt.Errorf("check clinical client access: %w", err)
	}
	if !allowed {
		return domainerrors.ErrForbidden
	}
	return nil
}

func (s *Service) recordCreateMetric(err error) {
	if s.metrics == nil {
		return
	}
	s.metrics.RecordAppointmentCreated(err)
}

func (s *Service) recordCancelMetric(err error) {
	if s.metrics == nil {
		return
	}
	s.metrics.RecordAppointmentCanceled(err)
}

func (s *Service) recordWriteAudit(ctx context.Context, tenantID, actorUserID uuid.UUID, action string, entityID uuid.UUID) error {
	if repo, ok := s.repo.(TransactionalWriteAuditor); ok && repo.WritesAreTransactionallyAudited() {
		return nil
	}
	if err := s.recordAudit(ctx, tenantID, actorUserID, action, "appointment", entityID, map[string]any{}); err != nil {
		return fmt.Errorf("audit appointment write: %w", err)
	}
	return nil
}

func (s *Service) recordAudit(
	ctx context.Context,
	tenantID uuid.UUID,
	actorUserID uuid.UUID,
	action string,
	entity string,
	entityID uuid.UUID,
	metadata map[string]any,
) error {
	if s.auditor == nil {
		return nil
	}
	entityIDCopy := entityID
	return s.auditor.RecordDomainEvent(ctx, tenantID, actorUserID, action, entity, &entityIDCopy, metadata)
}
